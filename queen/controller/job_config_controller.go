package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/internal/acl"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// JobConfigController structure
type JobConfigController struct {
	auditRecordRepository   repository.AuditRecordRepository
	jobDefinitionRepository repository.JobDefinitionRepository
	webserver               web.Server
}

// NewJobConfigController instantiates controller for updating job-configs
func NewJobConfigController(
	auditRecordRepository repository.AuditRecordRepository,
	jobDefinitionRepository repository.JobDefinitionRepository,
	webserver web.Server) *JobConfigController {
	cfgCtrl := &JobConfigController{
		auditRecordRepository:   auditRecordRepository,
		jobDefinitionRepository: jobDefinitionRepository,
		webserver:               webserver,
	}
	webserver.GET("/api/jobs/definitions/:job/configs", cfgCtrl.queryJobConfigs, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "query_job_configs"
	webserver.GET("/api/jobs/definitions/:job/configs/:id", cfgCtrl.getJobConfig, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_job_config"
	webserver.POST("/api/jobs/definitions/:job/configs", cfgCtrl.postJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "create_job_config"
	webserver.PUT("/api/jobs/definitions/:job/configs/:id", cfgCtrl.putJobConfig, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "update_job_config"
	webserver.DELETE("/api/jobs/definitions/:job/configs/:id", cfgCtrl.deleteJobConfig, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "delete_job_config"
	return cfgCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/jobs/definitions/{jobId}/configs job-configs queryJobConfigs
// Queries job configs by criteria such as name, type, etc.
// responses:
//   200: jobConfigQueryResponse
func (cc *JobConfigController) queryJobConfigs(c web.APIContext) error {
	jobID := c.Param("job")
	qc := web.BuildQueryContext(c)
	job, err := cc.jobDefinitionRepository.Get(qc, jobID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, job.Configs)
}

// swagger:route POST /api/jobs/definitions/{jobId}/configs job-configs postJobConfig
// Adds a config for the job.
// responses:
//   200: jobConfig
func (cc *JobConfigController) postJobConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID, err := cc.getJobID(c, qc)
	if err != nil {
		return err
	}

	now := time.Now()
	cfg, err := types.NewJobDefinitionConfig("", "", false)
	if err != nil {
		return err
	}
	err = json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	saved, err := cc.jobDefinitionRepository.SaveConfig(qc, jobID, cfg.Name, cfg.Value, cfg.Secret)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobConfigController",
			"Config":    cfg,
			"Error":     err,
		}).Warn("failed to save job config")
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromJobDefinitionConfig(saved, types.JobDefinitionUpdated, qc))
	return c.JSON(status, saved)
}

// swagger:route PUT /api/jobs/definitions/{jobId}/configs/{id} job-configs putJobConfig
// Updates a config for the job.
// responses:
//   200: jobConfig
func (cc *JobConfigController) putJobConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID, err := cc.getJobID(c, qc)
	if err != nil {
		return err
	}
	cfg, err := types.NewJobDefinitionConfig("", "", false)
	if err != nil {
		return err
	}
	err = json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	saved, err := cc.jobDefinitionRepository.SaveConfig(qc, jobID, cfg.Name, cfg.Value, cfg.Secret)
	if err != nil {
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromJobDefinitionConfig(saved, types.JobDefinitionUpdated, qc))
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/jobs/definitions/{jobId}/configs/{id} job-configs getJobConfig
// Finds a config for the job by id.
// responses:
//   200: jobConfig
func (cc *JobConfigController) getJobConfig(c web.APIContext) error {
	jobID := c.Param("job")
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	job, err := cc.jobDefinitionRepository.Get(qc, jobID)
	if err != nil {
		return err
	}
	cfg := job.GetConfigByID(id)
	if cfg == nil {
		return c.String(http.StatusNotFound, fmt.Sprint("no config with matching id"))
	}
	return c.JSON(http.StatusOK, cfg)
}

// swagger:route DELETE /api/jobs/definitions/{jobId}/configs/{id} job-configs deleteJobConfig
// Deletes a config for the job by id.
// responses:
//   200: emptyResponse
// deleteJobConfig - deletes job-config by id
func (cc *JobConfigController) deleteJobConfig(c web.APIContext) error {
	jobID := c.Param("job")
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	err := cc.jobDefinitionRepository.DeleteConfig(qc, jobID, id)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryOrgConfigs
// The params for querying jobConfigs.
type jobConfigQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Name defines name of property
	Name string `yaml:"name" json:"name"`
}

// Paginated results of jobConfigs matching query
// swagger:response jobConfigQueryResponse
type jobConfigQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []types.JobDefinitionConfig
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters deleteJobConfig getJobConfig
// The parameters for accessing job-config by id
type jobConfigIDParams struct {
	// in:path
	OrgID string `json:"jobId"`
	// in:path
	ID string `json:"id"`
}

// swagger:parameters putJobConfig
// The parameters for updating job config by id
type jobConfigUpdateParams struct {
	// in:path
	OrgID string `json:"jobId"`
	// in:path
	ID string `json:"id"`
	// in:body
	Body types.JobDefinitionConfig
}

// swagger:parameters postJobConfig
// The request body includes job-request for persistence.
type jobConfigParams struct {
	// in:body
	Body types.JobDefinitionConfig
}

// OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution.
// swagger:response jobConfig
type jobConfigBody struct {
	// in:body
	Body types.JobDefinitionConfig
}

func (cc *JobConfigController) getJobID(
	c web.APIContext,
	qc *common.QueryContext) (string, error) {
	jobID := c.Param("job")
	job, err := cc.jobDefinitionRepository.Get(qc, jobID)
	if job == nil {
		job, err = cc.jobDefinitionRepository.GetByType(qc, jobID)
		if err != nil {
			return "", err
		}
		jobID = job.ID
	}
	return jobID, nil
}
