package admin

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"mime/multipart"
	"net/http"
	"strings"
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

	webserver.GET("/dashboard/jobs/definitions", jdaCtr.queryJobDefinitions, acl.NewPermission(acl.JobDefinition, acl.Query)).Name = "query_admin_job_definitions"
	webserver.GET("/dashboard/jobs/plugins", jdaCtr.queryPlugins, acl.NewPermission(acl.JobDefinition, acl.Query)).Name = "query_admin_job_plugins"
	webserver.GET("/dashboard/jobs/definitions/:id", jdaCtr.getJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_admin_job_definitions"
	webserver.GET("/dashboard/jobs/definitions/:id/mermaid", jdaCtr.mermaidJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "mermaid_job_definition"
	webserver.GET("/dashboard/jobs/definitions/:id/dot", jdaCtr.dotJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "dot_job_definition"
	webserver.GET("/dashboard/jobs/definitions/:id/dot.png", jdaCtr.dotImageJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "dot_png_job_definition"
	webserver.GET("/dashboard/jobs/definitions/stats", jdaCtr.statsJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Metrics)).Name = "stats_admin_job_definition"
	webserver.GET("/dashboard/jobs/definitions/new", jdaCtr.newJobDefinitions, acl.NewPermission(acl.JobDefinition, acl.Create)).Name = "new_admin_job_definitions"
	webserver.GET("/dashboard/jobs/definitions/:id/edit", jdaCtr.editJobDefinitions, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "new_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions", jdaCtr.saveJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Create)).Name = "upload_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/upload", jdaCtr.uploadJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Create)).Name = "upload_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/disable", jdaCtr.disableJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Disable)).Name = "disable_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/enable", jdaCtr.enableJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Enable)).Name = "enable_admin_job_definitions"
	webserver.POST("/dashboard/jobs/definitions/:id/delete", jdaCtr.deleteJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Delete)).Name = "delete_admin_job_definitions"
	return jdaCtr
}

// ********************************* HTTP Handlers ***********************************
// queryJobDefinitions - queries job-definition
func (jdaCtr *JobDefinitionAdminController) queryJobDefinitions(c web.APIContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	if params["public_plugin"] == nil {
		params["public_plugin"] = false
	}
	qc := web.BuildQueryContext(c)
	recs, total, err := jdaCtr.jobManager.QueryJobDefinitions(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/jobs/definitions?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/index", res)
}
func (jdaCtr *JobDefinitionAdminController) queryPlugins(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	params["public_plugin"] = true
	jobs, total, err := jdaCtr.jobManager.QueryJobDefinitions(
		common.NewQueryContext(nil, ""),
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/jobs/plugins?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Jobs":       jobs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	for _, job := range jobs {
		job.CanEdit = (job.OrganizationID != "" &&
			qc.GetOrganizationID() != "" &&
			qc.GetOrganizationID() == job.OrganizationID) ||
			qc.GetUserID() == job.UserID || qc.IsAdmin()
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/plugins", res)
}

// statsJobDefinition - stats of job-definition
func (jdaCtr *JobDefinitionAdminController) statsJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobStats := jdaCtr.jobStatsRegistry.GetStats(qc, 0, 500)
	from := jdaCtr.startedAt
	to := jdaCtr.startedAt

	for _, c := range jobStats {
		if c.FirstJobAt != nil && c.FirstJobAt.Unix() < from.Unix() {
			from = *c.FirstJobAt
		}
		if c.LastJobAt != nil && c.LastJobAt.Unix() > to.Unix() {
			to = *c.LastJobAt
		}
	}

	res := map[string]interface{}{
		"Stats":    jobStats,
		"FromDate": from.Format("2006-01-02"),
		"ToDate":   to.Format("2006-01-02"),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/stats", res)
}

// uploadJobDefinitions - adds job-definition
func (jdaCtr *JobDefinitionAdminController) uploadJobDefinition(c web.APIContext) error {
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

func (jdaCtr *JobDefinitionAdminController) newJobDefinitions(c web.APIContext) error {
	job := &types.JobDefinition{}
	qc := web.BuildQueryContext(c)
	if c.QueryParam("plugin") != "" || c.FormValue("plugin") != "" {
		job.PublicPlugin = true
		job.RawYaml = `job_type: ` + qc.GetBundle() + `.sample-plugin
description: Simple Plugin example
public_plugin: true
sem_version: 1.0-dev
tasks:
- task_type: hello
  container:
    image: alpine
  script:
    - echo hello there
  on_completed: bye
- task_type: bye
  container:
    image: alpine
  script:
    - echo good bye
`
	} else {
		job.RawYaml = `job_type: sample-job
description: Simple job example
tasks:
- task_type: hello
  container:
    image: alpine
  script:
    - echo hello there
  on_completed: bye
- task_type: bye
  container:
    image: alpine
  script:
    - echo good bye
`
	}
	res := map[string]interface{}{
		"Job": job,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/new", res)
}

func (jdaCtr *JobDefinitionAdminController) editJobDefinitions(c web.APIContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	job, err := jdaCtr.jobManager.GetJobDefinition(qc, id)
	if err != nil {
		return nil
	}
	res := map[string]interface{}{
		"Job": job,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/new", res)
}

// saveJobDefinitions - save job-definition
func (jdaCtr *JobDefinitionAdminController) saveJobDefinition(c web.APIContext) (err error) {
	rawYaml := strings.TrimSpace(c.FormValue("raw_yaml"))
	job, err := types.NewJobDefinitionFromYaml([]byte(rawYaml))
	if err != nil {
		job := &types.JobDefinition{}
		job.RawYaml = rawYaml
		job.Errors = map[string]string{"Error": err.Error()}
		res := map[string]interface{}{
			"Job": job,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/def/new", res)
	}
	qc := web.BuildQueryContext(c)
	job.OrganizationID = qc.GetOrganizationID()
	job.UserID = qc.GetUserID()
	saved, err := jdaCtr.jobManager.SaveJobDefinition(qc, job)
	if err != nil {
		job.Errors = map[string]string{"Error": err.Error()}
		res := map[string]interface{}{
			"Job": job,
		}
		logrus.WithFields(logrus.Fields{
			"Component": "JobDefinitionAdminController",
			"Error":     err,
			"Raw":       rawYaml,
		}).Warnf("failed to save job definition")
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/def/new", res)
	}
	_, _ = jdaCtr.jobManager.SaveAudit(types.NewAuditRecordFromJobDefinition(saved, types.JobDefinitionUpdated, qc))
	if job.PublicPlugin {
		return c.Redirect(http.StatusFound, "/dashboard/jobs/plugins")
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
	job.OrganizationID = qc.GetOrganizationID()
	job.UserID = qc.GetUserID()
	saved, err := jdaCtr.jobManager.SaveJobDefinition(qc, job)
	if err != nil {
		return err
	}
	_, _ = jdaCtr.jobManager.SaveAudit(types.NewAuditRecordFromJobDefinition(saved, types.JobDefinitionUpdated, qc))
	return nil
}

// getJobDefinition - finds job-definition by id
func (jdaCtr *JobDefinitionAdminController) getJobDefinition(c web.APIContext) (err error) {
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

// disableJobDefinition - disable job-definition by id
func (jdaCtr *JobDefinitionAdminController) disableJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.DisableJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

// enableJobDefinition - disable job-definition by id
func (jdaCtr *JobDefinitionAdminController) enableJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.EnableJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

// deleteJobDefinition - deletes job-definition by id
func (jdaCtr *JobDefinitionAdminController) deleteJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jdaCtr.jobManager.DeleteJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/definitions")
}

func (jdaCtr *JobDefinitionAdminController) mermaidJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jdaCtr.jobManager.GetMermaidConfigForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, d)
}

func (jdaCtr *JobDefinitionAdminController) dotJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jdaCtr.jobManager.GetDotConfigForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, d)
}

func (jdaCtr *JobDefinitionAdminController) dotImageJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jdaCtr.jobManager.GetDotImageForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "image/png", d)
}
