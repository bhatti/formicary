package admin

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/types"
)

// JobDefinitionAdminController structure
type JobDefinitionAdminController struct {
	jobManager       *manager.JobManager
	resourceManager  resource.Manager
	jobStatsRegistry *stats.JobStatsRegistry
	webserver        web.Server
	startedAt        time.Time
}

// NewJobDefinitionAdminController admin dashboard for managing job-definitions
func NewJobDefinitionAdminController(
	jobManager *manager.JobManager,
	resourceManager resource.Manager,
	jobStatsRegistry *stats.JobStatsRegistry,
	webserver web.Server) *JobDefinitionAdminController {
	jdaCtr := &JobDefinitionAdminController{
		jobManager:       jobManager,
		jobStatsRegistry: jobStatsRegistry,
		resourceManager:  resourceManager,
		webserver:        webserver,
		startedAt:        time.Now(),
	}

	webserver.GET("/dashboard/jobs/definitions", jdaCtr.queryJobDefinitions, acl.New(acl.JobDefinition, acl.Query)).Name = "query_admin_job_definitions"
	webserver.GET("/dashboard/jobs/plugins", jdaCtr.queryPlugins, acl.New(acl.JobDefinition, acl.Query)).Name = "query_admin_job_plugins"
	webserver.POST("/dashboard/jobs/definitions/upload", jdaCtr.uploadJobDefinition, acl.New(acl.JobDefinition, acl.Create)).Name = "upload_admin_job_definitions"
	webserver.GET("/dashboard/jobs/definitions/:id", jdaCtr.getJobDefinition, acl.New(acl.JobDefinition, acl.View)).Name = "get_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/pause", jdaCtr.pauseJobDefinition, acl.New(acl.JobDefinition, acl.Pause)).Name = "pause_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/unpause", jdaCtr.unpauseJobDefinition, acl.New(acl.JobDefinition, acl.Unpause)).Name = "unpause_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/delete", jdaCtr.deleteJobDefinition, acl.New(acl.JobDefinition, acl.Delete)).Name = "delete_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/pause", jdaCtr.pauseJobDefinition, acl.New(acl.JobDefinition, acl.Pause)).Name = "pause_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/unpause", jdaCtr.unpauseJobDefinition, acl.New(acl.JobDefinition, acl.Unpause)).Name = "unpause_admin_job_definitions"
	webserver.GET("/dashboard/jobs/definitions/:id/dot", jdaCtr.dotJobDefinition, acl.New(acl.JobDefinition, acl.View)).Name = "dot_job_definition"
	webserver.GET("/dashboard/jobs/definitions/:id/dot.png", jdaCtr.dotImageJobDefinition, acl.New(acl.JobDefinition, acl.View)).Name = "dot_png_job_definition"
	webserver.GET("/dashboard/jobs/definitions/stats", jdaCtr.statsJobDefinition, acl.New(acl.JobDefinition, acl.Metrics)).Name = "stats_admin_job_definition"
	return jdaCtr
}

// ********************************* HTTP Handlers ***********************************
// queryJobDefinitions - queries job-definition
func (jdaCtr *JobDefinitionAdminController) queryJobDefinitions(c web.WebContext) error {
	params, order, page, pageSize, q := controller.ParseParams(c)
	if params["public_plugin"] == nil {
		params["public_plugin"] = false
	}
	qc := web.BuildQueryContext(c)
	jobs, total, err := jdaCtr.jobManager.QueryJobDefinitions(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/jobs/definitions?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{"Jobs": jobs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/index", res)
}
func (jdaCtr *JobDefinitionAdminController) queryPlugins(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q := controller.ParseParams(c)
	params["public_plugin"] = true
	jobs, total, err := jdaCtr.jobManager.QueryJobDefinitions(common.NewQueryContext("", "", ""), params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/jobs/plugins?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{"Jobs": jobs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
	}
	for _, job := range jobs {
		job.CanEdit = (job.OrganizationID != "" && qc.OrganizationID != "" && qc.OrganizationID == job.OrganizationID) || qc.UserID == job.UserID || qc.Admin()
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/plugins", res)
}

// statsJobDefinition - stats of job-definition
func (jdaCtr *JobDefinitionAdminController) statsJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	jobStats := jdaCtr.jobStatsRegistry.GetStats(qc, 0, 500)
	res := map[string]interface{}{"Stats": jobStats,
		"FromDate": jdaCtr.startedAt.Format("Jan _2, 15:04:05 MST"),
		"ToDate":   time.Now().Format("Jan _2, 15:04:05 MST"),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/stats", res)
}

// uploadJobDefinitions - adds job-definition
func (jdaCtr *JobDefinitionAdminController) uploadJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["files"]

	for _, file := range files {
		err := jdaCtr.uploadFile(qc, file)
		if err != nil {
			return err
		}
	}

	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

func (jdaCtr *JobDefinitionAdminController) uploadFile(
	qc *common.QueryContext,
	file *multipart.FileHeader) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(src); err != nil {
		return err
	}
	job, err := types.NewJobDefinitionFromYaml(buf.Bytes())
	if err != nil {
		return err
	}
	job.OrganizationID = qc.OrganizationID
	job.UserID = qc.UserID
	saved, err := jdaCtr.jobManager.SaveJobDefinition(qc, job)
	if err != nil {
		return err
	}
	_, _ = jdaCtr.jobManager.SaveAudit(types.NewAuditRecordFromJobDefinition(saved, types.JobDefinitionUpdated, qc))
	return nil
}

// getJobDefinition - finds job-definition by id
func (jdaCtr *JobDefinitionAdminController) getJobDefinition(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	version := c.QueryParam("version")
	var job *types.JobDefinition
	if len(id) == 36 {
		job, err = jdaCtr.jobManager.GetJobDefinition(qc, id)
		if err != nil {
			job, err = jdaCtr.jobManager.GetJobDefinitionByType(qc, id, version)
		}
		if err != nil {
			return err
		}
	} else {
		job, err = jdaCtr.jobManager.GetJobDefinitionByType(qc, id, version)
		if err != nil {
			job, err = jdaCtr.jobManager.GetJobDefinition(qc, id)
		}
		if err != nil {
			return err
		}
	}

	reservations, err := jdaCtr.resourceManager.CheckJobResources(job)
	res := map[string]interface{}{
		"Definition":      job,
		"AllocationError": err,
		"Allocations":     reservations,
	}

	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/view", res)
}

// pauseJobDefinition - pause job-definition by id
func (jdaCtr *JobDefinitionAdminController) pauseJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.PauseJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

// unpauseJobDefinition - pause job-definition by id
func (jdaCtr *JobDefinitionAdminController) unpauseJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.UnpauseJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

// deleteJobDefinition - deletes job-definition by id
func (jdaCtr *JobDefinitionAdminController) deleteJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.DeleteJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

func (jdaCtr *JobDefinitionAdminController) dotJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jdaCtr.jobManager.GetDotConfigForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, d)
}

func (jdaCtr *JobDefinitionAdminController) dotImageJobDefinition(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jdaCtr.jobManager.GetDotImageForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "image/png", d)
}
