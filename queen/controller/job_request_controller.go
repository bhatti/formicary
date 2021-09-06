package controller

import (
	"encoding/json"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/stats"
	"strconv"
	"time"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/types"
)

// JobRequestController structure
type JobRequestController struct {
	jobManager *manager.JobManager
	webserver  web.Server
}

// NewJobRequestController instantiates controller for managing job-requests
func NewJobRequestController(
	jobManager *manager.JobManager,
	webserver web.Server) *JobRequestController {
	jobReqCtrl := &JobRequestController{
		jobManager: jobManager,
		webserver:  webserver,
	}

	webserver.GET("/api/jobs/requests", jobReqCtrl.queryJobRequests, acl.New(acl.JobRequest, acl.Query)).Name = "query_job_requests"
	webserver.GET("/api/jobs/requests/:id", jobReqCtrl.getJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "get_job_request"
	webserver.GET("/api/jobs/requests/:id/dot", jobReqCtrl.dotJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "dot_job_request"
	webserver.GET("/api/jobs/requests/:id/dot.png", jobReqCtrl.dotImageJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "dot_png_job_request"
	webserver.POST("/api/jobs/requests", jobReqCtrl.submitJobRequest, acl.New(acl.JobRequest, acl.Submit)).Name = "create_job_request"
	webserver.POST("/api/jobs/requests/:id/cancel", jobReqCtrl.cancelJobRequest, acl.New(acl.JobRequest, acl.Cancel)).Name = "cancel_job_request"
	webserver.POST("/api/jobs/requests/:id/restart", jobReqCtrl.restartJobRequest, acl.New(acl.JobRequest, acl.Restart)).Name = "restart_job_request"
	webserver.POST("/api/jobs/requests/:id/trigger", jobReqCtrl.triggerJobRequest, acl.New(acl.JobRequest, acl.Trigger)).Name = "trigger_job_request"
	webserver.GET("/api/jobs/requests/:id/wait_time", jobReqCtrl.getWaitTimeJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "get_wait_time_job_requests"
	webserver.GET("/api/jobs/requests/stats", jobReqCtrl.statsJobRequests, acl.New(acl.JobRequest, acl.Metrics)).Name = "stats_job_requests"
	webserver.GET("/api/jobs/requests/dead_ids", jobReqCtrl.getDeadIDs, acl.New(acl.JobRequest, acl.Query)).Name = "get_dead_ids"
	return jobReqCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/jobs/requests job-requests queryJobRequests
// Queries job requests by criteria such as type, platform, etc.
// responses:
//   200: jobRequestQueryResponse
func (jobReqCtrl *JobRequestController) queryJobRequests(c web.WebContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := jobReqCtrl.jobManager.QueryJobRequests(
		qc,
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// swagger:route GET /api/jobs/requests/{id} job-requests getJobRequest
// Finds the job-request by id.
// responses:
//   200: jobRequest
func (jobReqCtrl *JobRequestController) getJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	request, err := jobReqCtrl.jobManager.GetJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, request)
}

// swagger:route POST /api/jobs/requests job-requests submitJobRequest
// Submits a job-request for processing, which is saved in the database and is then scheduled for execution.
// responses:
//   200: jobRequest
func (jobReqCtrl *JobRequestController) submitJobRequest(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	request, err := types.NewJobRequestFromDefinition(types.NewJobDefinition(""))
	if err != nil {
		return err
	}
	err = json.NewDecoder(c.Request().Body).Decode(request)
	if err != nil {
		return err
	}

	request.UserID = qc.UserID
	request.OrganizationID = qc.OrganizationID

	jobDefinition, err := jobReqCtrl.jobManager.GetJobDefinitionByType(qc, request.JobType, request.JobVersion)
	if err != nil {
		return err
	}
	request.UpdateUserKeyFromScheduleIfCronJob(jobDefinition)

	// delete duplicate entry if exists
	_ = jobReqCtrl.jobManager.DeactivateOldCronRequest(qc, request)

	saved, err := jobReqCtrl.jobManager.SaveJobRequest(qc, request)
	if err != nil {
		return err
	}
	_, _ = jobReqCtrl.jobManager.SaveAudit(types.NewAuditRecordFromJobRequest(saved, types.JobRequestCreated, qc))

	return c.JSON(http.StatusCreated, saved)
}

// swagger:route POST /api/jobs/requests/{id}/cancel job-requests cancelJobRequest
// Cancels a job-request that is pending for execution or already executing.
// responses:
//   200: emptyResponse
func (jobReqCtrl *JobRequestController) cancelJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	if err := jobReqCtrl.jobManager.CancelJobRequest(qc, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// swagger:route POST /api/jobs/requests/{id}/trigger job-requests triggerJobRequest
// Triggers a scheduled job
// responses:
//   200: emptyResponse
func (jobReqCtrl *JobRequestController) triggerJobRequest(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	err := jobReqCtrl.jobManager.TriggerJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// swagger:route POST /api/jobs/requests/{id}/restart job-requests restartJobRequest
// Restarts a previously failed job so that it can re-executed, the restart may perform soft-restart where only
// failed tasks are executed or hard-restart where all tasks are executed.
// responses:
//   200: emptyResponse
func (jobReqCtrl *JobRequestController) restartJobRequest(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	err := jobReqCtrl.jobManager.RestartJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// swagger:route GET /api/jobs/requests/{id}/dot job-requests dotJobRequest
// Returns Graphviz DOT request for the graph of tasks defined in the job request.
// responses:
//   200: stringResponse
func (jobReqCtrl *JobRequestController) dotJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	d, err := jobReqCtrl.jobManager.GetDotConfigForJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, d)
}

// swagger:route GET /api/jobs/requests/{id}/dot.png job-requests dotImageJobRequest
// Returns Graphviz DOT image for the graph of tasks defined in the job.
// responses:
//   200: byteResponse
func (jobReqCtrl *JobRequestController) dotImageJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	d, err := jobReqCtrl.jobManager.GetDotImageForJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "image/png", d)
}

// swagger:route GET /api/jobs/requests/{id}/wait_time job-requests getWaitTimeJobRequest
// Returns wait time for the job-request.
// responses:
//   200: jobRequestWaitTimes
func (jobReqCtrl *JobRequestController) getWaitTimeJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	estimate, err := jobReqCtrl.jobManager.GetWaitEstimate(qc, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, estimate)
}

// swagger:route GET /api/jobs/requests/stats job-requests statsJobRequests
// Returns statistics for the job-request such as success rate, latency, etc.
// `This requires admin access`
// responses:
//   200: jobRequestStats
func (jobReqCtrl *JobRequestController) statsJobRequests(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	start := time.Unix(0, 0)
	end := time.Now()
	if d, err := time.Parse("2006-01-02T15:04:05-0700", c.QueryParam("from")); err == nil {
		start = d
	} else if d, err := time.Parse("2006-01-02", c.QueryParam("from")); err == nil {
		start = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	}
	if d, err := time.Parse("2006-01-02T15:04:05-0700", c.QueryParam("to")); err == nil {
		end = d
	} else if d, err := time.Parse("2006-01-02", c.QueryParam("to")); err == nil {
		end = time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, d.Location())
	}
	recs, err := jobReqCtrl.jobManager.GetJobRequestCounts(qc, start, end)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, recs)
}

// swagger:route GET /api/jobs/requests/dead_ids job-requests getDeadIDs
// Returns job-request ids for recently completed jobs.
// responses:
//   200: jobRequestIDs
func (jobReqCtrl *JobRequestController) getDeadIDs(c web.WebContext) error {
	limit, _ := strconv.Atoi(c.Param("limit"))
	if limit == 0 {
		limit = 200
	} else if limit > 10000 {
		limit = 10000
	}
	ids, err := jobReqCtrl.jobManager.RecentDeadIDs(limit)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, ids)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryJobRequests
// The params for querying jobRequests.
type jobRequestQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// JobType defines a unique type of job
	JobType string `yaml:"job_type" json:"job_type"`
	// Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering
	Platform string `json:"platform"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState common.RequestState `json:"job_state"`
	// JobGroup defines a property for grouping related job
	JobGroup string `json:"job_group"`
}

// Paginated results of jobRequests matching query
// swagger:response jobRequestQueryResponse
type jobRequestQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []types.JobRequest
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters jobRequestIDParams getJobRequest cancelJobRequest restartJobRequest triggerJobRequest dotJobRequest dotImageJobRequest
// The parameters for finding job-request by id
type jobRequestIDParams struct {
	// in:path
	ID string `json:"id"`
}

// swagger:parameters submitJobRequest
// The request body includes job-request for persistence.
type jobRequestParams struct {
	// in:body
	Body types.JobRequest
}

// JobRequest defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution.
// swagger:response jobRequest
type jobRequestBody struct {
	// in:body
	Body types.JobRequest
}

// The job-request wait times based on average of previously executed jobs and pending jobs in the queue.
// swagger:response jobRequestWaitTimes
type jobRequestWaitTimesBody struct {
	// in:body
	Body stats.JobWaitEstimate
}

// The job-request statistics about success-rate, latency, etc.
// swagger:response jobRequestStats
type jobRequestStatsBody struct {
	// in:body
	Body []types.JobCounts
}

// The job-request ids
// swagger:response jobRequestIDs
type jobRequestIDsBody struct {
	// in:body
	Body []uint64
}
