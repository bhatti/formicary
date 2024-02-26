package manager

import (
	"context"
	"fmt"
	"plexobject.com/formicary/queen/security"
	"strings"
	"time"

	"plexobject.com/formicary/queen/notify"

	yaml "gopkg.in/yaml.v3"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/queen/stats"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/dot"
	"plexobject.com/formicary/queen/resource"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

const maxForkJobs = 10

// JobManager for managing state of request and its execution
type JobManager struct {
	serverCfg               *config.ServerConfig
	auditRecordRepository   repository.AuditRecordRepository
	jobDefinitionRepository repository.JobDefinitionRepository
	jobRequestRepository    repository.JobRequestRepository
	jobExecutionRepository  repository.JobExecutionRepository
	userManager             *UserManager
	resourceManager         resource.Manager
	artifactManager         *ArtifactManager
	jobStatsRegistry        *stats.JobStatsRegistry
	metricsRegistry         *metrics.Registry
	queueClient             queue.Client
	jobsNotifier            notify.Notifier
	jobIdsTicker            *time.Ticker
}

// NewJobManager manages job request, definition and execution
func NewJobManager(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	auditRecordRepository repository.AuditRecordRepository,
	jobDefinitionRepository repository.JobDefinitionRepository,
	jobRequestRepository repository.JobRequestRepository,
	jobExecutionRepository repository.JobExecutionRepository,
	userManager *UserManager,
	resourceManager resource.Manager,
	artifactManager *ArtifactManager,
	jobStatsRegistry *stats.JobStatsRegistry,
	metricsRegistry *metrics.Registry,
	queueClient queue.Client,
	jobsNotifier notify.Notifier) (*JobManager, error) {
	if serverCfg == nil {
		return nil, fmt.Errorf("server-config is not specified")
	}
	if jobDefinitionRepository == nil {
		return nil, fmt.Errorf("job-definition-repository is not specified")
	}
	if auditRecordRepository == nil {
		return nil, fmt.Errorf("audit-repository is not specified")
	}
	if jobRequestRepository == nil {
		return nil, fmt.Errorf("job-request-repository is not specified")
	}
	if jobExecutionRepository == nil {
		return nil, fmt.Errorf("job-execution-repository is not specified")
	}
	if userManager == nil {
		return nil, fmt.Errorf("user-manager is not specified")
	}
	if resourceManager == nil {
		return nil, fmt.Errorf("resource-manager is not specified")
	}
	if artifactManager == nil {
		return nil, fmt.Errorf("artifact-manager is not specified")
	}
	if jobStatsRegistry == nil {
		return nil, fmt.Errorf("jobStats-registry is not specified")
	}
	if queueClient == nil {
		return nil, fmt.Errorf("queue-client is not specified")
	}
	if jobsNotifier == nil {
		return nil, fmt.Errorf("jobs-notifier is not specified")
	}
	// initialize registry
	initializeStatsRegistry(jobRequestRepository, jobStatsRegistry)

	jm := &JobManager{
		serverCfg:               serverCfg,
		auditRecordRepository:   auditRecordRepository,
		jobDefinitionRepository: jobDefinitionRepository,
		jobRequestRepository:    jobRequestRepository,
		jobExecutionRepository:  jobExecutionRepository,
		userManager:             userManager,
		resourceManager:         resourceManager,
		artifactManager:         artifactManager,
		jobStatsRegistry:        jobStatsRegistry,
		metricsRegistry:         metricsRegistry,
		queueClient:             queueClient,
		jobsNotifier:            jobsNotifier,
	}

	if err := jm.startRecentlyCompletedJobIdsTicker(ctx); err != nil {
		return nil, err
	}
	return jm, nil
}

// SaveAudit - save persists audit-record
func (jm *JobManager) SaveAudit(
	record *types.AuditRecord) (*types.AuditRecord, error) {
	return jm.auditRecordRepository.Save(record)
}

/////////////////////////////////////////// JOB DEFINITION METHODS ////////////////////////////////////////////

// GetCronTriggeredJobTypes returns types of jobs that are triggered by cron
func (jm *JobManager) GetCronTriggeredJobTypes() ([]types.JobTypeCronTrigger, error) {
	cronTriggersByJobType, err := jm.jobDefinitionRepository.GetJobTypesAndCronTrigger(
		common.NewQueryContext(nil, "").WithAdmin())
	if err != nil {
		return nil, err
	}
	res := make([]types.JobTypeCronTrigger, 0)
	for _, jobInfo := range cronTriggersByJobType {
		if jobInfo.CronTrigger != "" {
			res = append(res, jobInfo)
		}
	}
	return res, nil
}

// GetJobTypesAsArray returns types of jobs
func (jm *JobManager) GetJobTypesAsArray(qc *common.QueryContext) ([]types.JobTypeCronTrigger, error) {
	return jm.jobDefinitionRepository.GetJobTypesAndCronTrigger(qc)
}

// QueryJobDefinitions query job definitions
func (jm *JobManager) QueryJobDefinitions(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobs []*types.JobDefinition, totalRecords int64, err error) {
	return jm.jobDefinitionRepository.Query(
		qc,
		params,
		page,
		pageSize,
		order)
}

// SaveJobDefinition saves job-definition and triggers request for cron-based triggers
func (jm *JobManager) SaveJobDefinition(
	qc *common.QueryContext,
	jobDefinition *types.JobDefinition) (*types.JobDefinition, error) {
	if jobDefinition.PublicPlugin && jobDefinition.UserID != "" {
		if jobDefinition.OrganizationID == "" {
			return nil, fmt.Errorf("public plugins is only supported for organizations")
		}
		org, err := jm.userManager.GetOrganization(qc, jobDefinition.OrganizationID)
		if err != nil {
			return nil, common.NewValidationError(
				fmt.Errorf("the organization not found: `%s`",
					jobDefinition.OrganizationID))
		}
		if !strings.HasPrefix(jobDefinition.JobType, org.BundleID) {
			return nil, common.NewValidationError(
				fmt.Errorf("the `job_type` of public plugins must start with organization bundle `%s`",
					org.BundleID))
		}
	}

	if err := jobDefinition.Validate(); err != nil {
		return nil, err
	}
	saved, err := jm.jobDefinitionRepository.Save(qc, jobDefinition)
	if err != nil {
		return nil, err
	}
	jm.metricsRegistry.Incr("job_definitions_updated_total", map[string]string{"JobType": jobDefinition.JobType})
	_ = jm.fireJobDefinitionChange(
		saved.UserID,
		saved.ID,
		saved.JobType,
		events.UPDATED)

	// check if this job requires cron trigger
	if cronScheduledAt, _ := jobDefinition.GetCronScheduleTimeAndUserKey(); cronScheduledAt != nil {
		if err = jm.scheduleCronRequest(saved, nil); err != nil {
			return nil, err
		}
	}
	return saved, nil
}

// schedule missing cron jobs
func (jm *JobManager) scheduleCronRequest(jobDefinition *types.JobDefinition, oldReq types.IJobRequest) error {
	request, err := types.NewJobRequestFromDefinition(jobDefinition)
	if err != nil {
		return err
	}
	executing := jm.GetExecutionCount(request)
	if executing > 0 {
		logrus.WithFields(logrus.Fields{
			"JobRequestID":      request.ID,
			"UserKey":           request.UserKey,
			"JobType":           request.JobType,
			"GetUserJobTypeKey": request.GetUserJobTypeKey(),
			"Executing":         executing,
		}).Warnf("skip scheduling cron job request because it's already running")
		return nil
	}
	qc := common.NewQueryContextFromIDs(request.UserID, request.OrganizationID)
	if oldReq != nil {
		var oldReqFull *types.JobRequest
		switch oldReq.(type) {
		case *types.JobRequest:
			oldReqFull = oldReq.(*types.JobRequest)
		case *types.JobRequestInfo:
			oldReqFull, _ = jm.jobRequestRepository.Get(qc, oldReq.GetID())
		}
		if oldReqFull != nil {
			for _, p := range oldReqFull.Params {
				v, _ := p.GetParsedValue()
				_, _ = request.AddParam(p.Name, v)
			}
		}
		request.ParentID = oldReq.GetID()
		request.UserID = oldReq.GetUserID()
		request.OrganizationID = oldReq.GetOrganizationID()
	}

	if _, err := jm.SaveJobRequest(qc, request); err != nil {
		if !strings.Contains(err.Error(), "Duplicate entry") {
			logrus.WithFields(logrus.Fields{
				"JobRequestID": request.ID,
				"UserKey":      request.UserKey,
				"JobType":      request.JobType,
				"Error":        err,
			}).Warnf("failed to schedule job after updating job-definition")
			return err
		}
	}
	return nil
}

// SetJobDefinitionMaxConcurrency - update concurrency of job-definition
func (jm *JobManager) SetJobDefinitionMaxConcurrency(
	qc *common.QueryContext,
	id string,
	concurrency int) (err error) {
	if err = jm.jobDefinitionRepository.SetMaxConcurrency(id, concurrency); err == nil {
		if jobDefinition, dbErr := jm.GetJobDefinition(
			qc,
			id); dbErr == nil {
			_ = jm.fireJobDefinitionChange(
				jobDefinition.UserID,
				id,
				jobDefinition.JobType,
				events.UPDATED)
		}
	}
	return
}

// RecentDeadIDs returns recently completed job-ids
func (jm *JobManager) RecentDeadIDs(
	limit int,
	fromOffset time.Duration,
	toOffset time.Duration,
) ([]uint64, error) {
	return jm.jobRequestRepository.RecentDeadIDs(limit, fromOffset, toOffset)
}

// RecentDeadIDs returns recently completed job-ids
func (jm *JobManager) publishDeadJobIds(ctx context.Context) (err error) {
	var ids []uint64
	if ids, err = jm.RecentDeadIDs(10000, jm.serverCfg.Common.MaxJobTimeout, 5*time.Second); err != nil {
		return err
	}
	event := events.NewRecentlyCompletedJobsEvent("JobManager", ids)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal recently-completed-job-ids event due to %w", err)
	}
	if _, err = jm.queueClient.Publish(
		ctx,
		jm.serverCfg.Common.GetRecentlyCompletedJobsTopic(),
		payload,
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
		),
	); err != nil {
		return fmt.Errorf("failed to send recently-completed-job-ids event due to %w", err)
	}
	return nil
}

// GetWaitEstimate calculates wait time for the job
func (jm *JobManager) GetWaitEstimate(
	qc *common.QueryContext,
	requestID uint64) (q stats.JobWaitEstimate, err error) {
	var request *types.JobRequest
	request, err = jm.GetJobRequest(qc, requestID)
	if err != nil {
		return
	}
	var jobDef *types.JobDefinition
	jobDef, err = jm.GetJobDefinition(qc, request.JobDefinitionID)
	if err != nil {
		return
	}

	// finds queue record from job stats
	q = jm.jobStatsRegistry.BuildWaitEstimate(request)
	q.ScheduledAt = request.ScheduledAt

	// calculate minimum ants available for the job (by iterating each task)
	availableAnts := 0
	// check resources available
	if reservations, err := jm.resourceManager.CheckJobResources(jobDef); err != nil {
		q.ErrorMessage = err.Error()
	} else if len(reservations) > 0 {
		q.JobStats.AntsCapacity = len(reservations)
		availableAnts = reservations[0].TotalReservations
		for i := 1; i < len(reservations); i++ {
			if reservations[i].TotalReservations < availableAnts {
				availableAnts = reservations[i].TotalReservations
			}
		}
	}
	// calculate average times
	q.JobStats.Calculate()

	// build job request info
	q.JobRequest = request.ToInfo()
	if !q.JobStats.AntsAvailable {
		q.ErrorMessage = q.JobStats.AntUnavailableError
	} else if q.JobStats.JobDisabled {
		q.ErrorMessage = fmt.Sprintf("job %s is disabled", request.JobType)
	}

	// if job is not pending then return
	if !request.JobState.Pending() {
		q.ErrorMessage = fmt.Sprintf("job %d is not pending but is %s", requestID, request.JobState)
		return
	}

	// match pending jobs using request id
	matched := false
	for i, id := range q.PendingJobIDs {
		if id == requestID {
			// number in the queue
			q.QueueNumber = i
			if q.JobStats.SucceededJobsAverage > 0 && availableAnts > 0 {
				// assuming 70% are close to finish
				total := i + 1 + int(float64(q.JobStats.ExecutingJobs)*0.7)
				q.EstimatedWait = time.Duration(float64(total)/float64(availableAnts)) * time.Duration(q.JobStats.SucceededJobsAverage) * time.Millisecond
			}
			matched = true
		}
	}

	// check if job was scheduled in future
	scheduledDiff := time.Now().Sub(request.ScheduledAt)
	if !matched {
		q.ErrorMessage = fmt.Sprintf("could not calculate estimated queue time for request %d and job %s",
			request.ID, request.JobType)
	} else if scheduledDiff > q.EstimatedWait {
		q.EstimatedWait = scheduledDiff
	}
	if q.EstimatedWait > 0 {
		q.ErrorMessage = ""
	}
	return
}

// DisableJobDefinition - update job-definition -- only admin can do it so no need for query context
func (jm *JobManager) DisableJobDefinition(
	qc *common.QueryContext,
	id string) (err error) {
	if err = jm.jobDefinitionRepository.SetDisabled(id, true); err == nil {
		if jobDefinition, dbErr := jm.GetJobDefinition(
			qc,
			id); dbErr == nil {
			jm.jobStatsRegistry.SetDisabled(jobDefinition, true)
			jm.metricsRegistry.Incr("job_definitions_disabled_total", map[string]string{"JobType": jobDefinition.JobType})
			_ = jm.fireJobDefinitionChange(
				jobDefinition.UserID,
				id,
				jobDefinition.JobType,
				events.DISABLED)
		}
	}
	return
}

// EnableJobDefinition - update job-definition -- only admin can do it so no need for query context
func (jm *JobManager) EnableJobDefinition(
	qc *common.QueryContext,
	id string) (err error) {
	if err = jm.jobDefinitionRepository.SetDisabled(id, false); err == nil {
		if jobDefinition, dbErr := jm.GetJobDefinition(
			qc,
			id); dbErr == nil {
			jm.jobStatsRegistry.SetDisabled(jobDefinition, false)
			jm.metricsRegistry.Incr("job_definitions_enabled_total", map[string]string{"JobType": jobDefinition.JobType})
			_ = jm.fireJobDefinitionChange(
				jobDefinition.UserID,
				id,
				jobDefinition.JobType,
				events.ENABLED)
		}
	}
	return
}

// GetJobDefinition - finds job-definition by id
func (jm *JobManager) GetJobDefinition(
	qc *common.QueryContext,
	id string) (*types.JobDefinition, error) {
	return jm.jobDefinitionRepository.Get(qc, id)
	// TODO
	//reservation, err := jobDefCtrl.resourceManager.CheckJobResources(job)
	//if err == nil {
	//}
}

// GetJobDefinitionByType - finds job-definition by type
func (jm *JobManager) GetJobDefinitionByType(
	qc *common.QueryContext,
	jobType string,
	version string) (*types.JobDefinition, error) {
	return jm.jobDefinitionRepository.GetByType(qc, jobType+":"+version)
}

// GetYamlJobDefinitionByType - finds job-definition by type
func (jm *JobManager) GetYamlJobDefinitionByType(
	qc *common.QueryContext,
	jobType string) ([]byte, error) {
	job, err := jm.GetJobDefinitionByType(qc, jobType, "")
	if err != nil {
		return nil, err
	}
	var b []byte
	if job.RawYaml == "" {
		b, err = yaml.Marshal(job)
		if err != nil {
			return nil, err
		}
	} else {
		b = []byte(job.RawYaml)
	}
	return b, nil
}

// DeleteJobDefinition - deletes job-definition by id
func (jm *JobManager) DeleteJobDefinition(
	qc *common.QueryContext,
	id string) (err error) {
	if err = jm.jobDefinitionRepository.Delete(qc, id); err == nil {
		if jobDefinition, dbErr := jm.GetJobDefinition(qc, id); dbErr == nil {
			jm.metricsRegistry.Incr("job_definitions_deleted_total", map[string]string{"JobType": jobDefinition.JobType})
			_ = jm.fireJobDefinitionChange(
				jobDefinition.UserID,
				id,
				jobDefinition.JobType,
				events.DELETED)
		}
	}
	return
}

// GetDotConfigForJobDefinition - creates graphviz dot file
func (jm *JobManager) GetDotConfigForJobDefinition(
	qc *common.QueryContext,
	id string) (string, error) {
	definition, err := jm.jobDefinitionRepository.Get(qc, id)
	if err != nil {
		return "", err
	}

	generator, err := dot.New(definition, nil)
	if err != nil {
		return "", err
	}
	d, err := generator.GenerateDot()
	if err != nil {
		return "", err
	}
	return d, nil
}

// GetDotImageForJobDefinition - creates graphviz png image file
func (jm *JobManager) GetDotImageForJobDefinition(
	qc *common.QueryContext,
	id string) ([]byte, error) {
	definition, err := jm.jobDefinitionRepository.Get(qc, id)
	if err != nil {
		return nil, err
	}

	generator, err := dot.New(definition, nil)
	if err != nil {
		return nil, err
	}
	return generator.GenerateDotImage()
}

/////////////////////////////////////////// JOB REQUEST METHODS ////////////////////////////////////////////

// SaveJobRequest saves request and updates scheduled-date from cron-based triggers
func (jm *JobManager) SaveJobRequest(
	qc *common.QueryContext,
	request *types.JobRequest) (saved *types.JobRequest, err error) {
	request.UserID = qc.GetUserID()
	request.OrganizationID = qc.GetOrganizationID()
	jobDefinition, err := jm.GetJobDefinitionByType(qc, request.GetJobType(), request.GetJobVersion())
	if err != nil {
		return nil, err
	}

	// Check quota
	if jm.serverCfg.SubscriptionQuotaEnabled && request.GetUserID() != "" {
		user, err := jm.userManager.GetUser(
			qc,
			request.GetUserID())
		if err != nil {
			return nil, err
		}
		_, _, err = jm.CheckSubscriptionQuota(qc, user)
		if err != nil {
			if jobDefinition.CronTrigger != "" {
				_ = jm.DisableJobDefinition(qc, jobDefinition.ID)
				logrus.WithFields(logrus.Fields{
					"Component":       "JobManager",
					"RequestID":       request.ID,
					"User":            request.UserID,
					"Organization":    request.OrganizationID,
					"JobType":         request.JobType,
					"UserKey":         request.UserKey,
					"CronTrigger":     jobDefinition.CronTrigger,
					"JobDefinitionID": jobDefinition.ID,
					"Params":          request.ParamString(),
					"Error":           err,
				}).Warnf("pausing cron job because subscription is out")
			}
			return nil, err
		}
	}

	request.JobState = common.PENDING
	request.JobDefinitionID = jobDefinition.ID
	request.JobExecutionID = ""
	request.ErrorCode = ""
	request.ErrorMessage = ""
	request.ScheduleAttempts = 0
	request.Retried = 0
	request.CreatedAt = time.Now()
	request.UpdatedAt = time.Now()
	request.UpdateUserKeyFromScheduleIfCronJob(jobDefinition)

	if !request.UpdateScheduledAtFromCronTrigger(jobDefinition) &&
		(request.ScheduledAt.IsZero() || request.ScheduledAt.Unix() < time.Now().Unix()-1) {
		request.ScheduledAt = time.Now()
	}

	// prevent fork bombing where job-A forks another job-A or forks job-B, which forks job-A
	forkedCount := 0
	for _, param := range request.Params {
		if strings.HasPrefix(param.Name, types.ParentJobTypePrefix) {
			forkedCount++
			if param.Value == request.JobType {
				return nil, common.NewValidationError(
					fmt.Errorf("cannot fork job of same type '%s'", request.JobType))
			}
		}
	}
	if forkedCount > maxForkJobs {
		return nil, common.NewValidationError(
			fmt.Errorf("cannot fork job more than %d jobs", maxForkJobs))
	}

	if len(jobDefinition.RequiredParams) > 0 {
		for _, name := range jobDefinition.RequiredParams {
			if request.GetParam(name) == nil && jobDefinition.GetVariable(name) == nil {
				return nil, common.NewValidationError(
					fmt.Errorf("failed to find required parameter for %s", name))
			}
		}
	}

	saved, err = jm.jobRequestRepository.Save(qc, request)
	if err == nil {
		_ = jm.fireJobRequestChange(saved)
		jm.metricsRegistry.Incr("job_submitted_total", map[string]string{"JobType": jobDefinition.JobType})
		jm.jobStatsRegistry.Pending(saved.ToInfo(), false)
		logrus.WithFields(logrus.Fields{
			"Component":    "JobManager",
			"RequestID":    request.ID,
			"User":         request.UserID,
			"Organization": request.OrganizationID,
			"JobType":      request.JobType,
			"UserKey":      request.UserKey,
			"Params":       request.ParamString(),
		}).Infof("saved request job")
	}
	return
}

// DeactivateOldCronRequest soft deletes old job request
func (jm *JobManager) DeactivateOldCronRequest(
	qc *common.QueryContext,
	request *types.JobRequest) error {
	// delete duplicate entry if exists
	if request.CronTriggered {
		return jm.jobRequestRepository.DeletePendingCronByJobType(qc, request.JobType)
	}
	return nil
}

// QueryJobRequests finds matching job-request by parameters
func (jm *JobManager) QueryJobRequests(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobRequests []*types.JobRequest, totalRecords int64, err error) {
	return jm.jobRequestRepository.Query(
		qc,
		params,
		page,
		pageSize,
		order)
}

// GetJobRequestParams - finds job-request-params by id
func (jm *JobManager) GetJobRequestParams(
	id uint64) ([]*types.JobRequestParam, error) {
	return jm.jobRequestRepository.GetParams(id)
}

// GetJobRequest - finds job-request by id
func (jm *JobManager) GetJobRequest(
	qc *common.QueryContext,
	id uint64) (*types.JobRequest, error) {
	request, err := jm.jobRequestRepository.Get(qc, id)
	if err != nil {
		return nil, err
	}
	if request.JobExecutionID != "" {
		request.Execution, err = jm.jobExecutionRepository.Get(request.JobExecutionID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"JobRequestID":   id,
				"JobType":        request.JobType,
				"JobExecutionID": request.JobExecutionID,
				"Error":          err,
			}).Error("failed to load job-execution for job request")
		} else {
			for _, task := range request.Execution.Tasks {
				for _, art := range task.Artifacts {
					jm.artifactManager.UpdateURL(context.Background(), art)
				}
			}
		}
	}
	return request, nil
}

// CancelJobRequest - cancels/stops job-request
func (jm *JobManager) CancelJobRequest(
	qc *common.QueryContext,
	id uint64) error {
	req, err := jm.jobRequestRepository.Get(qc, id)
	if err != nil {
		return err
	}
	jm.jobStatsRegistry.Cancelled(req)
	if req.JobState == common.PENDING || req.JobState == common.PAUSED {
		if err = jm.jobRequestRepository.Cancel(qc, id); err != nil {
			return err
		}
		req.JobState = common.CANCELLED
		_ = jm.fireJobRequestChange(req)
	} else {
		if err = jm.cancelJob(qc, req); err != nil {
			return err
		}
	}
	logrus.WithFields(logrus.Fields{
		"Component":    "JobManager",
		"RequestID":    req.ID,
		"User":         req.UserID,
		"Organization": req.OrganizationID,
		"JobType":      req.JobType,
		"UserKey":      req.UserKey,
	}).Infof("canceled request")
	_, _ = jm.auditRecordRepository.Save(types.NewAuditRecordFromJobRequest(req, types.JobRequestCancelled, qc))
	return nil
}

// RequeueOrphanJobRequests queries jobs with EXECUTING/STARTED status and puts them back to PENDING/PAUSED
func (jm *JobManager) RequeueOrphanJobRequests(
	staleInterval time.Duration) (total int64, err error) {
	return jm.jobRequestRepository.RequeueOrphanRequests(staleInterval)
}

// DeleteJobRequest deletes job request
func (jm *JobManager) DeleteJobRequest(
	qc *common.QueryContext,
	id uint64) error {
	return jm.jobRequestRepository.Delete(qc, id)
}

// NextSchedulableJobRequestsByType queries basic job id/state for pending/ready state from parameter
func (jm *JobManager) NextSchedulableJobRequestsByType(
	jobTypes []string,
	states []common.RequestState,
	limit int) ([]*types.JobRequestInfo, error) {
	return jm.jobRequestRepository.NextSchedulableJobsByTypes(
		jobTypes,
		states,
		limit)
}

// FindMissingCronScheduledJobsByType queries scheduled jobs and determines missing jobs that should have been scheduled
// Note: the length of job-types and user keys must match and correspond to the index
func (jm *JobManager) FindMissingCronScheduledJobsByType(
	jobTypes []types.JobTypeCronTrigger,
) ([]types.JobTypeCronTrigger, error) {
	activeJobInfos, err := jm.jobRequestRepository.FindActiveCronScheduledJobsByJobType(
		jobTypes)
	if err != nil {
		return nil, err
	}
	res := make([]types.JobTypeCronTrigger, 0)
	for _, jobType := range jobTypes {
		matched := false
		for _, active := range activeJobInfos {
			if jobType.JobType == active.JobType &&
				((jobType.OrganizationID != "" && active.OrganizationID != "" && jobType.OrganizationID == active.OrganizationID) ||
					jobType.UserID == active.UserID) {
				matched = true
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"JobType":       jobType.JobType,
						"Org":           jobType.OrganizationID,
						"UserID":        jobType.UserID,
						"UserKey":       jobType.UserKey,
						"ActiveOrg":     active.OrganizationID,
						"ActiveUserID":  active.UserID,
						"ActiveUserKey": active.GetUserJobTypeKey(),
					}).Debugf("matched missing type")
				}
				break
			}
		}
		if !matched {
			res = append(res, jobType)
		}
	}

	// update cron pending jobs
	for _, info := range activeJobInfos {
		jm.jobStatsRegistry.Pending(info, false)
	}
	return res, nil
}

// IncrementScheduleAttemptsForJobRequest bump schedule time and decrement priority for jobs that are not ready
func (jm *JobManager) IncrementScheduleAttemptsForJobRequest(
	req *types.JobRequestInfo,
	scheduleSecs time.Duration,
	decrPriority int,
	errorMessage string) (err error) {
	jm.jobStatsRegistry.SetAntsAvailable(req, false, errorMessage)
	if err = jm.jobRequestRepository.IncrementScheduleAttempts(
		req.ID,
		scheduleSecs,
		decrPriority,
		errorMessage); err != nil {
		logrus.WithFields(logrus.Fields{
			"JobRequestID": req.ID,
			"JobType":      req.JobType,
			"ScheduleSecs": scheduleSecs,
			"DecrPriority": decrPriority,
			"ErrorMessage": errorMessage,
			"Error":        err,
		}).Error("failed to increment schedule attempt for job request")
	} else {
		jm.metricsRegistry.Incr("job_rescheduled_total", map[string]string{"JobType": req.JobType})
		jm.jobStatsRegistry.Pending(req, false)
	}
	return
}

// UpdateJobRequestState sets state of job-request
func (jm *JobManager) UpdateJobRequestState(
	qc *common.QueryContext,
	req types.IJobRequest,
	oldState common.RequestState,
	newState common.RequestState,
	errorMessage string,
	errorCode string,
	scheduleDelay time.Duration,
	retried int,
	reverted bool) (err error) {
	err = jm.jobRequestRepository.UpdateJobState(
		req.GetID(),
		oldState,
		newState,
		errorMessage,
		errorCode,
		scheduleDelay,
		retried)
	if newState == common.PENDING {
		jm.jobStatsRegistry.Pending(req, reverted)
	} else if newState.IsTerminal() && req.GetCronTriggered() {
		var jobDefinition *types.JobDefinition
		if jobDefinition, err = jm.GetJobDefinitionByType(qc, req.GetJobType(), req.GetJobVersion()); err != nil {
			return err
		}
		if cronScheduledAt, _ := jobDefinition.GetCronScheduleTimeAndUserKey(); cronScheduledAt != nil {
			if err = jm.scheduleCronRequest(jobDefinition, req); err != nil {
				return err
			}
		}
	}
	return
}

// GetJobRequestCounts calculates counts for all job-types/statuses/error-codes within given range
func (jm *JobManager) GetJobRequestCounts(
	qc *common.QueryContext,
	start time.Time,
	end time.Time) ([]*types.JobCounts, error) {
	return jm.jobRequestRepository.JobCounts(qc, start, end)
}

// GetExecutionCount return count of executing jobs
func (jm *JobManager) GetExecutionCount(key types.UserJobTypeKey) int32 {
	return jm.jobStatsRegistry.GetExecutionCount(key)
}

// UserOrgExecuting return count of executing jobs by user and org
func (jm *JobManager) UserOrgExecuting(req types.IJobRequestSummary) (int, int) {
	return jm.jobStatsRegistry.UserOrgExecuting(req)
}

// SetJobRequestReadyToExecute marks job as ready to execute so that job can be picked up by job launcher
func (jm *JobManager) SetJobRequestReadyToExecute(
	id uint64,
	jobExecutionID string,
	lastJobExecutionID string) error {
	return jm.jobRequestRepository.SetReadyToExecute(id, jobExecutionID, lastJobExecutionID)
}

// TriggerJobRequest - triggers job
func (jm *JobManager) TriggerJobRequest(
	qc *common.QueryContext,
	id uint64) (err error) {
	if err = jm.jobRequestRepository.Trigger(qc, id); err == nil {
		if req, dbErr := jm.jobRequestRepository.Get(qc, id); dbErr == nil {
			logrus.WithFields(logrus.Fields{
				"Component":    "JobManager",
				"RequestID":    req.ID,
				"User":         req.UserID,
				"Organization": req.OrganizationID,
				"JobType":      req.JobType,
				"UserKey":      req.UserKey,
			}).Infof("triggered job request")
			jm.metricsRegistry.Incr("job_trigger_total", map[string]string{"JobType": req.JobType})
			_ = jm.fireJobRequestChange(req)
			_, _ = jm.auditRecordRepository.Save(types.NewAuditRecordFromJobRequest(req, types.JobRequestTriggered, qc))
		}
	}
	return
}

// RestartJobRequest - restarts job
func (jm *JobManager) RestartJobRequest(
	qc *common.QueryContext,
	id uint64) (err error) {
	if err = jm.jobRequestRepository.Restart(qc, id); err == nil {
		if req, dbErr := jm.jobRequestRepository.Get(qc, id); dbErr == nil {
			jm.metricsRegistry.Incr("job_restarted_total", map[string]string{"JobType": req.JobType})
			_ = jm.fireJobRequestChange(req)
			_, _ = jm.auditRecordRepository.Save(types.NewAuditRecordFromJobRequest(req, types.JobRequestRestarted, qc))
			logrus.WithFields(logrus.Fields{
				"Component":    "JobManager",
				"RequestID":    req.ID,
				"User":         req.UserID,
				"Organization": req.OrganizationID,
				"JobType":      req.JobType,
				"UserKey":      req.UserKey,
			}).Infof("restarted job request")
		}
	}
	return
}

// GetResourceUsage usage
func (jm *JobManager) GetResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	return jm.jobExecutionRepository.GetResourceUsage(qc, ranges)
}

// UpdateJobRequestTimestamp updates timestamp so that job scheduler doesn't consider it as orphan
func (jm *JobManager) UpdateJobRequestTimestamp(requestID uint64) (err error) {
	return jm.jobRequestRepository.UpdateRunningTimestamp(requestID)
}

// GetDotConfigForJobRequest - creates graphviz dot file
func (jm *JobManager) GetDotConfigForJobRequest(
	qc *common.QueryContext,
	id uint64) (string, error) {
	request, err := jm.jobRequestRepository.Get(qc, id)
	if err != nil {
		return "", err
	}
	definition, err := jm.GetJobDefinitionByType(qc, request.JobType, request.JobVersion)
	if err != nil {
		return "", err
	}

	if request.JobExecutionID != "" {
		request.Execution, _ = jm.jobExecutionRepository.Get(request.JobExecutionID)
	}
	generator, err := dot.New(definition, request.Execution)
	if err != nil {
		return "", err
	}
	return generator.GenerateDot()
}

// GetDotImageForJobRequest - creates graphviz png image file
func (jm *JobManager) GetDotImageForJobRequest(
	qc *common.QueryContext,
	id uint64) ([]byte, error) {
	request, err := jm.jobRequestRepository.Get(qc, id)
	if err != nil {
		return nil, err
	}
	definition, err := jm.GetJobDefinitionByType(qc, request.JobType, request.JobVersion)
	if err != nil {
		return nil, err
	}

	if request.JobExecutionID != "" {
		request.Execution, _ = jm.jobExecutionRepository.Get(request.JobExecutionID)
	}
	generator, err := dot.New(definition, request.Execution)
	if err != nil {
		return nil, err
	}
	return generator.GenerateDotImage()
}

// BuildGithubPostWebhookHandler returns GithubPostWebhookHandler callback
func (jm *JobManager) BuildGithubPostWebhookHandler() security.GithubPostWebhookHandler {
	return func(
		qc *common.QueryContext,
		jobType string,
		jobVersion string,
		params map[string]string,
		hash256 string,
		body []byte) error {
		jobDef, err := jm.GetJobDefinitionByType(qc, jobType, jobVersion)
		if err != nil {
			return err
		}
		jobReq, err := types.NewJobRequestFromDefinition(jobDef)
		if err != nil {
			return err
		}
		webhookSecret := jobDef.GetConfigString("GithubWebhookSecret")
		if webhookSecret == "" {
			if qc.HasOrganization() {
				webhookSecret = qc.User.Organization.GetConfigString("GithubWebhookSecret")
			}
		}
		if webhookSecret == "" {
			return fmt.Errorf("`GithubWebhookSecret` config is not set for job `%s`", jobType)
		}
		if err = utils.VerifySignature(webhookSecret, hash256, body); err != nil {
			return err
		}
		for k, v := range params {
			_, _ = jobReq.AddParam(k, v)
		}
		saved, err := jm.SaveJobRequest(qc, jobReq)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":  "GithubAuth",
				"JobType":    jobType,
				"JobRequest": saved,
				"SHA256":     hash256,
				"QC":         qc,
				"Error":      err,
			}).Errorf("failed to submit job request from github webhook")
			return err
		}

		logrus.WithFields(logrus.Fields{
			"Component":  "GithubAuth",
			"SHA256":     hash256,
			"JobType":    jobType,
			"JobRequest": saved,
			"QC":         qc,
		}).Infof("submitted job as a result of github webhook")
		return nil
	}
}

/////////////////////////////////////////// JOB EXECUTION METHODS ////////////////////////////////////////////

// GetJobExecution method finds JobExecution by id
func (jm *JobManager) GetJobExecution(
	id string) (*types.JobExecution, error) {
	return jm.jobExecutionRepository.Get(id)
}

// CreateJobExecution saves job-execution
func (jm *JobManager) CreateJobExecution(
	jobExec *types.JobExecution) (saved *types.JobExecution, err error) {
	saved, err = jm.jobExecutionRepository.Save(jobExec)
	if err == nil {
		jm.jobStatsRegistry.Started(saved)
	}
	return
}

// ResetJobExecutionStateToReady resets state to ready
func (jm *JobManager) ResetJobExecutionStateToReady(exec *types.JobExecution) error {
	jm.jobStatsRegistry.Started(exec)
	return jm.jobExecutionRepository.ResetStateToReady(exec.ID)
}

// SetJobRequestAndExecutingStatusToExecuting updates state of job-execution and job-request to EXECUTING
func (jm *JobManager) SetJobRequestAndExecutingStatusToExecuting(executionID string) error {
	return jm.jobExecutionRepository.UpdateJobRequestAndExecutionState(
		executionID,
		common.READY,
		common.EXECUTING)
}

// NotifyJobMessage notifies job results
func (jm *JobManager) NotifyJobMessage(
	qc *common.QueryContext,
	user *common.User,
	job *types.JobDefinition,
	request types.IJobRequest,
	jobExec *types.JobExecution,
	lastRequestState common.RequestState,
) error {
	return jm.jobsNotifier.NotifyJob(
		qc,
		user,
		job,
		request,
		jobExec,
		lastRequestState)
}

// FinalizeJobRequestAndExecutionState updates final state of job-execution and job-request
// Also, it triggers next request for cron based job definitions
func (jm *JobManager) FinalizeJobRequestAndExecutionState(
	qc *common.QueryContext,
	user *common.User,
	job *types.JobDefinition,
	req types.IJobRequest,
	jobExec *types.JobExecution,
	oldState common.RequestState,
	scheduleDelay time.Duration,
	retried int,
) (err error) {
	lastRequestState := jm.jobStatsRegistry.LastJobStatus(req)
	err = jm.jobExecutionRepository.FinalizeJobRequestAndExecutionState(
		jobExec.ID,
		oldState,
		jobExec.JobState,
		jobExec.ErrorMessage,
		jobExec.ErrorCode,
		jobExec.ExecutionCostSecs(),
		scheduleDelay,
		retried)
	if req.GetJobState().Failed() {
		jm.jobStatsRegistry.Failed(jobExec, jobExec.ElapsedMillis())
	} else if req.GetJobState().Paused() {
		jm.jobStatsRegistry.Paused(jobExec, jobExec.ElapsedMillis())
	} else {
		jm.jobStatsRegistry.Succeeded(jobExec, jobExec.ElapsedMillis())
	}
	forkedJob := false
	for _, p := range req.GetParams() {
		if p.Name == common.ForkedJob && p.Value == "true" {
			forkedJob = true
			break
		}
	}
	if !forkedJob { // don't notify on child jobs
		// Send notification asynchronously
		// TODO add background process for message notifications with database persistence
		go func() {
			if notifyErr := jm.NotifyJobMessage(
				qc,
				user,
				job,
				req,
				jobExec,
				lastRequestState); notifyErr != nil {
				logrus.WithFields(logrus.Fields{
					"Component":        "JobManager",
					"User":             user,
					"Request":          req,
					"LastRequestState": lastRequestState,
					"Error":            notifyErr,
				}).Warnf("failed to send job notification")
			}
		}()
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":        "JobManager",
			"User":             user,
			"Request":          req,
			"LastRequestState": lastRequestState,
		}).Infof("skipping job notification for child job")
	}

	// trigger cron job if needed
	if err == nil && jobExec.JobState.IsTerminal() && req.GetCronTriggered() {
		var jobDefinition *types.JobDefinition
		if jobDefinition, err = jm.GetJobDefinitionByType(qc, jobExec.JobType, jobExec.JobVersion); err != nil {
			return err
		}
		if cronScheduledAt, _ := jobDefinition.GetCronScheduleTimeAndUserKey(); cronScheduledAt != nil {
			if err = jm.scheduleCronRequest(jobDefinition, req); err != nil {
				return err
			}
		}
	}
	return
}

// UpdateJobExecutionContext updates context of job-execution
func (jm *JobManager) UpdateJobExecutionContext(
	id string, contexts []*types.JobExecutionContext) error {
	return jm.jobExecutionRepository.UpdateJobContext(id, contexts)
}

// DeleteJobExecution soft deletes job-execution
func (jm *JobManager) DeleteJobExecution(
	id string) error {
	return jm.jobExecutionRepository.Delete(id)
}

// SaveExecutionTask creates or saves task execution
func (jm *JobManager) SaveExecutionTask(
	task *types.TaskExecution) (*types.TaskExecution, error) {
	return jm.jobExecutionRepository.SaveTask(task)
}

// UpdateTaskExecutionState sets state of task-execution
func (jm *JobManager) UpdateTaskExecutionState(
	id string,
	oldState common.RequestState,
	newState common.RequestState) error {
	return jm.jobExecutionRepository.UpdateTaskState(
		id,
		oldState,
		newState)
}

// DeleteExecutionTask deletes task of job-execution
func (jm *JobManager) DeleteExecutionTask(
	id string) error {
	return jm.jobExecutionRepository.DeleteTask(id)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (jm *JobManager) startRecentlyCompletedJobIdsTicker(ctx context.Context) error {
	jm.jobIdsTicker = time.NewTicker(jm.serverCfg.Common.DeadJobIDsEventsInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				jm.jobIdsTicker.Stop()
				return
			case <-jm.jobIdsTicker.C:
				if err := jm.publishDeadJobIds(ctx); err != nil {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(logrus.Fields{
							"Component": "JobManager",
							"Error":     err,
						}).Debug("failed to publish job-manager")
					}
				}
			}
		}
	}()
	return nil
}

// Fire event to job-definition lifecycle event
func (jm *JobManager) fireJobDefinitionChange(
	username string,
	id string,
	jobType string,
	eventType events.JobDefinitionStateChange) (err error) {
	event := events.NewJobDefinitionLifecycleEvent(
		jm.serverCfg.Common.ID,
		username,
		id,
		jobType,
		eventType)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal job-definition event due to %w", err)
	}
	if _, err = jm.queueClient.Publish(
		context.Background(),
		jm.serverCfg.Common.GetJobDefinitionLifecycleTopic(),
		payload,
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
			"JobDefinitionID", id,
			"JobType", jobType,
			"UserID", username,
		),
	); err != nil {
		return fmt.Errorf("failed to send job-definition event due to %w", err)
	}
	return nil
}

// Fire event to job-request lifecycle event
func (jm *JobManager) fireJobRequestChange(req *types.JobRequest) (err error) {
	event := events.NewJobRequestLifecycleEvent(
		jm.serverCfg.Common.ID,
		req.UserID,
		req.ID,
		req.JobType,
		req.JobState,
		make(map[string]interface{}),
	)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "JobManager",
			"RequestID":    req.ID,
			"User":         req.UserID,
			"Organization": req.OrganizationID,
			"JobType":      req.JobType,
			"JobState":     req.JobState,
			"UserKey":      req.UserKey,
			"Error":        err,
		}).Warnf("failed to marshal publish event")
		return fmt.Errorf("failed to marshal job-request event due to %w", err)
	}
	if _, err = jm.queueClient.Publish(context.Background(),
		jm.serverCfg.Common.GetJobRequestLifecycleTopic(),
		payload,
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
			"RequestID", fmt.Sprintf("%d", req.GetID()),
			"UserID", req.UserID,
		),
	); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "JobManager",
			"RequestID":    req.ID,
			"User":         req.UserID,
			"Organization": req.OrganizationID,
			"JobType":      req.JobType,
			"JobState":     req.JobState,
			"UserKey":      req.UserKey,
			"Error":        err,
		}).Warnf("failed to publish event")
		return fmt.Errorf("failed to send job-request event due to %w", err)
	}
	return nil
}

// Fire event to cancel job
func (jm *JobManager) cancelJob(
	qc *common.QueryContext,
	req *types.JobRequest) (err error) {
	jobExecutionLifecycleEvent := events.NewJobExecutionLifecycleEvent(
		jm.serverCfg.Common.ID,
		req.UserID,
		req.ID,
		req.JobType,
		req.JobExecutionID,
		common.CANCELLED,
		req.JobPriority,
		make(map[string]interface{}),
	)
	var payload []byte
	if payload, err = jobExecutionLifecycleEvent.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal job-execution jobExecutionLifecycleEvent due to %w", err)
	}
	// TODO add better reliability for this pub/sub
	if _, err = jm.queueClient.Publish(context.Background(),
		jm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		payload,
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
			"RequestID", fmt.Sprintf("%d", req.GetID()),
			"UserID", req.UserID,
		),
	); err != nil {
		return fmt.Errorf("failed to send job-execution jobExecutionLifecycleEvent due to %w", err)
	}

	go func() {
		time.Sleep(1 * time.Second)
		jm.overrideCancelRequest(qc, req, jobExecutionLifecycleEvent)
	}()
	logrus.WithFields(logrus.Fields{
		"Component":                  "JobManager",
		"ID":                         jobExecutionLifecycleEvent.ID,
		"Topic":                      jm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
		"EventState":                 jobExecutionLifecycleEvent.JobState,
		"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
	}).Infof("firing cancel event for jobExecutionLifecycleEvent")
	return nil
}

func (jm *JobManager) overrideCancelRequest(
	qc *common.QueryContext,
	req *types.JobRequest,
	jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) {
	if verify, err := jm.GetJobRequest(qc, req.ID); err == nil && !verify.JobState.IsTerminal() {
		if err = jm.jobRequestRepository.Cancel(qc, req.ID); err == nil {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobManager",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Topic":                      jm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
				"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
				"EventState":                 jobExecutionLifecycleEvent.JobState,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
			}).Info("cancelled request after override")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobManager",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Topic":                      jm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
				"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
				"EventState":                 jobExecutionLifecycleEvent.JobState,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
				"Error":                      err,
			}).Errorf("failed to cancel request after override")
		}
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":                  "JobManager",
			"ID":                         jobExecutionLifecycleEvent.ID,
			"Topic":                      jm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
			"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
			"EventState":                 jobExecutionLifecycleEvent.JobState,
			"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
			"Error":                      err,
		}).Errorf("failed to find request after cancel verification")
	}
}

// CheckSubscriptionQuota checks quota
func (jm *JobManager) CheckSubscriptionQuota(
	qc *common.QueryContext,
	user *common.User,
) (cpuUsage types.ResourceUsage, diskUsage types.ResourceUsage, err error) {
	if jm.serverCfg.SubscriptionQuotaEnabled && jm.serverCfg.Common.Auth.Enabled {
		cpuUsage, diskUsage, err = jm.doCheckSubscriptionQuota(qc, user)
		dirty := false
		if err != nil {
			if user != nil && user.StickyMessage == "" {
				switch quotaErr := err.(type) {
				case *common.QuotaExceededError:
					user.StickyMessage = quotaErr.Internal.Error()
				default:
					user.StickyMessage = err.Error()
				}
				dirty = true
			}
			if user.HasOrganization() && user.Organization.StickyMessage == "" {
				switch quotaErr := err.(type) {
				case *common.QuotaExceededError:
					user.Organization.StickyMessage = quotaErr.Internal.Error()
				default:
					user.Organization.StickyMessage = err.Error()
				}
				dirty = true
			}
		} else {
			if user != nil && strings.Contains(user.StickyMessage, "quota-error") {
				user.StickyMessage = ""
				dirty = true
			}
			if user.HasOrganization() && strings.Contains(
				user.Organization.StickyMessage,
				"quota-error") {
				user.Organization.StickyMessage = ""
				dirty = true
			}
		}
		if dirty {
			// ignore if update failed
			_ = jm.userManager.UpdateStickyMessage(qc, user)
		}
	}
	return
}

// CheckSubscriptionQuota checks quota
func (jm *JobManager) doCheckSubscriptionQuota(
	qc *common.QueryContext,
	user *common.User,
) (cpuUsage types.ResourceUsage, diskUsage types.ResourceUsage, err error) {
	if user != nil && user.IsAdmin() {
		return
	}
	if user == nil || user.Subscription == nil {
		return cpuUsage, diskUsage, fmt.Errorf("quota-error: user subscription not found")
	}
	if user.Subscription.Expired() {
		return cpuUsage, diskUsage, fmt.Errorf("quota-error: user subscription is expired")
	}
	ranges := []types.DateRange{{StartDate: user.Subscription.StartedAt, EndDate: user.Subscription.EndedAt}}
	if cpuUsages, err := jm.GetResourceUsage(
		qc, ranges); err == nil {
		if cpuUsages[0].Value >= user.Subscription.CPUQuota {
			return cpuUsage, diskUsage, common.NewQuotaExceededError(
				fmt.Errorf("quota-error: exceeded cpu quota %d secs, check your subscription", user.Subscription.CPUQuota))
		}
		cpuUsage = cpuUsages[0]
	} else {
		return cpuUsage, diskUsage, err
	}
	if diskUsages, err := jm.artifactManager.GetResourceUsage(
		qc, ranges); err == nil {
		if diskUsages[0].MValue() >= user.Subscription.DiskQuota {
			return cpuUsage, diskUsage, common.NewQuotaExceededError(
				fmt.Errorf("quota-error: exceeded disk quota %d MiB, check your subscription", user.Subscription.DiskQuota))
		}
		diskUsage = diskUsages[0]
	} else {
		return cpuUsage, diskUsage, err
	}
	return
}

func initializeStatsRegistry(
	jobRequestRepository repository.JobRequestRepository,
	jobStatsRegistry *stats.JobStatsRegistry) {
	pending := 0
	succeeded := 0
	failed := 0
	cancelled := 0
	if times, err := jobRequestRepository.GetJobTimes(1000); err == nil {
		for _, t := range times {
			if t.Pending() {
				jobStatsRegistry.Pending(t.ToInfo(), false)
				pending++
			} else if t.Completed() {
				jobStatsRegistry.Started(t)
				jobStatsRegistry.Succeeded(t, t.Elapsed())
				succeeded++
			} else if t.Failed() {
				jobStatsRegistry.Started(t)
				jobStatsRegistry.Failed(t, t.Elapsed())
				failed++
			} else if t.Cancelled() {
				jobStatsRegistry.Started(t)
				jobStatsRegistry.Cancelled(t)
				cancelled++
			}
		}
		logrus.WithFields(logrus.Fields{
			"Component": "JobManager",
			"Pending":   pending,
			"Succeeded": succeeded,
			"Failed":    failed,
			"Cancelled": cancelled,
		}).Infof("initialized jobs registry")
	}
}
