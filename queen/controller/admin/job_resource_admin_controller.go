package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// JobResourceAdminController structure
type JobResourceAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	jobResourceRepository repository.JobResourceRepository
	webserver             web.Server
}

// NewJobResourceAdminController admin dashboard for managing job-resources
func NewJobResourceAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	jobResourceRepository repository.JobResourceRepository,
	webserver web.Server) *JobResourceAdminController {
	jraCtr := &JobResourceAdminController{
		auditRecordRepository: auditRecordRepository,
		jobResourceRepository: jobResourceRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/jobs/resources", jraCtr.queryJobResources, acl.NewPermission(acl.JobResource, acl.Query)).Name = "query_admin_job_resources"
	webserver.GET("/dashboard/jobs/resources/new", jraCtr.newJobResource, acl.NewPermission(acl.JobResource, acl.Create)).Name = "new_admin_job_resources"
	webserver.POST("/dashboard/jobs/resources", jraCtr.createJobResource, acl.NewPermission(acl.JobResource, acl.Create)).Name = "create_admin_job_resources"
	webserver.POST("/dashboard/jobs/resources/:id", jraCtr.updateJobResource, acl.NewPermission(acl.JobResource, acl.Update)).Name = "update_admin_job_resources"
	webserver.POST("/dashboard/jobs/resources/:id/pause", jraCtr.pauseJobResource, acl.NewPermission(acl.JobResource, acl.Pause)).Name = "pause_admin_job_resources"
	webserver.POST("/dashboard/jobs/resources/:id/unpause", jraCtr.unpauseJobResource, acl.NewPermission(acl.JobResource, acl.Unpause)).Name = "unpause_admin_job_resources"
	webserver.GET("/dashboard/jobs/resources/:id", jraCtr.getJobResource, acl.NewPermission(acl.JobResource, acl.View)).Name = "get_admin_job_resources"
	webserver.GET("/dashboard/jobs/resources/:id/edit", jraCtr.editJobResource, acl.NewPermission(acl.JobResource, acl.Update)).Name = "edit_admin_job_resources"
	webserver.POST("/dashboard/jobs/resources/:id/delete", jraCtr.deleteJobResource, acl.NewPermission(acl.JobResource, acl.Delete)).Name = "delete_admin_job_resources"
	webserver.GET("/dashboard/jobs/resources/:id/configs/new", jraCtr.newJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Update)).Name = "new_admin_job_resource_config"
	webserver.GET("/dashboard/jobs/resources/:id/configs/:config/edit", jraCtr.editJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Update)).Name = "edit_admin_job_resource_config"
	webserver.POST("/dashboard/jobs/resources/:id/configs/:config/delete", jraCtr.deleteJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Update)).Name = "delete_admin_job_resource_config"
	webserver.POST("/dashboard/jobs/resources/:id/configs", jraCtr.saveJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Query)).Name = "save_admin_job_resource_config"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryJobResources - queries job-resource
func (jraCtr *JobResourceAdminController) queryJobResources(c web.WebContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := jraCtr.jobResourceRepository.Query(
		qc,
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/jobs/resources?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/index", res)
}

// createJobResource - saves a new job-resource
func (jraCtr *JobResourceAdminController) createJobResource(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	resource := buildResource(c)
	err = resource.Validate()
	if err == nil {
		resource, err = jraCtr.jobResourceRepository.Save(qc, resource)
	}
	if err != nil {
		res := map[string]interface{}{
			"Resource": resource,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/res/new", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromJobResource(resource, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/resources/%s", resource.ID))
}

// pauseJobResources - update job-resource
func (jraCtr *JobResourceAdminController) pauseJobResource(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	err := jraCtr.jobResourceRepository.SetPaused(qc, id, true)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/resources")
}

// unpauseJobResources - update job-resource
func (jraCtr *JobResourceAdminController) unpauseJobResource(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	err := jraCtr.jobResourceRepository.SetPaused(qc, id, false)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/resources")
}

// updateJobResource - updates job-resource
func (jraCtr *JobResourceAdminController) updateJobResource(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	resource := buildResource(c)
	resource.ID = c.Param("id")
	err = resource.Validate()

	if err == nil {
		resource, err = jraCtr.jobResourceRepository.Save(qc, resource)
	}
	if err != nil {
		res := map[string]interface{}{
			"Resource": resource,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/res/edit", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromJobResource(resource, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/resources/%s", resource.ID))
}

// newJobResource - creates a new job resource
func (jraCtr *JobResourceAdminController) newJobResource(c web.WebContext) error {
	resource := types.NewJobResource("", 1)
	res := map[string]interface{}{
		"Resource": resource,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/new", res)
}

// getJobResource - finds job-resource by id
func (jraCtr *JobResourceAdminController) getJobResource(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Resource": resource,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/view", res)
}

// editJobResource - shows job-resource for edit
func (jraCtr *JobResourceAdminController) editJobResource(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		resource = types.NewJobResource("", 0)
		resource.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "JobResourceAdminController",
				"Error":     err,
				"ID":        id,
			}).Debug("failed to find resource")
		}
	}
	res := map[string]interface{}{
		"Resource": resource,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/edit", res)
}

// deleteJobResource - deletes job-resource by id
func (jraCtr *JobResourceAdminController) deleteJobResource(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jraCtr.jobResourceRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/jobs/resources")
}

// newJobResourceConfig - creates a new config for resource
func (jraCtr *JobResourceAdminController) newJobResourceConfig(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		return nil
	}
	config, _ := types.NewJobResourceConfig("", "", false)
	res := map[string]interface{}{
		"Resource": resource,
		"Config":   config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/config_edit", res)
}

// editJobResourceConfig - edits config for resource
func (jraCtr *JobResourceAdminController) editJobResourceConfig(c web.WebContext) error {
	id := c.Param("id")
	cfgID := c.Param("config")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		return nil
	}
	cfg := resource.GetConfigByID(cfgID)
	if cfg == nil {
		return fmt.Errorf("failed to find config for %s", cfgID)
	}
	res := map[string]interface{}{
		"Resource": resource,
		"Config":   cfg,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/res/config_edit", res)
}

// deleteJobResourceConfig - delete config for resource
func (jraCtr *JobResourceAdminController) deleteJobResourceConfig(c web.WebContext) error {
	id := c.Param("id")
	cfgID := c.Param("config")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		return nil
	}
	config := resource.GetConfigByID(cfgID)
	if config == nil {
		return fmt.Errorf("failed to find config for %s", cfgID)
	}
	resource.DeleteConfig(config.Name)
	err = jraCtr.jobResourceRepository.DeleteConfig(qc, id, cfgID)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/resources/%s", id))
}

// saveJobResourceConfig - delete config for resource
func (jraCtr *JobResourceAdminController) saveJobResourceConfig(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	resource, err := jraCtr.jobResourceRepository.Get(qc, id)
	if err != nil {
		return err
	}
	name := c.FormValue("name")
	value := c.FormValue("value")
	_, err = resource.AddConfig(name, value)
	if err == nil {
		_, err = jraCtr.jobResourceRepository.SaveConfig(qc, id, name, value)
	}
	if err != nil {
		cfg := &types.JobResourceConfig{NameTypeValue: common.NameTypeValue{Name: name, Value: value}}
		cfg.Errors = map[string]string{"Error": err.Error()}
		res := map[string]interface{}{
			"Config":   cfg,
			"Resource": resource,
			"Error":    err,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/res/config_edit", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/resources/%s", id))
}

func buildResource(c web.WebContext) *types.JobResource {
	quota, _ := strconv.Atoi(c.FormValue("quota"))
	resource := types.NewJobResource(c.FormValue("resourceType"), quota)
	resource.Platform = c.FormValue("platform")
	resource.Category = c.FormValue("category")
	resource.Tags = strings.Split(c.FormValue("tags"), ",")
	resource.ExternalID = c.FormValue("externalID")
	resource.LeaseTimeout, _ = time.ParseDuration(c.FormValue("leaseTimeout"))
	qc := web.BuildQueryContext(c)
	resource.OrganizationID = qc.GetOrganizationID()
	resource.UserID = qc.GetUserID()
	return resource
}
