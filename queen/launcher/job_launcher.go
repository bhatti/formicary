package launcher

import (
	"context"
	evbus "github.com/asaskevich/EventBus"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"sync"

	"plexobject.com/formicary/queen/resource"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/fsm"
	"plexobject.com/formicary/queen/supervisor"
)

// JobLauncher for launching jobs
type JobLauncher struct {
	id                         string
	serverCfg                  *config.ServerConfig
	queueClient                queue.Client
	jobManager                 *manager.JobManager
	artifactManager            *manager.ArtifactManager
	userManager                *manager.UserManager
	resourceManager            resource.Manager
	errorCodeRepository        repository.ErrorCodeRepository
	metricsRegistry            *metrics.Registry
	eventBus                   evbus.Bus
	supervisors                map[string]*supervisor.JobSupervisor
	jobLaunchSubscriptionID    string
	jobLifecycleSubscriptionID string
	lock                       sync.RWMutex
}

// New creates new scheduler
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	jobManager *manager.JobManager,
	artifactManager *manager.ArtifactManager,
	userManager *manager.UserManager,
	resourceManager resource.Manager,
	errorCodeRepository repository.ErrorCodeRepository,
	metricsRegistry *metrics.Registry,
) *JobLauncher {
	return &JobLauncher{
		id:                  serverCfg.Common.ID + "-job-launcher",
		serverCfg:           serverCfg,
		queueClient:         queueClient,
		jobManager:          jobManager,
		artifactManager:     artifactManager,
		errorCodeRepository: errorCodeRepository,
		userManager:         userManager,
		resourceManager:     resourceManager,
		metricsRegistry:     metricsRegistry,
		eventBus:            evbus.New(),
		supervisors:         make(map[string]*supervisor.JobSupervisor),
	}
}

// Start - creates periodic ticker for scheduling pending jobs
func (jl *JobLauncher) Start(ctx context.Context) (err error) {
	if jl.jobLaunchSubscriptionID, err = jl.subscribeToJobLaunch(ctx); err != nil {
		_ = jl.Stop(ctx)
		return err
	}
	if jl.jobLifecycleSubscriptionID, err = jl.subscribeToJobLifecycleEvent(
		ctx,
		jl.serverCfg.Common.GetJobExecutionLifecycleTopic()); err != nil {
		_ = jl.Stop(ctx)
		return err
	}
	return nil
}

// CountProcessingJobs returns count of jobs being processed
func (jl *JobLauncher) CountProcessingJobs() int {
	return len(jl.supervisors)
}

// Stop stops background processes
func (jl *JobLauncher) Stop(ctx context.Context) error {
	err1 := jl.queueClient.UnSubscribe(
		ctx,
		jl.serverCfg.GetJobExecutionLaunchTopic(),
		jl.jobLaunchSubscriptionID)
	err2 := jl.queueClient.UnSubscribe(
		ctx,
		jl.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		jl.jobLifecycleSubscriptionID)
	return utils.ErrorsAny(err1, err2)
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// launching job for execution
func (jl *JobLauncher) launchJob(
	ctx context.Context,
	requestID string,
	jobType string,
	jobExecutionID string,
	allocationsByTaskType map[string]*common.AntReservation) (err error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":             "JobLauncher",
			"RequestID":             requestID,
			"JobType":               jobType,
			"JobExecutionID":        jobExecutionID,
			"AllocationsByTaskType": allocationsByTaskType,
		}).Debug("launching job...")
	}

	requestInfo, err := jl.jobManager.GetJobRequest(
		common.NewQueryContext(nil, "").WithAdmin(),
		requestID)
	if err != nil {
		return err
	}

	// Creating state machine for request and execution
	jobStateMachine := fsm.NewJobExecutionStateMachine(
		jl.serverCfg,
		jl.queueClient,
		jl.jobManager,
		jl.artifactManager,
		jl.userManager,
		jl.resourceManager,
		jl.errorCodeRepository,
		jl.metricsRegistry,
		requestInfo,
		allocationsByTaskType)

	if err = jobStateMachine.PrepareLaunch(jobExecutionID); err != nil {
		switch err.(type) {
		case *common.JobRequeueError:
			jl.metricsRegistry.Incr("launcher_requeued_total", nil)
			// changing state from READY to PENDING
			return jobStateMachine.RevertRequestToPendingPaused(err)
		default:
			jl.metricsRegistry.Incr("launcher_failed_total", nil)
			logrus.WithFields(logrus.Fields{
				"Component":             "JobLauncher",
				"RequestID":             requestID,
				"JobType":               jobType,
				"JobExecutionID":        jobExecutionID,
				"AllocationsByTaskType": allocationsByTaskType,
			}).WithError(err).Warnf("failed to launch request")
			// changing state from READY to FAILED
			return jobStateMachine.LaunchFailed(ctx, err)
		}
	}

	// Starting job in a background goroutine
	jobSupervisor := supervisor.NewJobSupervisor(
		jl.serverCfg,
		jobStateMachine,
		jl.eventBus)
	jl.addSupervisor(requestID, jobSupervisor)
	_ = jobSupervisor.AsyncExecute(ctx)
	return nil
}

// Subscribing to job launch topic
func (jl *JobLauncher) subscribeToJobLaunch(ctx context.Context) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		jobLaunchEvent, err := events.UnmarshalJobExecutionLaunchEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "JobLauncher",
				"ID":        jl.serverCfg.Common.ID,
				"Data":      string(event.Payload),
				"Error":     err,
			}).Error("failed to parse launch event")
			return err
		}

		// Launching job
		if err := jl.launchJob(
			ctx,
			jobLaunchEvent.JobRequestID,
			jobLaunchEvent.JobType,
			jobLaunchEvent.JobExecutionID,
			jobLaunchEvent.Reservations,
		); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":      "JobLauncher",
				"ID":             jl.serverCfg.Common.ID,
				"Request":        jobLaunchEvent.JobRequestID,
				"JobType":        jobLaunchEvent.JobType,
				"JobExecutionID": jobLaunchEvent.JobExecutionID,
				"Error":          err,
			}).Error("failed to launch job as a result of launch event")
		}

		return nil
	}
	return jl.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    jl.serverCfg.GetJobExecutionLaunchTopic(),
		Shared:   true,
		Callback: callback,
		Props:    make(map[string]string),
	})
}

func (jl *JobLauncher) subscribeToJobLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		jobExecutionLifecycleEvent, err := events.UnmarshalJobExecutionLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobLauncher",
				"Target":                     jl.id,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
				"Error":                      err}).Error("failed to unmarshal jobExecutionLifecycleEvent")
			return err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobLauncher",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Target":                     jl.id,
				"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
			}).Debug("received job lifecycle event")
		}
		// job-launcher just subscribes to messaging queue once and then uses messaging bus to propagate events
		// to all job-supervisors so that each job-supervisor doesn't need to consume queue resources
		jl.eventBus.Publish(
			jl.serverCfg.Common.GetJobExecutionLifecycleTopic(),
			ctx,
			jobExecutionLifecycleEvent)
		if jobExecutionLifecycleEvent.JobState.IsTerminal() {
			jl.removeSupervisor(ctx, jobExecutionLifecycleEvent)
		}
		return nil
	}
	return jl.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    subscriptionTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}

func (jl *JobLauncher) addSupervisor(requestID string, jobSupervisor *supervisor.JobSupervisor) {
	jl.lock.Lock()
	defer jl.lock.Unlock()
	jl.supervisors[requestID] = jobSupervisor
}

func (jl *JobLauncher) removeSupervisor(
	ctx context.Context,
	jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) {
	jl.lock.Lock()
	jobSupervisor := jl.supervisors[jobExecutionLifecycleEvent.JobRequestID]
	delete(jl.supervisors, jobExecutionLifecycleEvent.JobRequestID)
	jl.lock.Unlock()
	// cancel explicitly to make sure we don't miss it
	if jobSupervisor != nil &&
		(jobExecutionLifecycleEvent.JobState == common.CANCELLED ||
			jobExecutionLifecycleEvent.JobState == common.PAUSED) { // not MANUAL_APPROVAL_REQUIRED
		logrus.WithFields(logrus.Fields{
			"Component":                  "JobLauncher",
			"ID":                         jobExecutionLifecycleEvent.ID,
			"Target":                     jl.id,
			"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
			"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
		}).Infof("forwarding cancellation job lifecycle event")
		_ = jobSupervisor.UpdateFromJobLifecycleEvent(ctx, jobExecutionLifecycleEvent)
	} else if jobSupervisor != nil &&
		jobExecutionLifecycleEvent.JobState == common.FAILED {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobLauncher",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Target":                     jl.id,
				"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
			}).Debugf("skip forwarding cancellation job lifecycle failed event")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":                  "JobLauncher",
			"ID":                         jobExecutionLifecycleEvent.ID,
			"Target":                     jl.id,
			"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
			"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
		}).Infof("skip forwarding cancellation job lifecycle event")
	}
}
