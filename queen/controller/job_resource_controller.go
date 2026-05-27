package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"plexobject.com/formicary/internal/acl"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// JobResourceController structure
type JobResourceController struct {
	auditRecordRepository repository.AuditRecordRepository
	jobResourceRepository repository.JobResourceRepository
	webserver             web.Server
}

// NewJobResourceController instantiates controller for updating job-resources
func NewJobResourceController(
	auditRecordRepository repository.AuditRecordRepository,
	jobResourceRepository repository.JobResourceRepository,
	webserver web.Server) *JobResourceController {
	jobResCtrl := &JobResourceController{
		auditRecordRepository: auditRecordRepository,
		jobResourceRepository: jobResourceRepository,
		webserver:             webserver,
	}
	webserver.GET("/api/jobs/resources", jobResCtrl.queryJobResources, acl.NewPermission(acl.JobResource, acl.Query)).Name = "query_job_resources"
	webserver.GET("/api/jobs/resources/:id", jobResCtrl.getJobResource, acl.NewPermission(acl.JobResource, acl.View)).Name = "get_job_resource"
	webserver.POST("/api/jobs/resources", jobResCtrl.postJobResource, acl.NewPermission(acl.JobResource, acl.Create)).Name = "create_job_resource"
	webserver.POST("/api/jobs/resources/:id/disable", jobResCtrl.disableJobResource, acl.NewPermission(acl.JobResource, acl.Disable)).Name = "disable_job_resource"
	webserver.POST("/api/jobs/resources/:id/enable", jobResCtrl.enableJobResource, acl.NewPermission(acl.JobResource, acl.Enable)).Name = "enable_job_resource"
	webserver.PUT("/api/jobs/resources/:id", jobResCtrl.putJobResource, acl.NewPermission(acl.JobResource, acl.Update)).Name = "update_job_resource"
	webserver.DELETE("/api/jobs/resources/:id", jobResCtrl.deleteJobResource, acl.NewPermission(acl.JobResource, acl.Delete)).Name = "delete_job_resource"

	webserver.DELETE("/dashboard/jobs/resources/:id/configs/:config/delete", jobResCtrl.deleteJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Update)).Name = "delete_admin_job_resource_config"
	webserver.POST("/dashboard/jobs/resources/:id/configs", jobResCtrl.saveJobResourceConfig, acl.NewPermission(acl.JobResource, acl.Query)).Name = "save_admin_job_resource_config"
	return jobResCtrl
}

// ********************************* HTTP Handlers ***********************************

// Queries job resources by criteria such as type, platform, etc.
// responses:
//
//	200: jobResourceQueryResponse
func (jobResCtrl *JobResourceController) queryJobResources(c web.APIContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := jobResCtrl.jobResourceRepository.Query(
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

// Finds the job-resource by id.
// responses:
//
//	200: jobResource
func (jobResCtrl *JobResourceController) getJobResource(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	resource, err := jobResCtrl.jobResourceRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resource)
}

// Adds a job-resource that can be used for managing internal or external constraints.
// responses:
//
//	200: jobResource
func (jobResCtrl *JobResourceController) postJobResource(c web.APIContext) error {
	now := time.Now()
	resource := types.NewJobResource("", 0)
	err := json.NewDecoder(c.Request().Body).Decode(resource)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	resource.UserID = qc.GetUserID()
	resource.OrganizationID = qc.GetOrganizationID()
	saved, err := jobResCtrl.jobResourceRepository.Save(qc, resource)
	if err != nil {
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	_, _ = jobResCtrl.auditRecordRepository.Save(types.NewAuditRecordFromJobResource(saved, qc))
	return c.JSON(status, saved)
}

// Updates a job-resource that can be used for managing internal or external constraints.
// responses:
//
//	200: jobResource
func (jobResCtrl *JobResourceController) putJobResource(c web.APIContext) error {
	resource := types.NewJobResource("", 0)
	err := json.NewDecoder(c.Request().Body).Decode(resource)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	resource.UserID = qc.GetUserID()
	resource.OrganizationID = qc.GetOrganizationID()
	saved, err := jobResCtrl.jobResourceRepository.Save(qc, resource)
	if err != nil {
		return err
	}
	_, _ = jobResCtrl.auditRecordRepository.Save(types.NewAuditRecordFromJobResource(saved, qc))
	return c.JSON(http.StatusOK, saved)
}

// disables the job-resource so that any jobs requiring it will not be able to execute.
// responses:
//
//	200: emptyResponse
func (jobResCtrl *JobResourceController) disableJobResource(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobResCtrl.jobResourceRepository.SetDisabled(qc, c.Param("id"), true)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// disables the job-resource so that any jobs requiring it will not be able to execute.
// responses:
//
//	200: emptyResponse
func (jobResCtrl *JobResourceController) enableJobResource(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobResCtrl.jobResourceRepository.SetDisabled(qc, c.Param("id"), false)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Deletes the job-resource by id.
// responses:
//
//	200: emptyResponse
func (jobResCtrl *JobResourceController) deleteJobResource(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobResCtrl.jobResourceRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Deletes the job-resource config by id of job-resource and config-id.
// responses:
//
//	200: emptyResponse
func (jobResCtrl *JobResourceController) deleteJobResourceConfig(c web.APIContext) error {
	id := c.Param("id")
	cfgID := c.Param("config")
	qc := web.BuildQueryContext(c)
	resource, err := jobResCtrl.jobResourceRepository.Get(qc, id)
	if err != nil {
		return nil
	}
	config := resource.GetConfigByID(cfgID)
	if config == nil {
		return fmt.Errorf("failed to find config for %s", cfgID)
	}
	resource.DeleteConfig(config.Name)
	err = jobResCtrl.jobResourceRepository.DeleteConfig(qc, id, cfgID)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Save the job-resource config.
// responses:
//
//	200: jobResourceConfig
func (jobResCtrl *JobResourceController) saveJobResourceConfig(c web.APIContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	resource, err := jobResCtrl.jobResourceRepository.Get(qc, id)
	if err != nil {
		return nil
	}
	name := c.FormValue("name")
	value := c.FormValue("value")
	_, err = resource.AddConfig(name, value)
	if err != nil {
		return err
	}
	saved, err := jobResCtrl.jobResourceRepository.SaveConfig(qc, id, name, value)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, saved)
}

// ********************************* Swagger types ***********************************

// The params for querying jobResources.
type jobResourceQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// ResourceType defines type of resource such as Device, CPU, Memory
	ResourceType string `json:"resource_type"`
	// Description of resource
	Description string `json:"description"`
	// Platform can be OS platform or target runtime
	Platform string `json:"platform"`
}

// Paginated results of jobResources matching query
type jobResourceQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*types.JobResource
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// The parameters for finding job-request by id
type jobResourceIDParams struct {
	// in:path
	ID string `json:"id"`
}

// The request body includes job-resource for persistence.
type jobResourceCreateParams struct {
	// in:body
	Body types.JobResource
}

// The request body includes job-resource for persistence.
type jobResourceUpdateParams struct {
	// in:path
	ID string `json:"id"`
	// in:body
	Body types.JobResource
}

// JobResource represents a virtual resource, which can be used to implement mutex/semaphores for a job.
type jobResourceBody struct {
	// in:body
	Body types.JobResource
}

// The request params for querying/deleting resource-config
type jobResourceConfigDeleteParams struct {
	// in:path
	ID string `json:"id"`
	// in:path
	ConfigID string `json:"config_id"`
}

// jobResourceConfig represents config for the resource
type jobResourceConfigBody struct {
	// in:body
	Body types.JobResourceConfig
}
