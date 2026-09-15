package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
	"regexp"
)

// ArtifactController structure
type ArtifactController struct {
	artifactManager *manager.ArtifactManager
	webserver       web.Server
}

// NewArtifactController instantiates controller for updating artifacts
func NewArtifactController(
	artifactManager *manager.ArtifactManager,
	webserver web.Server) *ArtifactController {
	ac := &ArtifactController{
		artifactManager: artifactManager,
		webserver:       webserver,
	}
	webserver.GET("/api/artifacts", ac.queryArtifacts, acl.NewPermission(acl.Artifact, acl.Query)).Name = "query_artifacts"
	// Static segment "by-job" must be registered before the parametric "/:id" routes so that
	// Echo's radix-tree router never interprets "by-job" as an artifact SHA256 value.
	webserver.GET("/api/artifacts/by-job/:job_id/download", ac.downloadJobArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_job_artifact"
	webserver.GET("/api/artifacts/:id", ac.getArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "get_artifact"
	webserver.GET("/api/artifacts/:id/download", ac.downloadArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_artifact"
	webserver.GET("/api/artifacts/:id/download/raw", ac.downloadRawArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_raw_artifact"
	webserver.POST("/api/artifacts", ac.uploadArtifact, acl.NewPermission(acl.Artifact, acl.Upload)).Name = "post_artifact"
	webserver.DELETE("/api/artifacts/:id", ac.deleteArtifact, acl.NewPermission(acl.Artifact, acl.Delete)).Name = "delete_artifact"
	return ac
}

// ********************************* HTTP Handlers ***********************************

// Queries artifacts by name, task-type, etc.
// responses:
//   200: artifactsQueryResponse
func (ac *ArtifactController) queryArtifacts(c web.APIContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	records, total, err := ac.artifactManager.QueryArtifacts(context.Background(), qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(records, total, page, pageSize))
}

// Uploads artifact data from the request body and returns metadata for the uploaded data.
// responses:
//   200: artifactResponse
func (ac *ArtifactController) uploadArtifact(c web.APIContext) error {
	params := make(map[string]string)
	for k, v := range c.Request().Header {
		params[k] = v[0]
	}
	for k, v := range c.Request().Form {
		params[k] = v[0]
	}
	qc := web.BuildQueryContext(c)
	artifact, err := ac.artifactManager.UploadArtifact(context.Background(), qc, c.Request().Body, params)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, artifact)
}

// Retrieves artifact by its id
// responses:
//   200: artifactResponse
func (ac *ArtifactController) getArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	art, err := ac.artifactManager.GetArtifact(context.Background(), qc, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, art)
}

// Download artifact by its id. When the optional ?file=<path> query param is provided,
// extracts and returns just that file from the artifact zip.
// responses:
//   200: byteResponse
func (ac *ArtifactController) downloadArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	filePath := c.QueryParam("file")
	var reader io.ReadCloser
	var name, contentType string
	var err error
	if filePath != "" {
		reader, name, contentType, err = ac.artifactManager.ExtractFileFromArtifact(context.Background(), qc, id, filePath)
	} else {
		reader, name, contentType, err = ac.artifactManager.DownloadArtifactBySHA256(context.Background(), qc, id)
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.Stream(http.StatusOK, contentType, reader)
}

// Download a single file from the artifact zip for a given job request ID.
// The ?file=<path> query param is required (e.g. ?file=reports/pr_audit_report.html).
// Use this when the artifact SHA256 is not yet known at request time (e.g. from inside
// the job pod, where artifacts are uploaded after the pod exits).
//
// responses:
//
//	200: byteResponse
func (ac *ArtifactController) downloadJobArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job_id")
	filePath := c.QueryParam("file")
	if filePath == "" {
		return fmt.Errorf("query param 'file' is required")
	}
	reader, name, contentType, err := ac.artifactManager.ExtractFileFromJobArtifact(context.Background(), qc, jobID, filePath)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.Stream(http.StatusOK, contentType, reader)
}

// Download artifact by its id
// responses:
//   200: byteResponse
func (ac *ArtifactController) downloadRawArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	reader, name, contentType, err := ac.artifactManager.DownloadArtifactBySHA256(context.Background(), qc, id)
	if err != nil {
		return err
	}
	matchedName, _ := regexp.Match("(txt|csv|text|html)", []byte(name))
	matchedContent, _ := regexp.Match("(txt|csv|text|html|plain)", []byte(contentType))
	if matchedName || matchedContent {
		buf := new(bytes.Buffer)
		if _, err = buf.ReadFrom(reader); err != nil {
			return err
		}
		contentType = http.DetectContentType(buf.Bytes())
		c.Response().Header().Set("Content-Type", contentType)
		return c.Blob(http.StatusOK, contentType, buf.Bytes())
	}
	return types.NewValidationError(fmt.Sprintf("cannot return artifact %s of content-type %s", name, contentType))
}

// Deletes artifact by its id
// responses:
//   200: emptyResponse
func (ac *ArtifactController) deleteArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := ac.artifactManager.DeleteArtifact(context.Background(), qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// The params for querying artifacts
type artifactsQueryParamsBody struct {
	// in:query
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Order    string `json:"order"`
	// Name - name of artifact for display
	Name string `json:"name"`
	// Group of artifact
	Group string `json:"group"`
	// Kind of artifact
	Kind string `json:"kind"`
	// JobRequestID refers to request-id being processed
	JobRequestID uint64 `json:"job_request_id"`
	// TaskType defines type of task
	TaskType string `yaml:"task_type" json:"task_type"`
	// SHA256 refers hash of the contents
	SHA256 string `json:"sha256"`
	// ContentType refers to content-type of artifact
	ContentType string `json:"content_type"`
	// ContentLength refers to content-length of artifact
	ContentLength int64 `json:"content_length"`
}

// Paginated results of artifacts matching query
type artifactsQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []types.Artifact
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// Artifact body for upload
type artifactUploadParams struct {
	// in:body
	Body []byte
}

// The parameter for id in path
type artifactIDParamsBody struct {
	// in:path
	ID string `json:"id"`
}

// Artifact body
type artifactResponseBody struct {
	// in:body
	Body types.Artifact
}

// Empty response body
type emptyResponseBody struct {
}

// String response body
type stringResponseBody struct {
	// in:body
	Body string
}

// Byte Array response body
type byteResponseBody struct {
	// in:body
	Body []byte
}
