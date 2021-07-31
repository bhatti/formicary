package scheduler

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/metrics"
	"sync"
	"sync/atomic"
	"time"

	"plexobject.com/formicary/internal/math"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"

	"plexobject.com/formicary/internal/health"

	"plexobject.com/formicary/queen/fsm"
	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
)

const maxNoMoreJobs = 10

// JobScheduler for scheduling jobs
type JobScheduler struct {
	id                            string
	serverCfg                     *config.ServerConfig
	queueClient                   queue.Client
	jobManager                    *manager.JobManager
	artifactManager               *manager.ArtifactManager
	errorRepository               repository.ErrorCodeRepository
	userRepository                repository.UserRepository
	orgRepository                 repository.OrganizationRepository
	resourceManager               resource.Manager
	metricsRegistry               *metrics.Registry
	monitor                       *health.Monitor
	jobSchedulerLeaderTopic       string
	lastJobSchedulerLeaderEventAt time.Time
	lock                          sync.RWMutex
	busy                          bool
	noJobsTries                   int
	totalPendingJobs              uint64
	totalScheduledJobs            uint64
	done                          chan bool
	tickers                       []*time.Ticker
}

// New creates new scheduler
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	jobManager *manager.JobManager,
	artifactManager *manager.ArtifactManager,
	errorRepository repository.ErrorCodeRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	resourceManager resource.Manager,
	monitor *health.Monitor,
	metricsRegistry *metrics.Registry,
) *JobScheduler {
	return &JobScheduler{
		id:                            serverCfg.ID + "-job-scheduler",
		serverCfg:                     serverCfg,
		queueClient:                   queueClient,
		jobManager:                    jobManager,
		artifactManager:               artifactManager,
		errorRepository:               errorRepository,
		userRepository:                userRepository,
		orgRepository:                 orgRepository,
		resourceManager:               resourceManager,
		monitor:                       monitor,
		metricsRegistry:               metricsRegistry,
		jobSchedulerLeaderTopic:       serverCfg.GetJobSchedulerLeaderTopic(),
		lastJobSchedulerLeaderEventAt: time.Unix(0, 0),
		busy:                          false,
		noJobsTries:                   0,
		done:                          make(chan bool, 1),
		tickers:                       make([]*time.Ticker, 0),
	}
}

// Start - creates periodic ticker for scheduling pending jobs
func (js *JobScheduler) Start(ctx context.Context) (err error) {
	if err = js.subscribeToJobSchedulerLeader(ctx); err != nil {
		return err
	}
	js.tickers = append(js.tickers, js.startTickerToSendJobSchedulerLeaderEvents(ctx))
	js.tickers = append(js.tickers, js.startTickerToSchedulePendingJobs(ctx))
	js.tickers = append(js.tickers, js.startTickerToCheckOrphanJobs(ctx))
	js.tickers = append(js.tickers, js.startTickerToCheckMissingCronJobs(ctx))
	return nil
}

// Stop - stops background subscription and ticker routine
func (js *JobScheduler) Stop(ctx context.Context) error {
	if err := js.unsubscribeToJobSchedulerLeader(ctx); err != nil {
		return err
	}
	for _, ticker := range js.tickers {
		ticker.Stop()
		js.done <- true
	}
	return nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (js *JobScheduler) schedulePendingJobs(ctx context.Context) (err error) {
	js.lockWithBusy()
	defer js.unlockWithBusy()
	if js.noJobsTries > 0 {
		js.noJobsTries--
		return
	}

	if js.serverCfg.ShuttingDown {
		logrus.WithFields(logrus.Fields{
			"Component": "JobScheduler",
			"ID":        js.serverCfg.ID,
		}).Error("server shutting down so stopping scheduling")
		_ = js.Stop(ctx)
		js.metricsRegistry.Incr("scheduler_shutting_down_total", nil)
		return fmt.Errorf("server shutting down so stopping scheduling")
	}

	if err = js.monitor.HealthStatus(ctx); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobScheduler",
			"ID":        js.serverCfg.ID,
			"Error":     err,
		}).Error("failed to schedule jobs because health monitor failed")
		js.metricsRegistry.Incr("scheduler_bad_health", nil)
		js.noJobsTries += 2
		return
	}

	js.metricsRegistry.Incr("scheduler_checked_pending_total", nil)
	requests, err := js.jobManager.NextSchedulableJobRequestsByType(
		[]string{},
		common.PENDING,
		1000)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobScheduler",
			"ID":        js.serverCfg.ID,
			"Error":     err,
		}).
			Error("failed to find pending jobs")
		if js.noJobsTries < maxNoMoreJobs {
			js.metricsRegistry.Incr("scheduler_no_more_jobs_total", nil)
			js.noJobsTries++
		}
		return err
	}

	if len(requests) == 0 {
		js.noJobsTries += 3 // TODO better backoff policy here
		return fmt.Errorf("no pending jobs")
	}

	atomic.AddUint64(&js.totalPendingJobs, uint64(len(requests)))
	scheduled := 0
	for _, req := range requests {
		if err := js.scheduleJob(ctx, req); err != nil {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component":        "JobScheduler",
					"ID":               js.serverCfg.ID,
					"RequestID":        req.ID,
					"JobType":          req.JobType,
					"RequestRetry":     req.GetRetried(),
					"Priority":         req.GetJobPriority(),
					"ScheduleAttempts": req.ScheduleAttempts,
				}).Debug("failed to schedule job")
			}
			js.metricsRegistry.Incr("scheduler_failed_total", nil)
		} else {
			scheduled++
		}

	}
	atomic.AddUint64(&js.totalScheduledJobs, uint64(scheduled))
	if scheduled == 0 {
		if js.noJobsTries < maxNoMoreJobs {
			js.noJobsTries++
		}
	} else {
		js.noJobsTries = 0
	}
	return nil
}

// scheduling the job for execution if resources are available to execute it
func (js *JobScheduler) scheduleJob(
	ctx context.Context,
	request *types.JobRequestInfo) (err error) {
	// Creating state machine for request and execution
	jobStateMachine := fsm.NewJobExecutionStateMachine(
		js.serverCfg,
		js.queueClient,
		js.jobManager,
		js.artifactManager,
		js.errorRepository,
		js.userRepository,
		js.orgRepository,
		js.resourceManager,
		js.metricsRegistry,
		request,
		map[string]*common.AntReservation{},
	)

	if err = jobStateMachine.Validate(); err != nil {
		// change status from READY to FAILED
		return jobStateMachine.ScheduleFailed(
			ctx,
			fmt.Errorf("failed to validate job-state due to %s", err.Error()),
			common.ErrorValidation,
		)
	}
	if err := jobStateMachine.CheckSubscriptionQuota(); err != nil {
		// change status from READY to FAILED
		return jobStateMachine.ScheduleFailed(
			ctx,
			err,
			common.ErrorQuotaExceeded,
		)
	}

	if err = jobStateMachine.ShouldFilter(); err != nil {
		// change status from READY to FAILED
		err = jobStateMachine.ScheduleFailed(
			ctx,
			err,
			common.ErrorFilteredJob,
		)

		if jobStateMachine.JobDefinition.DeleteFilteredCronJobs() {
			// mark cron job that filtered as deleted
			_ = js.jobManager.DeleteJobRequest(common.NewQueryContext("", "", ""), request.ID)
			logrus.WithFields(logrus.Fields{
				"Component":        "JobScheduler",
				"ID":               js.serverCfg.ID,
				"RequestID":        request.ID,
				"JobType":          request.JobType,
				"ScheduleAttempts": request.ScheduleAttempts,
				"Error":            err,
			}).Warn("deleting filtered cron-job")
		}
		return nil
	}

	// Check to make sure we have ants connected to execute job for task methods/tags
	if err = jobStateMachine.CheckAntResourcesAndConcurrencyForJob(); err != nil {
		if request.ScheduleAttempts+1 > js.serverCfg.Jobs.MaxScheduleAttempts {
			// changing state from PENDING to FAILED
			return jobStateMachine.ScheduleFailed(
				ctx,
				fmt.Errorf("allocation failed due to %s, max schedule attempts exceeded %d",
					err.Error(), request.ScheduleAttempts),
				common.ErrorAntResources)
		}
		decrPriority := 0
		scheduleAttempts := request.ScheduleAttempts + 1
		scheduleSecs := math.Min(int(js.serverCfg.Jobs.NotReadyJobsMaxWait.Seconds()), scheduleAttempts*5)
		if scheduleAttempts >= 5 && scheduleAttempts%5 == 0 && request.JobPriority > 5 {
			// decrement priority if we can't schedule it immediately -- every 5th attempt, decrement by 1
			decrPriority = 1
		}
		request.ScheduledAt = request.ScheduledAt.Add(time.Duration(scheduleSecs) * time.Second)
		request.JobPriority = request.JobPriority - decrPriority
		_ = js.jobManager.IncrementScheduleAttemptsForJobRequest(
			request,
			time.Duration(scheduleSecs),
			decrPriority,
			err.Error())
		// will try again
		logrus.WithFields(logrus.Fields{
			"Component":        "JobScheduler",
			"ID":               js.serverCfg.ID,
			"RequestID":        request.ID,
			"JobType":          request.JobType,
			"Organization":     request.OrganizationID,
			"User":             request.UserID,
			"ScheduleAttempts": request.ScheduleAttempts,
			"Error":            err,
		}).Warnf("failed to schedule job")
		return fmt.Errorf("ant resources are not available for id=%d, type=%s attempts=%d priority=%d due to %v",
			request.ID, request.JobType, scheduleAttempts, request.JobPriority-decrPriority, err)
	}

	// Reserve resources for tasks
	if err = jobStateMachine.ReserveJobResources(); err != nil {
		if request.ScheduleAttempts+1 > js.serverCfg.Jobs.MaxScheduleAttempts {
			// changing state from PENDING to FAILED
			return jobStateMachine.ScheduleFailed(
				ctx,
				fmt.Errorf("max schedule attempts exceeded %d", request.ScheduleAttempts),
				common.ErrorAntResources,
			)
		}
		// will try again
		return fmt.Errorf("ant resources cannot be allocated for ID=%d, Type=%s State=%s due to %v",
			request.ID, request.JobType, request.JobState, err)
	}

	// Creating a new job-execution
	var dbError, eventError error
	if dbError, eventError = jobStateMachine.CreateJobExecution(ctx); dbError != nil {
		logrus.WithFields(logrus.Fields{
			"Component":        "JobScheduler",
			"ID":               js.serverCfg.ID,
			"RequestID":        request.ID,
			"JobType":          request.JobType,
			"ScheduleAttempts": request.ScheduleAttempts,
			"Error":            dbError,
		}).Error("failed to create job execution")
		// changing state from PENDING to FAILED if job-execution cannot be created
		return jobStateMachine.ScheduleFailed(
			ctx,
			dbError,
			common.ErrorJobExecute)
	}

	if eventError != nil {
		// change status from READY to PENDING so that we can try to send it again (retryable error)
		return jobStateMachine.RevertRequestToPending(eventError)
	}
	logrus.WithFields(logrus.Fields{
		"Component":        "JobScheduler",
		"ID":               js.serverCfg.ID,
		"RequestID":        request.ID,
		"CronTrigger":      jobStateMachine.JobDefinition.CronTrigger,
		"JobExecutionID":   jobStateMachine.JobExecution.ID,
		"JobType":          request.JobType,
		"Paused":           jobStateMachine.JobDefinition.Paused,
		"RequestRetry":     request.GetRetried(),
		"Priority":         request.GetJobPriority(),
		"ScheduleAttempts": request.ScheduleAttempts,
	}).Info("scheduling job...")
	return nil
}

func (js *JobScheduler) unlockWithBusy() {
	js.lock.Unlock()
	js.busy = false
}

func (js *JobScheduler) lockWithBusy() {
	js.lock.Lock()
	js.busy = true
}
