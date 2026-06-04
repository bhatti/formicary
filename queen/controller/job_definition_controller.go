package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/stats"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/queen/manager"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/types"
)

// JobDefinitionController structure
type JobDefinitionController struct {
	jobManager       *manager.JobManager
	jobStatsRegistry *stats.JobStatsRegistry
	webserver        web.Server
}

// NewJobDefinitionController instantiates controller for updating job-definitions
func NewJobDefinitionController(
	jobManager *manager.JobManager,
	jobStatsRegistry *stats.JobStatsRegistry,
	webserver web.Server) *JobDefinitionController {
	jobDefCtrl := &JobDefinitionController{
		jobManager:       jobManager,
		jobStatsRegistry: jobStatsRegistry,
		webserver:        webserver,
	}

	webserver.GET("/api/jobs/definitions", jobDefCtrl.queryJobDefinitions, acl.NewPermission(acl.JobDefinition, acl.Query)).Name = "query_job_definitions"
	webserver.GET("/api/jobs/plugins", jobDefCtrl.queryPlugins, acl.NewPermission(acl.JobDefinition, acl.Query)).Name = "query_job_plugins"
	webserver.GET("/api/jobs/definitions/:id", jobDefCtrl.getJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_job_definition"
	webserver.GET("/api/jobs/definitions/:id/mermaid", jobDefCtrl.mermaidJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "mermaid_job_definition"
	webserver.GET("/api/jobs/definitions/:id/dot", jobDefCtrl.dotJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "dot_job_definition"
	webserver.GET("/api/jobs/definitions/:id/dot.png", jobDefCtrl.dotImageJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "dot_png_job_definition"
	webserver.GET("/api/jobs/definitions/type/:type/versions", jobDefCtrl.getJobDefinitionVersions, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_job_definition_versions"
	webserver.GET("/api/jobs/definitions/:type/yaml", jobDefCtrl.getYamlJobDefinition, acl.NewPermission(acl.JobDefinition, acl.View)).Name = "get_yaml_job_definition"
	webserver.GET("/api/jobs/definitions/stats", jobDefCtrl.statsJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Metrics)).Name = "stats_job_definition"
	webserver.POST("/api/jobs/definitions", jobDefCtrl.postJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Create)).Name = "create_job_definition"
	webserver.POST("/api/jobs/definitions/:id/disable", jobDefCtrl.disableJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Disable)).Name = "disable_job_definitions"
	webserver.POST("/api/jobs/definitions/:id/enable", jobDefCtrl.enableJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Enable)).Name = "enable_job_definitions"
	webserver.PUT("/api/jobs/definitions/:id/concurrency", jobDefCtrl.updateConcurrencyJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Update)).Name = "update_concurrency_job_definition"
	webserver.DELETE("/api/jobs/definitions/:id", jobDefCtrl.deleteJobDefinition, acl.NewPermission(acl.JobDefinition, acl.Delete)).Name = "delete_job_definition"
	return jobDefCtrl
}

// ********************************* HTTP Handlers ***********************************

// Queries job definitions by criteria such as type, platform, etc.
// responses:
//
//	200: jobDefinitionQueryResponse
func (jobDefCtrl *JobDefinitionController) queryJobDefinitions(c web.APIContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	if params["public_plugin"] == nil {
		params["public_plugin"] = false
	}
	qc := web.BuildQueryContext(c)
	recs, total, err := jobDefCtrl.jobManager.QueryJobDefinitions(
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

// Queries job definitions by criteria such as type, platform, etc.
// responses:
//
//	200: jobDefinitionQueryResponse
func (jobDefCtrl *JobDefinitionController) queryPlugins(c web.APIContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	params["public_plugin"] = true
	recs, total, err := jobDefCtrl.jobManager.QueryJobDefinitions(
		common.NewQueryContext(nil, ""),
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// Uploads job definitions using JSON or YAML body based on content-type header.
// responses:
//
//	200: jobDefinition
func (jobDefCtrl *JobDefinitionController) postJobDefinition(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	job := types.NewJobDefinition("")
	contentType := c.Request().Header.Get("content-type")
	yamlFormat := strings.Contains(strings.ToLower(contentType), "yaml")
	var b []byte
	b, err = ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return common.NewValidationError(
			fmt.Errorf("failed to load yaml job due to %w", err))
	}
	// checking yaml format
	if yamlFormat {
		job, err = types.NewJobDefinitionFromYaml(b)
	} else {
		err = json.Unmarshal(b, job)
		job.UpdateRawYaml()
	}
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":  "JobDefinitionController",
			"YAMLFormat": yamlFormat,
			"Length":     len(b),
			"YAML":       string(b),
			"Error":      err,
		}).Warnf("failed to unmarshal")
		return common.NewValidationError(
			fmt.Errorf("unable to unmarshal due to %w", err))
	}
	job.UserID = qc.GetUserID()
	job.OrganizationID = qc.GetOrganizationID()
	saved, err := jobDefCtrl.jobManager.SaveJobDefinition(qc, job)
	if err != nil {
		return err
	}
	_, _ = jobDefCtrl.jobManager.SaveAudit(types.NewAuditRecordFromJobDefinition(saved, types.JobDefinitionUpdated, qc))
	status := 0
	if saved.Version == 0 {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}

	logrus.WithFields(logrus.Fields{
		"Component":  "JobDefinitionController",
		"JobType":    saved.JobType,
		"SemVersion": saved.SemVersion,
		"Version":    saved.Version,
		"Length":     len(b),
	}).Info("updated job definition")
	return c.JSON(status, saved)
}

// disables job-definition so that no new requests are executed while in-progress jobs are allowed to complete.
// responses:
//
//	200: emptyResponse
func (jobDefCtrl *JobDefinitionController) disableJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobDefCtrl.jobManager.DisableJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Enables job-definition so that new requests can start processing.
// responses:
//
//	200: emptyResponse
func (jobDefCtrl *JobDefinitionController) enableJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobDefCtrl.jobManager.EnableJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Finds the job-definition by id.
// responses:
//
//	200: jobDefinition
func (jobDefCtrl *JobDefinitionController) getJobDefinition(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	version := c.QueryParam("version")
	var job *types.JobDefinition
	if len(id) == 36 {
		job, err = jobDefCtrl.jobManager.GetJobDefinition(qc, id)
		if err != nil {
			job, err = jobDefCtrl.jobManager.GetJobDefinitionByType(qc, id, version)
		}
		if err != nil {
			return err
		}
	} else {
		job, err = jobDefCtrl.jobManager.GetJobDefinitionByType(qc, id, version)
		if err != nil {
			job, err = jobDefCtrl.jobManager.GetJobDefinition(qc, id)
		}
		if err != nil {
			return err
		}
	}
	return c.JSON(http.StatusOK, job)
}

// Finds job-definition by type and returns response YAML format.
// responses:
//
//	200: jobDefinition
func (jobDefCtrl *JobDefinitionController) getYamlJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	b, err := jobDefCtrl.jobManager.GetYamlJobDefinitionByType(qc, c.Param("type"))
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, string(b))
}

// Returns all versions of a job definition by type, ordered by version descending.
// responses:
//
//	200: jobDefinitionVersionsResponse
func (jobDefCtrl *JobDefinitionController) getJobDefinitionVersions(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobType := c.Param("type")
	_, _, page, pageSize, _, _ := ParseParams(c)
	versions, total, err := jobDefCtrl.jobManager.GetJobDefinitionVersions(qc, jobType, page, pageSize)
	if err != nil {
		return err
	}
	type VersionSummary struct {
		ID         string `json:"id"`
		Version    int32  `json:"version"`
		SemVersion string `json:"sem_version"`
		Active     bool   `json:"active"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	}
	summaries := make([]VersionSummary, len(versions))
	for i, v := range versions {
		summaries[i] = VersionSummary{
			ID:         v.ID,
			Version:    v.Version,
			SemVersion: v.SemVersion,
			Active:     v.Active,
			CreatedAt:  v.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:  v.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(summaries, total, page, pageSize))
}

// Updates the concurrency for job-definition by id to limit the maximum jobs that can be executed at the same time.
// responses:
//
//	200: emptyResponse
func (jobDefCtrl *JobDefinitionController) updateConcurrencyJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	concurrency, err := strconv.Atoi(c.FormValue("concurrency"))
	if err != nil {
		return common.NewValidationError(
			fmt.Errorf("failed to parse concurrent value due to %w", err))
	}
	err = jobDefCtrl.jobManager.SetJobDefinitionMaxConcurrency(qc, c.Param("id"), concurrency)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Deletes the job-definition by id.
// responses:
//
//	200: emptyResponse
func (jobDefCtrl *JobDefinitionController) deleteJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := jobDefCtrl.jobManager.DeleteJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// Returns Mermaid definition for the graph of tasks defined in the job.
// responses:
//
//	200: stringResponse
func (jobDefCtrl *JobDefinitionController) mermaidJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jobDefCtrl.jobManager.GetMermaidConfigForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, d)
}

// Returns Graphviz DOT definition for the graph of tasks defined in the job.
// responses:
//
//	200: stringResponse
func (jobDefCtrl *JobDefinitionController) dotJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jobDefCtrl.jobManager.GetDotConfigForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, d)
}

// Returns Graphviz DOT image for the graph of tasks defined in the job.
// responses:
//
//	200: byteResponse
func (jobDefCtrl *JobDefinitionController) dotImageJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	d, err := jobDefCtrl.jobManager.GetDotImageForJobDefinition(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "image/png", d)
}

// Returns Real-time statistics of jobs running.
// responses:
//
//	200: jobDefinitionStatsResponse
//
// statsJobDefinition - stats of job-definition
func (jobDefCtrl *JobDefinitionController) statsJobDefinition(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobStats := jobDefCtrl.jobStatsRegistry.GetStats(qc, 0, 500)
	return c.JSON(http.StatusOK, jobStats)
}

// ********************************* Swagger types ***********************************

// The params for querying jobDefinitions.
type jobDefinitionQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// JobType defines a unique type of job
	JobType string `yaml:"job_type" json:"job_type"`
	// Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering
	Platform string `json:"platform"`
	// disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging.
	Disabled bool `json:"disabled"`
	// PublicPlugin means job is public plugin
	PublicPlugin bool `json:"public_plugin"`
	// Tags is aggregation of task tags, and it can be searched via `tags:in`
	Tags string `json:"tags"`
}

// Paginated results of jobDefinitions matching query
type jobDefinitionQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []types.JobDefinition
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// The job-definition can be specified in JSON or YAML format based on content-type
type jobDefinitionUploadParams struct {
	// in:body
	Body types.JobDefinition
}

// The job-definition defines DAG (directed acyclic graph) of tasks, which are executed by
// ant followers. The workflow of job uses task exit codes to define next task to execute.
type jobDefinitionBody struct {
	// in:body
	Body types.JobDefinition
}

// Real-time statistics of jobs that are recently completed or being executed.
type jobDefinitionStatsResponseBody struct {
	// in:body
	Body []stats.JobStats
}

type jobDefinitionTypeParams struct {
	// in:path
	Type string `json:"type"`
}

// The parameters for finding job-definition by id
type jobDefinitionIDParams struct {
	// in:path
	ID string `json:"id"`
}

// The parameters for updating job-definition concurrency by id
type jobDefinitionConcurrencyParams struct {
	// in:path
	ID string `json:"id"`
	// in:formData
	Concurrency int `json:"concurrency"`
}

// The parameters for job stats
type emptyJobDefinitionParams struct {
}
