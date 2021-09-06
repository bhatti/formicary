package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/internal/acl"

	"plexobject.com/formicary/queen/manager"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/types"
)

// JobRequestAdminController structure
type JobRequestAdminController struct {
	jobManager *manager.JobManager
	webserver  web.Server
}

// NewJobRequestAdminController admin dashboard for managing job-requests
func NewJobRequestAdminController(
	jobManager *manager.JobManager,
	webserver web.Server) *JobRequestAdminController {
	jraCtr := &JobRequestAdminController{
		jobManager: jobManager,
		webserver:  webserver,
	}
	webserver.GET("/dashboard/jobs/requests", jraCtr.queryJobRequests, acl.New(acl.JobResource, acl.Query)).Name = "query_admin_job_requests"
	webserver.GET("/dashboard/jobs/requests/new", jraCtr.newJobRequest, acl.New(acl.JobRequest, acl.Submit)).Name = "new_admin_job_requests"
	webserver.POST("/dashboard/jobs/requests", jraCtr.createJobRequest, acl.New(acl.JobRequest, acl.Submit)).Name = "create_admin_job_requests"
	webserver.POST("/dashboard/jobs/requests/:id/cancel", jraCtr.cancelJobRequest, acl.New(acl.JobRequest, acl.Cancel)).Name = "cancel_admin_job_requests"
	webserver.POST("/dashboard/jobs/requests/:id/restart", jraCtr.restartJobRequest, acl.New(acl.JobRequest, acl.Restart)).Name = "restart_admin_job_requests"
	webserver.POST("/dashboard/jobs/requests/:id/trigger", jraCtr.triggerJobRequest, acl.New(acl.JobRequest, acl.Trigger)).Name = "trigger_admin_job_requests"
	webserver.GET("/dashboard/jobs/requests/:id", jraCtr.getJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "get_admin_job_requests"
	webserver.GET("/dashboard/jobs/requests/:id/wait_time", jraCtr.getWaitTimeJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "get_wait_time_admin_job_requests"
	webserver.GET("/dashboard/jobs/requests/:id/dot", jraCtr.dotJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "dot_job_request"
	webserver.GET("/dashboard/jobs/requests/:id/dot.png", jraCtr.dotImageJobRequest, acl.New(acl.JobRequest, acl.View)).Name = "dot_png_job_request"
	webserver.GET("/dashboard/jobs/requests/stats", jraCtr.statsJobRequests, acl.New(acl.JobRequest, acl.Metrics)).Name = "stats_admin_job_requests"

	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// getWaitTimeJobRequest - wait time info of job-request
func (jraCtr *JobRequestAdminController) getWaitTimeJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	estimate, err := jraCtr.jobManager.GetWaitEstimate(qc, id)
	if err != nil {
		return fmt.Errorf("failed to estimate wait time for %d due to %v", id, err)
	}
	res := map[string]interface{}{"Estimate": estimate}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/req/estimate", res)
}

// statsJobRequests - stats of job-request -- admin only
func (jraCtr *JobRequestAdminController) statsJobRequests(c web.WebContext) error {
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
	recs, err := jraCtr.jobManager.GetJobRequestCounts(qc, start, end)
	if err != nil {
		return err
	}

	res := map[string]interface{}{"Stats": recs,
		"FromDate": start.Format("2006-01-02"),
		"ToDate":   end.Format("2006-01-02"),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/req/stats", res)
}

// queryJobRequests - queries job-request
func (jraCtr *JobRequestAdminController) queryJobRequests(c web.WebContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := jraCtr.jobManager.QueryJobRequests(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	//class="table-success table-danger table-warning table-info table-light table-dark table-active

	baseURL := fmt.Sprintf("/dashboard/jobs/requests?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	title := ""
	if c.QueryParam("job_state") == "WAITING" {
		title = "Pending Jobs"
	} else if c.QueryParam("job_state") == "RUNNING" {
		title = "Running Jobs"
	} else if c.QueryParam("job_state") == "DONE" {
		title = "Jobs History"
	}

	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"Title":      title,
		"JobTypes":   jraCtr.getJobTypes(c),
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	res["IsTerminal"] = false
	res["Pending"] = false
	for _, rec := range recs {
		if rec.IsTerminal() {
			res["IsTerminal"] = true
		} else if rec.Pending() {
			res["Pending"] = true
		}
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/req/index", res)
}

// cancelJobRequests - cancel job-request
func (jraCtr *JobRequestAdminController) cancelJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	if err := jraCtr.jobManager.CancelJobRequest(qc, id); err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/requests?job_state=DONE")
}

// triggerJobRequest - triggers a scheduled job-request
func (jraCtr *JobRequestAdminController) triggerJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	err := jraCtr.jobManager.TriggerJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/requests/%d", id))
}

// restartJobRequests - restart job-request
func (jraCtr *JobRequestAdminController) restartJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	err := jraCtr.jobManager.RestartJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/requests/%d", id))
}

// newJobRequest - creates a new job request
func (jraCtr *JobRequestAdminController) newJobRequest(c web.WebContext) error {
	request := types.JobRequest{ParamsJSON: "{}"}
	res := map[string]interface{}{
		"Request":  request,
		"JobTypes": jraCtr.getJobTypes(c),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/req/new", res)
}

func (jraCtr *JobRequestAdminController) getJobTypes(c web.WebContext) []string {
	jobTypes := make([]string, 0)
	qc := web.BuildQueryContext(c)
	if c.FormValue("jobType") != "" {
		jobTypes = []string{c.FormValue("jobType")}
	} else {
		if all, err := jraCtr.jobManager.GetJobTypesAsArray(qc); err == nil {
			for _, next := range all {
				jobTypes = append(jobTypes, next.JobType)
			}
		}
	}
	return jobTypes
}

// createJobRequest - saves a new job-request
func (jraCtr *JobRequestAdminController) createJobRequest(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	request := buildRequest(c)
	request.Errors = make(map[string]string)

	if err = buildRequestParams(c, request); err != nil {
		request.Errors["Error"] = err.Error()
		res := map[string]interface{}{
			"JobTypes": jraCtr.getJobTypes(c),
			"Request":  request,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/req/new", res)
	}

	saved, err := jraCtr.jobManager.SaveJobRequest(qc, request)
	if err != nil {
		request.Errors["Error"] = err.Error()
		res := map[string]interface{}{
			"Request":  request,
			"JobTypes": jraCtr.getJobTypes(c),
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/req/new", res)
	}
	_, _ = jraCtr.jobManager.SaveAudit(types.NewAuditRecordFromJobRequest(saved, types.JobRequestCreated, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/requests/%d", request.ID))
}

// getJobRequest - finds job-request by id
func (jraCtr *JobRequestAdminController) getJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	request, err := jraCtr.jobManager.GetJobRequest(qc, id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Request": request,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/req/view", res)
}

func (jraCtr *JobRequestAdminController) dotJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	d, err := jraCtr.jobManager.GetDotConfigForJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, d)
}

func (jraCtr *JobRequestAdminController) dotImageJobRequest(c web.WebContext) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	qc := web.BuildQueryContext(c)
	d, err := jraCtr.jobManager.GetDotImageForJobRequest(qc, id)
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "image/png", d)
}

func buildRequest(c web.WebContext) *types.JobRequest {
	request := types.NewRequest()
	request.Platform = c.FormValue("platform")
	request.JobType = c.FormValue("jobType")
	request.JobGroup = c.FormValue("jobGroup")
	request.JobPriority, _ = strconv.Atoi(c.FormValue("jobPriority"))
	request.JobState = common.PENDING
	request.JobExecutionID = ""
	request.ErrorCode = ""
	request.ErrorMessage = ""
	request.ScheduleAttempts = 0
	request.Retried = 0
	qc := web.BuildQueryContext(c)
	request.OrganizationID = qc.OrganizationID
	request.UserID = qc.UserID
	if request.ScheduledAt.IsZero() {
		request.ScheduledAt = time.Now()
	}
	request.CreatedAt = time.Now()
	request.UpdatedAt = time.Now()
	if params, err := c.FormParams(); err == nil {
		for k, v := range params {
			if !strings.Contains(types.ReservedRequestProperties, k) {
				_, _ = request.AddParam(k, v[0])
			}
		}
	}
	return request
}

func buildRequestParams(c web.WebContext, request *types.JobRequest) error {
	if err := request.SetParamsJSON(c.FormValue("params")); err != nil {
		return err
	}
	return nil
}
