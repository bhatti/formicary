package fsm

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
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
// - The job-launcher subscribes to JobExecutionLaunchEvent and starts job-supervisor to execute the job but
//   it reverts request/execution state from READY to FAILED if an error occurs.
// - The job-supervisor changes state of request/execution from READY to EXECUTING but
//   it reverts request/execution state from READY to FAILED if an error occurs.
// - The job-supervisor continues with task-execution and changes state of request/execution from EXECUTING
//   to FAILED/COMPLETED if a task fails.
// - A user may cancel a job externally and in that case the job must be cancelled.
// Note: a job request can be rerun so it may change its state to PENDING if it's retried or re-queued but
//   the job execution is immutable and cannot change its state from terminal to non-terminal.
//

// JobExecutionStateMachine for managing state of request and its execution
type JobExecutionStateMachine struct {
	QueueClient         queue.Client
	JobManager          *manager.JobManager
	ArtifactManager     *manager.ArtifactManager
	ErrorCodeRepository repository.ErrorCodeRepository
	userRepository      repository.UserRepository
	orgRepository       repository.OrganizationRepository
	ResourceManager     resource.Manager
	MetricsRegistry     *metrics.Registry
	Request             types.IJobRequest
	JobDefinition       *types.JobDefinition
	JobExecution        *types.JobExecution
	LastJobExecution    *types.JobExecution
	User                *common.User
	Organization        *common.Organization
	Reservations        map[string]*common.AntReservation
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
	errorCodeRepository repository.ErrorCodeRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	resourceManager resource.Manager,
	metricsRegistry *metrics.Registry,
	request types.IJobRequest,
	reservations map[string]*common.AntReservation) *JobExecutionStateMachine {
	return &JobExecutionStateMachine{
		id:                  fmt.Sprintf("%s-websocket-gateway-%d", serverCfg.ID, request.GetID()),
		serverCfg:           serverCfg,
		QueueClient:         queueClient,
		JobManager:          jobManager,
		ArtifactManager:     artifactManager,
		userRepository:      userRepository,
		orgRepository:       orgRepository,
		ErrorCodeRepository: errorCodeRepository,
		ResourceManager:     resourceManager,
		MetricsRegistry:     metricsRegistry,
		Request:             request,
		Reservations:        reservations,
	}
}

// Validate validates
func (jsm *JobExecutionStateMachine) Validate() (err error) {
	// Validate validates required fields
	if jsm.serverCfg == nil {
		return fmt.Errorf("server-config is not specified")
	}
	if jsm.userRepository == nil {
		return fmt.Errorf("userRepository is not specified")
	}
	if jsm.orgRepository == nil {
		return fmt.Errorf("orgRepository is not specified")
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

	// loading user and organization from request
	if jsm.Request.GetUserID() != "" {
		jsm.User, err = jsm.userRepository.Get(
			jsm.QueryContext(),
			jsm.Request.GetUserID())
		if err != nil {
			return err
		}
	}

	if jsm.Request.GetOrganizationID() != "" {
		jsm.Organization, err = jsm.orgRepository.Get(
			jsm.QueryContext(),
			jsm.Request.GetOrganizationID())
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
		jsm.User, err = jsm.userRepository.Get(
			jsm.QueryContext(),
			jsm.JobDefinition.UserID)
		if err != nil {
			return err
		}
	}
	if jsm.Organization == nil && jsm.JobDefinition.OrganizationID != "" && !jsm.JobDefinition.PublicPlugin {
		jsm.Organization, err = jsm.orgRepository.Get(
			jsm.QueryContext(),
			jsm.JobDefinition.OrganizationID)
		if err != nil {
			return err
		}
	}

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
	}).Infof("validated request")

	return
}

// ShouldFilter checks if job should be filtered
func (jsm *JobExecutionStateMachine) ShouldFilter() error {
	if jsm.JobDefinition == nil {
		return nil
	}
	data := jsm.buildDynamicParams()
	if jsm.JobDefinition.Filtered(data) {
		return fmt.Errorf("job filtered due to %s with params %v", jsm.JobDefinition.Filter(), data)
	}
	return nil
}

// PrepareLaunch initializes definition/request/execution objects by job-launcher before execution by job-supervisor
func (jsm *JobExecutionStateMachine) PrepareLaunch(jobExecutionID string) (err error) {
	if err = jsm.Validate(); err != nil {
		return err
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
		return fmt.Errorf("mismatched job-execution-id %s in request %s",
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
		return fmt.Errorf("mismatched request status %s, job-execution state %s",
			jsm.Request.GetJobState(), jsm.JobExecution.JobState)
	}

	return
}

// CreateJobExecution moves the state from PENDING to READY for request and creates a new
// job-execution with READY state but it moves states to FAILED for both in case of failure
func (jsm *JobExecutionStateMachine) CreateJobExecution(ctx context.Context) (dbError error, eventError error) {
	if jsm.JobExecution != nil {
		return fmt.Errorf("job-execution already exists"), nil
	}
	if jsm.Request.GetJobState() != common.PENDING {
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
	// Rollback job execution if can't update request table
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
	if jsm.serverCfg.SubscriptionQuotaEnabled {
		if jsm.User != nil && jsm.User.Subscription != nil {
			if jsm.User.Subscription.Expired() {
				err = fmt.Errorf("quota-error: user subscription is expired")
				return common.NewQuotaExceededError(err)
			}
			secs := time.Now().Sub(jsm.JobExecution.StartedAt).Seconds()
			if jsm.cpuUsage.Value+int64(secs) >= jsm.User.Subscription.CPUQuota {
				err = fmt.Errorf("quota-error: exceeded running cpu quota %d secs, usage %s, job time %f",
					jsm.User.Subscription.CPUQuota, jsm.cpuUsage.ValueString(), secs)
				return common.NewQuotaExceededError(err)
			}
		} else {
			err = fmt.Errorf("quota-error: user subscription not found for user %s", jsm.User)
			return common.NewQuotaExceededError(err)
		}
	}
	return jsm.JobManager.UpdateJobRequestTimestamp(jsm.Request.GetID())
}

// SetJobStatusToExecuting sets job request/execution status to EXECUTING
func (jsm *JobExecutionStateMachine) SetJobStatusToExecuting(ctx context.Context) (err error) {
	// Mark job request and execution to EXECUTING (from READY unless execution was already running)
	if err = jsm.JobManager.SetJobRequestAndExecutingStatusToExecuting(
		jsm.JobExecution.ID); err != nil {
		return err
	}
	jsm.Request.SetJobState(common.EXECUTING)
	jsm.JobExecution.JobState = common.EXECUTING

	// treating error sending lifecycle event as non-fatal error
	if eventError := jsm.sendJobExecutionLifecycleEvent(ctx); eventError != nil {
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
	// No need to send any lifecycle event because job hasn't started
	return jsm.failed(common.PENDING, false, err, errorCode)
}

// LaunchFailed is called when job-launcher fails to launch job and request and execution is marked as failed.
func (jsm *JobExecutionStateMachine) LaunchFailed(ctx context.Context, err error) error {
	saveError := jsm.failed(common.READY, false, err, "")
	// treating error sending lifecycle event as non-fatal error
	if eventError := jsm.sendJobExecutionLifecycleEvent(ctx); eventError != nil {
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
		return fmt.Errorf("job cannot be failed because it's already in terminal state")
	}

	if errorCode == "" {
		errorCode = common.ErrorJobCancelled
	}
	jsm.JobExecution.ErrorCode = errorCode
	jsm.JobExecution.ErrorMessage = errorMsg

	// Note: we won't fire any lifecycle event because that is fired upon cancellation

	return jsm.failed(common.EXECUTING, true, fmt.Errorf(errorMsg), common.ErrorJobCancelled)
}

// ExecutionFailed is called when job fails to execute
func (jsm *JobExecutionStateMachine) ExecutionFailed(
	ctx context.Context,
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
	saveError := jsm.failed(common.EXECUTING, false, fmt.Errorf(errorMsg), errorCode)

	// treating error sending lifecycle event as non-fatal error
	if eventError := jsm.sendJobExecutionLifecycleEvent(ctx); eventError != nil {
		logrus.WithFields(jsm.LogFields("JobSupervisor", eventError, saveError)).
			Warnf("failed to send job lifecycle event for %s after execution failure",
				jsm.JobDefinition.JobType)
	}
	return saveError
}

// ExecutionCompleted is called when job completes successfully
func (jsm *JobExecutionStateMachine) ExecutionCompleted(ctx context.Context) (saveError error) {
	now := time.Now()

	jsm.Request.SetJobState(common.COMPLETED)
	jsm.JobExecution.JobState = common.COMPLETED
	jsm.JobExecution.ErrorCode = ""
	jsm.JobExecution.ErrorMessage = ""
	jsm.JobExecution.EndedAt = &now

	// Mark job request and execution to COMPLETED
	saveError = jsm.JobManager.FinalizeJobRequestAndExecutionState(
		jsm.QueryContext(),
		jsm.Request,
		jsm.JobExecution,
		common.EXECUTING,
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
	if eventError := jsm.sendJobExecutionLifecycleEvent(ctx); eventError != nil {
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
		if jsm.Organization != nil && executingOrg >= jsm.Organization.MaxConcurrency {
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
		jsm.Organization)
	return err
}

// ReserveJobResources reserves ant resources when scheduling a job so that we don't over burden underlying
// ants beyond their capacity.
func (jsm *JobExecutionStateMachine) ReserveJobResources() (err error) {
	// Reserve resources for tasks
	jsm.Reservations, err = jsm.ResourceManager.ReserveJobResources(jsm.Request.GetID(), jsm.JobDefinition)
	return
}

// RevertRequestToPending puts back the job in PENDING state
func (jsm *JobExecutionStateMachine) RevertRequestToPending(err error) (saveError error) {
	// release ant resources for the job
	jsm.forceReleaseJobResources()
	now := time.Now()
	fields := jsm.LogFields("JobScheduler", err)

	jsm.MetricsRegistry.Incr(
		"job_reverted_to_pending_total",
		map[string]string{
			"Org": jsm.Request.GetOrganizationID(),
			"Job": jsm.Request.GetJobType()})

	// setting job request back to PENDING
	if saveRequestErr := jsm.JobManager.UpdateJobRequestState(
		jsm.QueryContext(),
		jsm.Request,
		common.READY,
		common.PENDING,
		err.Error(),
		""); saveRequestErr != nil {
		saveError = saveRequestErr
		fields["saveRequestErr"] = saveRequestErr
	}
	jsm.Request.SetJobState(common.PENDING)

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
	return common.NewQueryContext(jsm.Request.GetUserID(), jsm.Request.GetOrganizationID(), "")
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
		fields["DefaultJobTimeout"] = jsm.serverCfg.MaxJobTimeout
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

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
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
			jsm.Request,
			jsm.JobExecution,
			oldState,
			jsm.Request.GetRetried(),
		)
		_, _ = jsm.JobManager.SaveAudit(types.NewAuditRecordFromJobRequest(
			jsm.Request,
			types.JobRequestFailed,
			jsm.QueryContext()))
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
			errorCode)
	}

	fields := jsm.LogFields("JobExecutionStateMachine", err)
	if saveExecErr != nil {
		fields["saveExecErr"] = saveExecErr
	}
	//debug.PrintStack()
	if jsm.JobDefinition != nil {
		logrus.WithFields(fields).Errorf("failed to execute job for '%s'",
			jsm.JobDefinition.JobType)
	}
	return saveExecErr
}

// buildSecretConfigs builds secret configs
func (jsm *JobExecutionStateMachine) buildSecretConfigs() []string {
	res := make([]string, 0)
	if jsm.Organization != nil {
		for _, v := range jsm.Organization.Configs {
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
func (jsm *JobExecutionStateMachine) buildDynamicConfigs() map[string]interface{} {
	res := make(map[string]interface{})
	if jsm.Organization != nil {
		for _, v := range jsm.Organization.Configs {
			res[v.Name], _ = v.GetParsedValue()
		}
	}
	if jsm.JobDefinition != nil {
		for _, v := range jsm.JobDefinition.Configs {
			res[v.Name], _ = v.GetParsedValue()
		}
	}
	return res
}

// buildDynamicParams builds job params
func (jsm *JobExecutionStateMachine) buildDynamicParams() map[string]interface{} {
	res := jsm.buildDynamicConfigs()
	if n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64)); err == nil {
		res["Nonce"] = n
	}
	res["JobID"] = jsm.Request.GetID()
	res["JobType"] = jsm.JobDefinition.JobType
	for k, v := range jsm.JobDefinition.NameValueVariables.(map[string]interface{}) {
		res[k] = v
	}
	switch req := jsm.Request.(type) {
	case *types.JobRequest:
		for k, v := range req.NameValueParams {
			res[k] = v
		}
	}
	if jsm.JobExecution != nil {
		for _, v := range jsm.JobExecution.Contexts {
			res[v.Name], _ = v.GetParsedValue()
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
		jsm.buildDynamicParams(),
	)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal job-execution event due to %v", err)
	}
	if _, err = jsm.QueueClient.Publish(ctx,
		jsm.serverCfg.GetJobExecutionLifecycleTopic(),
		make(map[string]string),
		payload,
		true); err != nil {
		return fmt.Errorf("failed to send job-execution event due to %v", err)
	}
	return nil
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
			fmt.Errorf("failed to marshal launch event for job due to %s", err.Error()))
	}

	// Sends launch event to one of job launchers (using load balance via shared queue)
	if _, err = jsm.QueueClient.Send(
		ctx,
		jsm.serverCfg.GetJobExecutionLaunchTopic(),
		make(map[string]string),
		initiateEvent,
		true,
	); err != nil {
		return fmt.Errorf("failed to send launch event due to %s", err.Error())
	}
	return nil
}
