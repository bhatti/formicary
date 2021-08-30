package controller

import (
	"context"
	"fmt"
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
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
	webserver.GET("/api/artifacts", ac.queryArtifacts, acl.New(acl.Artifact, acl.Query)).Name = "query_artifacts"
	webserver.GET("/api/artifacts/:id", ac.getArtifact, acl.New(acl.Artifact, acl.View)).Name = "get_artifact"
	webserver.GET("/api/artifacts/:id/download", ac.downloadArtifact, acl.New(acl.Artifact, acl.View)).Name = "download_artifact"
	webserver.POST("/api/artifacts", ac.uploadArtifact, acl.New(acl.Artifact, acl.Upload)).Name = "post_artifact"
	webserver.DELETE("/api/artifacts/:id", ac.deleteArtifact, acl.New(acl.Artifact, acl.Delete)).Name = "delete_artifact"
	return ac
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/artifacts artifacts queryArtifacts
// Queries artifacts by name, task-type, etc.
// responses:
//   200: artifactsQueryResponse
func (ac *ArtifactController) queryArtifacts(c web.WebContext) error {
	params, order, page, pageSize, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	records, total, err := ac.artifactManager.QueryArtifacts(context.Background(), qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(records, total, page, pageSize))
}

// swagger:route POST /api/artifacts artifacts uploadArtifact
// Uploads artifact data from the request body and returns metadata for the uploaded data.
// responses:
//   200: artifactResponse
func (ac *ArtifactController) uploadArtifact(c web.WebContext) error {
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

// swagger:route GET /api/artifacts/{id} artifacts getArtifact
// Retrieves artifact by its id
// responses:
//   200: artifactResponse
func (ac *ArtifactController) getArtifact(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	art, err := ac.artifactManager.GetArtifact(context.Background(), qc, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, art)
}

// swagger:route GET /api/artifacts/{id}/download artifacts downloadArtifact
// Download artifact by its id
// responses:
//   200: byteResponse
func (ac *ArtifactController) downloadArtifact(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	reader, name, contentType, err := ac.artifactManager.DownloadArtifactBySHA256(context.Background(), qc, id)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.Stream(http.StatusOK, contentType, reader)
}

// swagger:route DELETE /api/artifacts/{id} artifacts deleteArtifact
// Deletes artifact by its id
// responses:
//   200: emptyResponse
func (ac *ArtifactController) deleteArtifact(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := ac.artifactManager.DeleteArtifact(context.Background(), qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryArtifacts
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
// swagger:response artifactsQueryResponse
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
// swagger:parameters uploadArtifact
type artifactUploadParams struct {
	// in:body
	Body []byte
}

// swagger:parameters getArtifact deleteArtifact downloadArtifact
// The parameter for id in path
type artifactIDParamsBody struct {
	// in:path
	ID string `json:"id"`
}

// Artifact body
// swagger:response artifactResponse
type artifactResponseBody struct {
	// in:body
	Body types.Artifact
}

// Empty response body
// swagger:response emptyResponse
type emptyResponseBody struct {
}

// String response body
// swagger:response stringResponse
type stringResponseBody struct {
	// in:body
	Body string
}

// Byte Array response body
// swagger:response byteResponse
type byteResponseBody struct {
	// in:body
	Body []byte
}
