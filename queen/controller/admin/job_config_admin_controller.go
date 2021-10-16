package admin

import (
	"fmt"
	"net/http"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/internal/acl"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// JobConfigAdminController structure
type JobConfigAdminController struct {
	auditRecordRepository   repository.AuditRecordRepository
	jobDefinitionRepository repository.JobDefinitionRepository
	webserver               web.Server
}

// NewJobConfigAdminController admin dashboard for managing job-configs
func NewJobConfigAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	jobDefinitionRepository repository.JobDefinitionRepository,
	webserver web.Server) *JobConfigAdminController {
	jraCtr := &JobConfigAdminController{
		auditRecordRepository:   auditRecordRepository,
		jobDefinitionRepository: jobDefinitionRepository,
		webserver:               webserver,
	}
	webserver.GET("/dashboard/jobs/definitions/:job/configs", jraCtr.queryJobConfigs, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "query_admin_job_configs"
	webserver.GET("/dashboard/jobs/definitions/:job/configs/new", jraCtr.newJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "new_admin_job_configs"
	webserver.POST("/dashboard/jobs/definitions/:job/configs", jraCtr.createJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "create_admin_job_configs"
	webserver.POST("/dashboard/jobs/definitions/:job/configs/:id", jraCtr.updateJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "update_admin_job_configs"
	webserver.GET("/dashboard/jobs/definitions/:job/configs/:id", jraCtr.getJobConfig, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_admin_job_configs"
	webserver.GET("/dashboard/jobs/definitions/:job/configs/:id/edit", jraCtr.editJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "edit_admin_job_configs"
	webserver.POST("/dashboard/jobs/definitions/:job/configs/:id/delete", jraCtr.deleteJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "delete_admin_job_configs"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryJobConfigs - queries job-config
func (jraCtr *JobConfigAdminController) queryJobConfigs(c web.APIContext) error {
	jobID := c.Param("job")
	qc := web.BuildQueryContext(c)
	job, err := jraCtr.jobDefinitionRepository.Get(qc, jobID)
	if err != nil {
		return err
	}
	res := map[string]interface{}{"Configs": job.Configs,
		"Job": job,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/configs/index", res)
}

// createJobConfig - saves a new job-config
func (jraCtr *JobConfigAdminController) createJobConfig(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job")
	name := c.FormValue("name")
	value := c.FormValue("value")
	secret := c.FormValue("secret")
	saved, err := jraCtr.jobDefinitionRepository.SaveConfig(qc, jobID, name, value, secret == "on")
	if err != nil {
		saved = &types.JobDefinitionConfig{NameTypeValue: common.NameTypeValue{Name: name, Value: value}}
		res := map[string]interface{}{
			"Config": saved,
			"Error":  err,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/def/configs/new", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromJobDefinitionConfig(saved, types.JobDefinitionUpdated, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/definitions/%s", jobID))
}

// updateJobConfig - updates job-config
func (jraCtr *JobConfigAdminController) updateJobConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job")
	name := c.FormValue("name")
	value := c.FormValue("value")
	secret := c.FormValue("secret")
	saved, err := jraCtr.jobDefinitionRepository.SaveConfig(qc, jobID, name, value, secret == "on")
	if err != nil {
		saved = &types.JobDefinitionConfig{NameTypeValue: common.NameTypeValue{Name: name, Value: value}}
		res := map[string]interface{}{
			"Config": saved,
			"Error":  err,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "jobs/def/configs/edit", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromJobDefinitionConfig(saved, types.JobDefinitionUpdated, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/definitions/%s", jobID))
}

// newJobConfig - creates a new job config
func (jraCtr *JobConfigAdminController) newJobConfig(c web.APIContext) error {
	jobID := c.Param("job")
	config := &types.JobDefinitionConfig{JobDefinitionID: jobID}
	if jobID == "" {
		config.Errors = map[string]string{"Error": "job-id is not specified"}
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/configs/new", res)
}

// getJobConfig - finds job-config by id
func (jraCtr *JobConfigAdminController) getJobConfig(c web.APIContext) error {
	jobID := c.Param("job")
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	job, err := jraCtr.jobDefinitionRepository.Get(qc, jobID)
	if err != nil {
		return err
	}
	cfg := job.GetConfigByID(id)
	if cfg == nil {
		return c.String(http.StatusNotFound, fmt.Sprint("no config with matching id"))
	}
	res := map[string]interface{}{
		"Config": cfg,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/configs/view", res)
}

// editJobConfig - shows job-config for edit
func (jraCtr *JobConfigAdminController) editJobConfig(c web.APIContext) error {
	jobID := c.Param("job")
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	job, err := jraCtr.jobDefinitionRepository.Get(qc, jobID)
	if err != nil {
		return err
	}
	cfg := job.GetConfigByID(id)
	if cfg == nil {
		return c.String(http.StatusNotFound, fmt.Sprint("no config with matching id"))
	}
	res := map[string]interface{}{
		"Job":    job,
		"Config": cfg,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "jobs/def/configs/edit", res)
}

// deleteJobConfig - deletes job-config by id
func (jraCtr *JobConfigAdminController) deleteJobConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job")
	id := c.Param("id")
	err := jraCtr.jobDefinitionRepository.DeleteConfig(qc, jobID, id)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/jobs/definitions/%s", jobID))
}
