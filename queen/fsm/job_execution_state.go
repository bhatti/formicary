package fsm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"plexobject.com/formicary/internal/metrics"

	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"

	"plexobject.com/formicary/internal/events"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/queen/config"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// - Request is created with PENDING state
// - It is picked up by job-scheduler, which creates a job-execution with READY state and then change state of
//   request to READY while saving the job-execution-id in request.
// - The job-scheduler publishes a JobExecutionLaunchEvent event, but it reverts request/execution state
//   from READY to FAILED if an error occurs.
// - The job-launcher subscribes to JobExecutionLaunchEvent and starts job-supervisor to execute the job, but
//   it reverts request/execution state from READY to FAILED if an error occurs.
// - The job-supervisor changes state of request/execution from READY to EXECUTING, but
//   it reverts request/execution state from READY to FAILED if an error occurs.
// - The job-supervisor continues with task-execution and changes state of request/execution from EXECUTING
//   to FAILED/COMPLETED if a task fails.
// - A user may cancel a job externally and in that case the job must be cancelled.
// Note: a job request can be rerun, so it may change its state to PENDING if it's retried or re-queued but
//   the job execution is immutable and cannot change its state from terminal to non-terminal.
//

// JobExecutionStateMachine for managing state of request and its execution
type JobExecutionStateMachine struct {
	QueueClient         queue.Client
	JobManager          *manager.JobManager
	ArtifactManager     *manager.ArtifactManager
	ErrorCodeRepository repository.ErrorCodeRepository
	userManager         *manager.UserManager
	ResourceManager     resource.Manager
	MetricsRegistry     *metrics.Registry
	Request             types.IJobRequest
	JobDefinition       *types.JobDefinition
	JobExecution        *types.JobExecution
	LastJobExecution    *types.JobExecution
	User                *common.User
	Reservations        map[string]*common.AntReservation
	StartedAt           time.Time
	revertState         common.RequestState
	id                  string
	serverCfg           *config.ServerConfig
	errorCode           *common.ErrorCode
	cpuUsage            types.ResourceUsage
	diskUsage           types.ResourceUsage
}

// NewJobExecutionStateMachine creates new state machine for request execution
func NewJobExecutionStateMachine(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	jobManager *manager.JobManager,
	artifactManager *manager.ArtifactManager,
	userManager *manager.UserManager,
	resourceManager resource.Manager,
	errorCodeRepository repository.ErrorCodeRepository,
	metricsRegistry *metrics.Registry,
	request types.IJobRequest,
	reservations map[string]*common.AntReservation) *JobExecutionStateMachine {
	return &JobExecutionStateMachine{
		id:                  fmt.Sprintf("%s-job-execution-fsm-%s", serverCfg.Common.ID, request.GetID()),
		serverCfg:           serverCfg,
		QueueClient:         queueClient,
		JobManager:          jobManager,
		ArtifactManager:     artifactManager,
		userManager:         userManager,
		ErrorCodeRepository: errorCodeRepository,
		ResourceManager:     resourceManager,
		MetricsRegistry:     metricsRegistry,
		Request:             request,
		Reservations:        reservations,
		StartedAt:           time.Now(),
	}
}

// Validate validates
func (jsm *JobExecutionStateMachine) Validate() (err error) {
	// Validate validates required fields
	if jsm.serverCfg == nil {
		return fmt.Errorf("server-config is not specified")
	}
	if jsm.userManager == nil {
		return fmt.Errorf("userManager is not specified")
	}
	if jsm.Request == nil {
		return fmt.Errorf("job-request is not specified")
	}
	if jsm.QueueClient == nil {
		return fmt.Errorf("queue-client is not specified")
	}
	if jsm.JobManager == nil {
		return fmt.Errorf("job-manager is not specified")
	}
	if jsm.Reservations == nil {
		return fmt.Errorf("job-allocations is not specified")
	}

	jsm.revertState = jsm.Request.GetJobState()
	if jsm.revertState != common.PAUSED {
		jsm.revertState = common.PENDING
	}

	// checking params
	if jsm.Request.GetParams() == nil {
		switch jsm.Request.(type) {
		case *types.JobRequestInfo:
			reqParams, err := jsm.JobManager.GetJobRequestParams(jsm.Request.GetID())
			if err != nil {
				return err
			}
			jsm.Request.SetParams(reqParams)
		}
	}

	// loading user and organization from request
	if jsm.Request.GetUserID() != "" {
		jsm.User, err = jsm.userManager.GetUser(
			jsm.QueryContext(),
			jsm.Request.GetUserID())
		if err != nil {
			return err
		}
	}

	// if pin job-definition to the version
	jsm.JobDefinition, err = jsm.JobManager.GetJobDefinition(
		jsm.QueryContext(),
		jsm.Request.GetJobDefinitionID(),
	)
	//jsm.JobDefinition, err = jsm.JobManager.GetJobDefinitionByType(
	//	jsm.QueryContext(),
	//	jsm.Request.GetJobType(),
	//	jsm.Request.GetJobVersion(),
	//)

	if err != nil {
		return err
	}
	if err = jsm.JobDefinition.Validate(); err != nil {
		return err
	}

	if jsm.User == nil && jsm.JobDefinition.UserID != "" && !jsm.JobDefinition.PublicPlugin {
		jsm.User, err = jsm.userManager.GetUser(
			jsm.QueryContext(),
			jsm.JobDefinition.UserID)
		if err != nil {
			return err
		}
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":          "JobExecutionStateMachine",
			"RequestID":          jsm.Request.GetID(),
			"RequestUser":        jsm.Request.GetUserID(),
			"RequestOrg":         jsm.Request.GetOrganizationID(),
			"JobType":            jsm.Request.GetJobType(),
			"RequestState":       jsm.Request.GetJobState(),
			"RequestRetry":       jsm.Request.GetRetried(),
			"CronTriggered":      jsm.Request.GetCronTriggered(),
			"Priority":           jsm.Request.GetJobPriority(),
			"Scheduled":          jsm.Request.GetScheduledAt(),
			"ScheduleAttempts":   jsm.Request.GetScheduleAttempts(),
			"LastJobExecutionID": jsm.Request.GetLastJobExecutionID(),
			"JobDefinitionID":    jsm.JobDefinition.ID,
			"JobDefinitionUser":  jsm.JobDefinition.UserID,
			"JobDefinitionOrg":   jsm.JobDefinition.OrganizationID,
			"QC":                 jsm.QueryContext(),
		}).Debugf("validated request")
	}

	return
}

// ShouldSkip checks if job should be filtered
func (jsm *JobExecutionStateMachine) ShouldSkip() error {
	if jsm.JobDefinition == nil || jsm.JobDefinition.SkipIf() == "" {
		return nil
	}

	data := jsm.buildDynamicParams(nil)
	if jsm.JobDefinition.ShouldSkip(data) {
		logrus.WithFields(logrus.Fields{
			"Component":         "JobExecutionStateMachine",
			"RequestID":         jsm.Request.GetID(),
			"RequestUser":       jsm.Request.GetUserID(),
			"RequestOrg":        jsm.Request.GetOrganizationID(),
			"JobType":           jsm.Request.GetJobType(),
			"RequestState":      jsm.Request.GetJobState(),
			"Scheduled":         jsm.Request.GetScheduledAt(),
			"ScheduleAttempts":  jsm.Request.GetScheduleAttempts(),
			"JobDefinitionID":   jsm.JobDefinition.ID,
			"JobDefinitionUser": jsm.JobDefinition.UserID,
			"JobDefinitionOrg":  jsm.JobDefinition.OrganizationID,
			"SkipIf":            jsm.JobDefinition.SkipIf(),
			"Data":              common.MaskVariableValues(data),
		}).Warnf("filtered job from schedule")

		return fmt.Errorf("job filtered due to %s", jsm.JobDefinition.SkipIf())
	}
	return nil
}

// PrepareLaunch initializes definition/request/execution objects by job-launcher before execution by job-supervisor
func (jsm *JobExecutionStateMachine) PrepareLaunch(jobExecutionID string) (err error) {
	if err = jsm.Validate(); err != nil {
		return err
	}
	if jobExecutionID == "" {
		return fmt.Errorf("job-execution-id is not specified for prepare launch")
	}

	// verify allocations
	if len(jsm.Reservations) != len(jsm.JobDefinition.Tasks) {
		return fmt.Errorf("expected ant allocations %d to match tasks count %d",
			len(jsm.Reservations), len(jsm.JobDefinition.Tasks))
	}

	for _, task := range jsm.JobDefinition.Tasks {
		if jsm.Reservations[task.TaskType] == nil {
			return common.NewJobRequeueError(
				fmt.Errorf("no ant reservations found for the task '%s', total=%d",
					task.TaskType, len(jsm.Reservations)))
		}
	}

	// Finding job execution
	if exec, err := jsm.JobManager.GetJobExecution(jobExecutionID); err == nil {
		jsm.JobExecution = exec
	} else {
		return err
	}

	// Verify job-execution id
	if jobExecutionID != jsm.Request.GetJobExecutionID() {
		return fmt.Errorf("mismatched job-execution-id '%s' but in request '%s'",
			jobExecutionID, jsm.Request.GetJobExecutionID())
	}

	// Load last job execution if exists
	if jsm.Request.GetLastJobExecutionID() != "" {
		jsm.LastJobExecution, _ = jsm.JobManager.GetJobExecution(
			jsm.Request.GetLastJobExecutionID())
	}

	// Verify job-request and job-execution state
	if jsm.Request.GetJobState() != common.READY ||
		jsm.JobExecution.JobState != common.READY {
		return fmt.Errorf("expected READY state but job request was status %s and job-execution was state %s",
			jsm.Request.GetJobState(), jsm.JobExecution.JobState)
	}

	return
}

// CreateJobExecution moves the state from PENDING|PAUSED to READY for request and creates a new
// job-execution with READY state, but it moves states to FAILED for both in case of failure
func (jsm *JobExecutionStateMachine) CreateJobExecution(ctx context.Context) (dbError error, eventError error) {
	if jsm.JobExecution != nil {
		return fmt.Errorf("job-execution already exists"), nil
	}
	if jsm.Request.GetJobState() != common.PENDING && jsm.Request.GetJobState() != common.PAUSED {
		return fmt.Errorf("invalid job-state %s", jsm.Request.GetJobState()), nil
	}

	var oldJobExec *types.JobExecution
	if jsm.Request.GetJobExecutionID() != "" {
		oldJobExec, _ = jsm.JobManager.GetJobExecution(jsm.Request.GetJobExecutionID())
	}

	// Note restart method of request repository sets last-execution-id to execution-id and nullifies execution-id
	if oldJobExec == nil || oldJobExec.JobState.IsTerminal() {
		// Saving job-execution record as READY
		jsm.JobExecution, dbError = jsm.JobManager.CreateJobExecution(types.NewJobExecution(jsm.Request))
		if dbError != nil {
			return fmt.Errorf("failed to save job execution due to %s",
				dbError.Error()), nil
		}
	} else {
		// Setting job-execution record to READY
		oldJobExec.JobRequestID = jsm.Request.GetID()
		dbError = jsm.JobManager.ResetJobExecutionStateToReady(oldJobExec)
		if dbError != nil {
			return fmt.Errorf("failed to update job execution for %s due to %s",
					oldJobExec.ID,
					dbError.Error()),
				nil
		}
		jsm.JobExecution = oldJobExec
	}

	// Set request state to READY and save execution-id
	// Rollback job execution if it can't update request table
	if dbError = jsm.JobManager.SetJobRequestReadyToExecute(
		jsm.Request.GetID(),
		jsm.JobExecution.ID,
		jsm.Request.GetLastJobExecutionID()); dbError != nil {
		return fmt.Errorf("failed to set request status to EXECUTING due to %s", dbError.Error()),
			nil
	}

	// update local references
	jsm.Request.SetJobExecutionID(jsm.JobExecution.ID)
	jsm.Request.SetJobState(common.READY)

	// in case of failure, sendLaunchJobEvent will put the job back in PENDING state
	if eventError = jsm.sendLaunchJobEvent(ctx); eventError != nil {
		return nil,
			fmt.Errorf("failed to send launch event after creating job-execution due to %s", eventError)
	}
	return nil, nil
}

// UpdateJobRequestTimestampAndCheckQuota updates timestamp so that job scheduler doesn't consider it as orphan
func (jsm *JobExecutionStateMachine) UpdateJobRequestTimestampAndCheckQuota(_ context.Context) (err error) {
	if jsm.serverCfg.SubscriptionQuotaEnabled && jsm.serverCfg.Common.Auth.Enabled {
		if jsm.User == nil {
			err = fmt.Errorf("quota-error: user not found for execution")
			return common.NewQuotaExceededError(err)
		}
		if jsm.User.IsAdmin() {
			// do nothing
		} else if jsm.User.Subscription != nil {
			if jsm.User.Subscription.Expired() {
				err = fmt.Errorf("quota-error: user subscription of user %s is expired for execution", jsm.User.ID)
				return common.NewQuotaExceededError(err)
			}
			secs := time.Now().Sub(jsm.JobExecution.StartedAt).Seconds()
			if jsm.cpuUsage.Value+int64(secs) >= jsm.User.Subscription.CPUQuota {
				err = fmt.Errorf("quota-error: exceeded running cpu quota %d secs, usage %s, job time %f of user %s for execution",
					jsm.User.Subscription.CPUQuota, jsm.cpuUsage.ValueString(), secs, jsm.User.ID)
				return common.NewQuotaExceededError(err)
			}
		} else { // subscription is nil
			err = fmt.Errorf("quota-error: user subscription not found for execution")
			return common.NewQuotaExceededError(err)
		}
	}
	return jsm.JobManager.UpdateJobRequestTimestamp(jsm.Request.GetID())
}

// SetJobStatusToExecuting sets job request/execution status to EXECUTING
func (jsm *JobExecutionStateMachine) SetJobStatusToExecuting(_ context.Context) (err error) {
	// Mark job request and execution to EXECUTING (from READY unless execution was already running)
	if err = jsm.JobManager.SetJobRequestAndExecutingStatusToExecuting(
		jsm.JobExecution.ID); err != nil {
		return err
	}
	jsm.Request.SetJobState(common.EXECUTING)
	jsm.JobExecution.JobState = common.EXECUTING

	// treating error sending lifecycle event as non-fatal error
	// using fresh context in case deadline reached
	if eventError := jsm.sendJobExecutionLifecycleEvent(context.Background()); eventError != nil {
		logrus.WithFields(jsm.LogFields("JobSupervisor", eventError)).
			Warnf("failed to send job lifecycle event for %s after setting state to EXECUTING",
				jsm.JobDefinition.JobType)
	}
	return nil
}

// CanRetry checks if job can be retried in case of failure
func (jsm *JobExecutionStateMachine) CanRetry() bool {
	return jsm.Request.GetRetried() < jsm.JobDefinition.Retry+1 ||
		(jsm.errorCode != nil && jsm.errorCode.Action == common.RetryJob &&
			jsm.Request.GetRetried() < jsm.errorCode.Retry+1)
}

// ScheduleFailed is called if job scheduler fails to schedule a job request
func (jsm *JobExecutionStateMachine) ScheduleFailed(
	_ context.Context,
	err error,
	errorCode string) error {
	// No need to send any lifecycle event because job hasn't started - from PENDING | PAUSED
	return jsm.failed(jsm.revertState, false, err, errorCode)
}

// LaunchFailed is called when job-launcher fails to launch job and request and execution is marked as failed.
func (jsm *JobExecutionStateMachine) LaunchFailed(_ context.Context, err error) error {
	saveError := jsm.failed(common.READY, false, err, "")
	// treating error sending lifecycle event as non-fatal error
	// using fresh context in case deadline reached
	if eventError := jsm.sendJobExecutionLifecycleEvent(context.Background()); eventError != nil {
		logrus.WithFields(jsm.LogFields("JobLauncher", eventError, saveError)).
			Warnf("failed to send job lifecycle event after launch failure")
	}
	return saveError
}

// ExecutionCancelled is called when job is called by user
func (jsm *JobExecutionStateMachine) ExecutionCancelled(
	_ context.Context,
	errorCode string,
	errorMsg string) error {
	if jsm.JobExecution.JobState.IsTerminal() {
		return fmt.Errorf("job cannot be cancelled because it's already in terminal state")
	}

	if errorCode == "" {
		errorCode = common.ErrorJobCancelled
	}
	jsm.JobExecution.ErrorCode = errorCode
	jsm.JobExecution.ErrorMessage = errorMsg

	// Note: we won't fire any lifecycle event because that is fired upon cancellation

	return jsm.failed(common.EXECUTING, true, fmt.Errorf("%s", errorMsg), errorCode)
}

// ExecutionFailed is called when job fails to execute
func (jsm *JobExecutionStateMachine) ExecutionFailed(
	_ context.Context,
	errorCode string,
	errorMsg string) error {
	if jsm.JobExecution.JobState.IsTerminal() {
		return fmt.Errorf("job cannot be failed because it's already in terminal state")
	}

	if errorCode == "" {
		errorCode = common.ErrorJobExecute
	}
	jsm.JobExecution.ErrorCode = errorCode
	jsm.JobExecution.ErrorMessage = errorMsg
	saveError := jsm.failed(common.EXECUTING, false, fmt.Errorf("%s", errorMsg), errorCode)

	// treating error sending lifecycle event as non-fatal error
	// using fresh context in case deadline reached
	if eventError := jsm.sendJobExecutionLifecycleEvent(context.Background()); eventError != nil {
		logrus.WithFields(jsm.LogFields("JobSupervisor", eventError, saveError)).
			Warnf("failed to send job lifecycle event for %s after execution failure",
				jsm.JobDefinition.JobType)
	}
	return saveError
}

// ExecutionCompleted is called when job completes successfully
func (jsm *JobExecutionStateMachine) ExecutionCompleted(
	_ context.Context) (saveError error) {
	now := time.Now()

	jsm.Request.SetJobState(common.COMPLETED)
	jsm.JobExecution.JobState = common.COMPLETED
	jsm.JobExecution.ErrorCode = ""
	jsm.JobExecution.ErrorMessage = ""
	jsm.JobExecution.EndedAt = &now

	// Mark job request and execution to COMPLETED
	saveError = jsm.JobManager.FinalizeJobRequestAndExecutionState(
		jsm.QueryContext(),
		jsm.User,
		jsm.JobDefinition,
		jsm.Request,
		jsm.JobExecution,
		common.EXECUTING,
		0,
		jsm.Request.GetRetried(),
	)

	_, _ = jsm.JobManager.SaveAudit(types.NewAuditRecordFromJobRequest(
		jsm.Request,
		types.JobRequestCompleted,
		jsm.QueryContext()))

	jsm.MetricsRegistry.Incr(
		"job_completed_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	jsm.MetricsRegistry.Set(
		"job_completed_secs",
		float64(jsm.JobExecution.ElapsedMillis()/1000),
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// treating error sending lifecycle event as non-fatal error
	// using fresh context in case deadline reached
	if eventError := jsm.sendJobExecutionLifecycleEvent(context.Background()); eventError != nil {
		logrus.WithFields(jsm.LogFields("JobSupervisor", eventError, saveError)).
			Warnf("failed to send job lifecycle event for %s after job completed successfully",
				jsm.JobDefinition.JobType)
	} else {
		logrus.WithFields(jsm.LogFields("JobSupervisor", saveError)).
			Infof("job completed successfully for %s", jsm.JobDefinition.JobType)
	}

	return
}

// CheckAntResourcesAndConcurrencyForJob - Checks availability of the job and use backpressure to start only
// jobs that can proceed. Also, the request will be put back in the queue with a small delay so that
// it can be scheduled again with bounded exponential backoff.
func (jsm *JobExecutionStateMachine) CheckAntResourcesAndConcurrencyForJob() error {
	executing := jsm.JobManager.GetExecutionCount(jsm.Request)
	if executing >= int32(jsm.JobDefinition.MaxConcurrency) {
		return fmt.Errorf("cannot submit more than jobs because already running %d instances for %s",
			executing, jsm.JobDefinition.JobType)
	}
	if jsm.JobDefinition.GetUserID() != "" || jsm.JobDefinition.GetOrganizationID() != "" {
		executingUser, executingOrg := jsm.JobManager.UserOrgExecuting(jsm.Request)
		if jsm.User != nil && executingUser >= jsm.User.MaxConcurrency {
			return fmt.Errorf("cannot submit more than jobs because user already running %d jobs",
				executingUser)
		}
		if jsm.User != nil &&
			jsm.User.HasOrganization() &&
			executingOrg >= jsm.User.Organization.MaxConcurrency {
			return fmt.Errorf("cannot submit more than jobs because org already running %d jobs",
				executingOrg)
		}
	}
	// TODO check concurrent jobs by the same user/org
	tags := utils.SplitTags(jsm.JobDefinition.Tags)
	methods := make([]common.TaskMethod, 0)
	for _, m := range strings.Split(jsm.JobDefinition.Methods, ",") {
		m = strings.TrimSpace(m)
		methods = append(methods, common.TaskMethod(m))
	}
	return jsm.ResourceManager.HasAntsForJobTags(methods, tags)
}

// CheckSubscriptionQuota checks quota
func (jsm *JobExecutionStateMachine) CheckSubscriptionQuota() (err error) {
	jsm.cpuUsage, jsm.diskUsage, err = jsm.JobManager.CheckSubscriptionQuota(
		jsm.QueryContext(),
		jsm.User,
	)
	return err
}

// ReserveJobResources reserves ant resources when scheduling a job so that we don't over burden underlying
// ants beyond their capacity.
func (jsm *JobExecutionStateMachine) ReserveJobResources() (err error) {
	// Reserve resources for tasks
	jsm.Reservations, err = jsm.ResourceManager.ReserveJobResources(jsm.Request.GetID(), jsm.JobDefinition)
	return
}

// PauseJob puts back the job in PAUSED state from executing
func (jsm *JobExecutionStateMachine) PauseJob() (saveError error) {
	if jsm.JobExecution.JobState != common.EXECUTING {
		return fmt.Errorf("job cannot be paused because it's not executing")
	}
	// release ant resources for the job
	jsm.forceReleaseJobResources()
	now := time.Now()
	jsm.Request.SetJobState(common.PAUSED)
	jsm.JobExecution.EndedAt = &now
	jsm.JobExecution.JobState = common.PAUSED
	jsm.JobExecution.ErrorCode = common.ErrorPauseJob
	jsm.JobExecution.EndedAt = &now

	fields := jsm.LogFields("JobExecutionStateMachine", nil)

	// Mark job request and execution to PAUSED
	saveError = jsm.JobManager.FinalizeJobRequestAndExecutionState(
		jsm.QueryContext(),
		jsm.User,
		jsm.JobDefinition,
		jsm.Request,
		jsm.JobExecution,
		common.EXECUTING,
		jsm.JobDefinition.GetPauseTime(),
		jsm.Request.IncrRetried(),
	)

	_, _ = jsm.JobManager.SaveAudit(types.NewAuditRecordFromJobRequest(
		jsm.Request,
		types.JobRequestPaused,
		jsm.QueryContext()))

	jsm.MetricsRegistry.Incr(
		"job_paused_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// treating error sending lifecycle event as non-fatal error
	// using fresh context in case deadline reached
	if eventError := jsm.sendJobExecutionLifecycleEvent(context.Background()); eventError != nil {
		fields["LifecycleNotificationsError"] = eventError
	}
	fields["JobExecutionState"] = jsm.JobExecution.JobState
	fields["JobExecutionErrorCode"] = jsm.JobExecution.ErrorCode
	fields["PauseTime"] = jsm.JobDefinition.GetPauseTime().String()
	if saveError == nil {
		logrus.WithFields(fields).Infof("paused job successfully.")
	} else {
		fields["saveErr"] = saveError
		logrus.WithFields(fields).Warnf("paused job failed to persist state.")
	}
	return
}

// RestartJobBackToPendingPaused puts back the job in PENDING|PAUSED state from executing
func (jsm *JobExecutionStateMachine) RestartJobBackToPendingPaused(err error) (saveError error) {
	// release ant resources for the job
	jsm.forceReleaseJobResources()
	fields := jsm.LogFields("JobExecutionStateMachine",
		fmt.Errorf("setting job back to %s due to %s after %s secs - %s",
			jsm.revertState,
			err,
			jsm.JobDefinition.GetDelayBetweenRetries().String(),
			reflect.TypeOf(jsm.Request)))

	jsm.MetricsRegistry.Incr(
		"job_restart_to_pending_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// setting job request back to PENDING|PAUSED
	if saveRequestErr := jsm.JobManager.UpdateJobRequestState(
		jsm.QueryContext(),
		jsm.Request,
		common.EXECUTING,
		jsm.revertState,
		err.Error(),
		"",
		jsm.JobDefinition.GetDelayBetweenRetries(),
		jsm.Request.IncrRetried(),
		true); saveRequestErr != nil {
		saveError = saveRequestErr
		fields["saveRequestErr"] = saveRequestErr
	}

	jsm.Request.SetJobState(jsm.revertState)
	logrus.WithFields(fields).Warn(err)
	return
}

// RevertRequestToPendingPaused puts back the job in PENDING|PAUSED state
func (jsm *JobExecutionStateMachine) RevertRequestToPendingPaused(err error) (saveError error) {
	// release ant resources for the job
	jsm.forceReleaseJobResources()
	now := time.Now()
	fields := jsm.LogFields("JobExecutionStateMachine", err)

	jsm.MetricsRegistry.Incr(
		"job_reverted_to_pending_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// setting job request back to PENDING|PAUSED
	if saveRequestErr := jsm.JobManager.UpdateJobRequestState(
		jsm.QueryContext(),
		jsm.Request,
		common.READY,
		jsm.revertState,
		err.Error(),
		"",
		jsm.JobDefinition.GetDelayBetweenRetries(),
		0,
		false); saveRequestErr != nil {
		saveError = saveRequestErr
		fields["saveRequestErr"] = saveRequestErr
	}
	jsm.Request.SetJobState(jsm.revertState)

	if jsm.JobExecution != nil {
		jsm.JobExecution.JobState = common.DELETED
		jsm.JobExecution.EndedAt = &now
		if saveExecErr := jsm.JobManager.DeleteJobExecution(
			jsm.JobExecution.ID,
		); saveExecErr != nil {
			fields["saveExecErr"] = saveExecErr
			saveError = saveExecErr
		}
	}

	logrus.WithFields(fields).Warn(err)
	return
}

// DoesRequireFullRestart returns true if job needs to be fully restarted
func (jsm *JobExecutionStateMachine) DoesRequireFullRestart() bool {
	return jsm.JobDefinition.HardResetAfterRetries > 0 &&
		jsm.Request.GetRetried() > 0 &&
		jsm.Request.GetRetried()%(jsm.JobDefinition.HardResetAfterRetries+1) == 0
}

// QueryContext builds query context
func (jsm *JobExecutionStateMachine) QueryContext() *common.QueryContext {
	if jsm.User != nil && jsm.User.IsAdmin() {
		return common.NewQueryContext(nil, "").WithAdmin()
	}
	return common.NewQueryContextFromIDs(jsm.Request.GetUserID(), jsm.Request.GetOrganizationID())
}

// LogFields common logging fields
func (jsm *JobExecutionStateMachine) LogFields(component string, err ...error) logrus.Fields {
	fields := logrus.Fields(map[string]interface{}{
		"Component":          component,
		"RequestID":          jsm.Request.GetID(),
		"JobType":            jsm.Request.GetJobType(),
		"RequestState":       jsm.Request.GetJobState(),
		"RequestRetry":       jsm.Request.GetRetried(),
		"CronTriggered":      jsm.Request.GetCronTriggered(),
		"Priority":           jsm.Request.GetJobPriority(),
		"Scheduled":          jsm.Request.GetScheduledAt(),
		"ScheduleAttempts":   jsm.Request.GetScheduleAttempts(),
		"LastJobExecutionID": jsm.Request.GetLastJobExecutionID(),
	})

	if jsm.JobDefinition != nil {
		fields["JobDefinitionID"] = jsm.JobDefinition.ID
		fields["JobTimeout"] = jsm.JobDefinition.Timeout
		fields["ReportStdout"] = jsm.JobDefinition.ReportStdoutTask()
		fields["DefaultJobTimeout"] = jsm.serverCfg.Common.MaxJobTimeout
	}

	if jsm.JobExecution != nil {
		fields["JobExecutionState"] = jsm.JobExecution.JobState
		fields["JobExecutionID"] = jsm.JobExecution.ID
		fields["ExecutionStarted"] = jsm.JobExecution.StartedAt
		fields["ExecutionEnded"] = jsm.JobExecution.EndedAt
		fields["ExecutionErrorCode"] = jsm.JobExecution.ErrorCode
		fields["ExecutionErrorMessage"] = jsm.JobExecution.ErrorMessage
	}

	for i, e := range err {
		if e != nil {
			fields[fmt.Sprintf("Error%d", i+1)] = e
		}
	}
	return fields
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (jsm *JobExecutionStateMachine) failed(
	oldState common.RequestState,
	cancelled bool,
	err error,
	errorCode string) error {
	now := time.Now()
	var saveExecErr error
	if errorCode == "" {
		errorCode = common.ErrorJobExecute
	}
	switch jsm.Request.(type) {
	case *types.JobRequest:
		jsm.Request.(*types.JobRequest).ErrorMessage = err.Error()
		jsm.Request.(*types.JobRequest).ErrorCode = errorCode
	}
	jsm.Request.SetJobState(common.FAILED)
	newState := common.FAILED
	if cancelled {
		newState = common.CANCELLED
		errorCode = common.ErrorJobCancelled
	}
	jsm.MetricsRegistry.Incr(
		"job_failed_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// Try saving job-execution
	if jsm.JobExecution != nil {
		jsm.JobExecution.EndedAt = &now
		jsm.JobExecution.JobState = newState
		jsm.JobExecution.ErrorMessage = err.Error()
		jsm.JobExecution.ErrorCode = errorCode
		jsm.JobExecution.JobRequestID = jsm.Request.GetID()
		// update job-request and execution state
		saveExecErr = jsm.JobManager.FinalizeJobRequestAndExecutionState(
			jsm.QueryContext(),
			jsm.User,
			jsm.JobDefinition,
			jsm.Request,
			jsm.JobExecution,
			oldState,
			jsm.JobDefinition.GetDelayBetweenRetries(),
			jsm.Request.GetRetried(),
		)
		_, _ = jsm.JobManager.SaveAudit(types.NewAuditRecordFromJobRequest(
			jsm.Request,
			types.JobRequestFailed,
			jsm.QueryContext()))
	}
	delayBetweenRetries := time.Second
	if jsm.JobDefinition != nil {
		delayBetweenRetries = jsm.JobDefinition.GetDelayBetweenRetries()
	}
	// if failed to save job execution or don't have job-execution then just update job request
	if jsm.JobExecution == nil || saveExecErr != nil {
		jsm.Request.SetJobState(newState)
		if jsm.JobExecution != nil {
			//_ = jsm.jobExecutionRepository.Delete(jsm.JobExecution.ID)
			//jsm.JobExecution.JobState = common.DELETED
			jsm.JobExecution.EndedAt = &now
		}

		// update job-request
		saveExecErr = jsm.JobManager.UpdateJobRequestState(
			jsm.QueryContext(),
			jsm.Request,
			oldState,
			newState,
			err.Error(),
			errorCode,
			delayBetweenRetries,
			0,
			false)
	}

	fields := jsm.LogFields("JobExecutionStateMachine", err)
	if saveExecErr != nil {
		fields["saveExecErr"] = saveExecErr
	}
	//debug.PrintStack()
	logrus.WithFields(fields).Warnf("failed to execute job")
	return saveExecErr
}

// buildSecretConfigs builds secret configs
func (jsm *JobExecutionStateMachine) buildSecretConfigs() []string {
	res := make([]string, 0)
	if jsm.User != nil && jsm.User.HasOrganization() {
		for _, v := range jsm.User.Organization.Configs {
			if v.Secret {
				res = append(res, v.Value)
			}
		}
	}
	if jsm.JobDefinition != nil {
		for _, v := range jsm.JobDefinition.Configs {
			if v.Secret {
				res = append(res, v.Value)
			}
		}
	}
	return res
}

// buildDynamicParams builds config params
func (jsm *JobExecutionStateMachine) buildDynamicConfigs() map[string]common.VariableValue {
	res := make(map[string]common.VariableValue)
	if jsm.User != nil && jsm.User.HasOrganization() {
		for _, v := range jsm.User.Organization.Configs {
			if vv, err := v.GetVariableValue(); err == nil {
				res[v.Name] = vv
			}
		}
	}
	if jsm.JobDefinition != nil {
		cfg := jsm.JobDefinition.GetDynamicConfigAndVariables(nil)
		for k, v := range cfg {
			res[k] = v
		}
	}
	return res
}

// buildDynamicParams builds job params
func (jsm *JobExecutionStateMachine) buildDynamicParams(taskDefParams map[string]common.VariableValue) map[string]common.VariableValue {
	res := jsm.buildDynamicConfigs()
	if jsm.User != nil {
		res["UserID"] = common.NewVariableValue(jsm.User.ID, false)
		if jsm.User.HasOrganization() {
			res["OrganizationID"] = common.NewVariableValue(jsm.User.OrganizationID, false)
		}
	}
	res["JobID"] = common.NewVariableValue(jsm.Request.GetID(), false)
	res["JobType"] = common.NewVariableValue(jsm.JobDefinition.JobType, false)
	res["JobRetry"] = common.NewVariableValue(jsm.Request.GetRetried(), false)
	res["JobElapsedSecs"] = common.NewVariableValue(uint64(time.Since(jsm.StartedAt).Seconds()), false)
	for k, v := range taskDefParams {
		res[k] = v
	}
	for _, next := range jsm.Request.GetParams() {
		if vv, err := next.GetVariableValue(); err == nil {
			res[next.Name] = vv
		}
	}
	if jsm.JobExecution != nil {
		for _, next := range jsm.JobExecution.Contexts {
			if vv, err := next.GetVariableValue(); err == nil {
				res[next.Name] = vv
			}
		}
	}
	return res
}

func (jsm *JobExecutionStateMachine) forceReleaseJobResources() {
	if jsm.Reservations != nil {
		_ = jsm.ResourceManager.ReleaseJobResources(jsm.Request.GetID())
	}
}

// Fire event to notify job state
func (jsm *JobExecutionStateMachine) sendJobExecutionLifecycleEvent(ctx context.Context) (err error) {
	if jsm.JobExecution == nil {
		return fmt.Errorf("job-execution is not set")
	}
	event := events.NewJobExecutionLifecycleEvent(
		jsm.id,
		jsm.Request.GetUserID(),
		jsm.Request.GetID(),
		jsm.Request.GetJobType(),
		jsm.JobExecution.ID,
		jsm.JobExecution.JobState,
		jsm.Request.GetJobPriority(),
		jsm.JobExecution.ContextMap(),
	)
	jsm.publishJobWebhook(ctx, event)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal job-execution event due to %w", err)
	}
	if _, err = jsm.QueueClient.Publish(ctx,
		jsm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		payload,
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
			"RequestID", jsm.Request.GetID(),
			"UserID", jsm.Request.GetUserID(),
		),
	); err != nil {
		return fmt.Errorf("failed to send job-execution event due to %w", err)
	}
	return nil
}

func (jsm *JobExecutionStateMachine) publishJobWebhook(ctx context.Context, event *events.JobExecutionLifecycleEvent) {
	if hook, err := jsm.JobDefinition.Webhook(jsm.buildDynamicParams(nil)); err == nil && hook != nil {
		hookEvent := events.NewWebhookJobEvent(event, hook)
		if hookPayload, err := hookEvent.Marshal(); err == nil {
			if _, err = jsm.QueueClient.Publish(ctx,
				jsm.serverCfg.Common.GetJobWebhookTopic(),
				hookPayload,
				queue.NewMessageHeaders(
					queue.DisableBatchingKey, "true",
					"RequestID", jsm.Request.GetID(),
					"UserID", jsm.Request.GetUserID(),
				),
			); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":       "JobExecutionStateMachine",
					"RequestID":       jsm.Request.GetID(),
					"UserID":          jsm.Request.GetUserID(),
					"Organization":    jsm.Request.GetOrganizationID(),
					"JobDefinitionID": jsm.JobDefinition.ID,
					"JobType":         jsm.JobDefinition.JobType,
					"Webhook":         hook,
					"Error":           err,
				}).Warnf("failed to publish job webhook event ...")
			}
		} else if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":       "JobExecutionStateMachine",
				"RequestID":       jsm.Request.GetID(),
				"UserID":          jsm.Request.GetUserID(),
				"Organization":    jsm.Request.GetOrganizationID(),
				"JobDefinitionID": jsm.JobDefinition.ID,
				"JobType":         jsm.JobDefinition.JobType,
				"Webhook":         hook,
				"Error":           err,
			}).Warnf("failed to marshal job webhook event ...")
		}
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":       "JobExecutionStateMachine",
			"RequestID":       jsm.Request.GetID(),
			"UserID":          jsm.Request.GetUserID(),
			"Organization":    jsm.Request.GetOrganizationID(),
			"JobDefinitionID": jsm.JobDefinition.ID,
			"JobType":         jsm.JobDefinition.JobType,
			"Webhook":         hook,
			"Error":           err,
		}).Warnf("failed to fetch job webhook ...")
	}
}

// Fire event to start the job by job-launcher that listens to incoming request in queue and executes it
func (jsm *JobExecutionStateMachine) sendLaunchJobEvent(
	ctx context.Context) error {
	// Create event to start new job
	initiateEvent, err := events.NewJobExecutionLaunchEvent(
		jsm.id,
		jsm.Request.GetUserID(),
		jsm.Request.GetID(),
		jsm.Request.GetJobType(),
		jsm.JobExecution.ID,
		jsm.Reservations).Marshal()

	// If failed, then rollback execution and job request state -- not retryable
	if err != nil {
		// change status from READY to FAILED
		return jsm.LaunchFailed(
			ctx,
			fmt.Errorf("failed to marshal launch event for job due to %w", err))
	}

	// Sends launch event to one of job launchers (using load balance via shared queue)
	if _, err = jsm.QueueClient.Send(
		ctx,
		jsm.serverCfg.GetJobExecutionLaunchTopic(),
		initiateEvent,
		queue.NewMessageHeaders(
			queue.ReusableTopicKey, "false",
			queue.MessageTarget, jsm.id,
		),
	); err != nil {
		return fmt.Errorf("failed to send launch event due to %w", err)
	}
	return nil
}
