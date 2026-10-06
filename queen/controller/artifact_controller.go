package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"

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
	webserver.GET("/api/artifacts", ac.queryArtifacts, acl.NewPermission(acl.Artifact, acl.Query)).Name = "query_artifacts"
	// Static "by-job" segment must be registered before parametric "/:id" routes.
	webserver.GET("/api/artifacts/by-job/:job_id/download/raw", ac.downloadJobRawArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_job_raw_artifact"
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

// Download and render a single file from the most recent artifact ZIP for a given job.
// The required ?file=<path> param selects the file within the zip (e.g. ?file=reports/index.html).
// HTML files are served inline with relative URL rewriting; Markdown is converted to HTML.
// Binary assets (images, CSS) referenced from HTML reports are served as-is.
//
// responses:
//
//	200: byteResponse
func (ac *ArtifactController) downloadJobRawArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job_id")
	filePath := c.QueryParam("file")
	if filePath == "" {
		return fmt.Errorf("query param 'file' is required")
	}
	cleanPath := path.Clean(filePath)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
		return types.NewValidationError("invalid file path")
	}
	reader, _, contentType, err := ac.artifactManager.ExtractFileFromJobArtifact(context.Background(), qc, jobID, "", cleanPath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	reportBase := "/api/artifacts/by-job/" + jobID + "/download/raw"
	return ServeReportFile(c, data, contentType, cleanPath, reportBase, ac.artifactManager)
}

// Download artifact by its id. When ?file=<path> is provided, extracts and
// renders that file from the artifact ZIP inline (HTML with URL rewriting,
// Markdown converted to HTML, other types served raw). Without ?file=, the
// full artifact is served inline when it is a text/HTML type.
//
// responses:
//
//	200: byteResponse
func (ac *ArtifactController) downloadRawArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	filePath := c.QueryParam("file")

	if filePath != "" {
		cleanPath := path.Clean(filePath)
		if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
			return types.NewValidationError("invalid file path")
		}
		reader, _, contentType, err := ac.artifactManager.ExtractFileFromArtifact(context.Background(), qc, id, cleanPath)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }()
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		reportBase := "/api/artifacts/" + id + "/download/raw"
		return ServeReportFile(c, data, contentType, cleanPath, reportBase, ac.artifactManager)
	}

	// No ?file= — serve the full artifact inline (existing behaviour)
	reader, name, contentType, err := ac.artifactManager.DownloadArtifactBySHA256(context.Background(), qc, id)
	if err != nil {
		return err
	}
	lower := strings.ToLower(name)
	isText := strings.Contains(lower, "txt") || strings.Contains(lower, "csv") ||
		strings.Contains(lower, "text") || strings.Contains(lower, "html") ||
		strings.Contains(contentType, "text") || strings.Contains(contentType, "html") ||
		strings.Contains(contentType, "plain")
	if isText {
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

// ServeReportFile serves a file extracted from an artifact ZIP with appropriate
// inline rendering: HTML with URL rewriting, Markdown converted to HTML, binary as-is.
// Exported so the admin dashboard controller can reuse the same logic.
func ServeReportFile(c web.APIContext, data []byte, contentType, cleanPath, reportBase string, am *manager.ArtifactManager) error {
	lower := strings.ToLower(cleanPath)
	// Allow inline <style> blocks (required for report CSS) while keeping scripts blocked.
	csp := "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'none'; object-src 'none'"
	switch {
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		rewritten := am.RewriteHTMLReport(data, reportBase, cleanPath)
		c.Response().Header().Set("Content-Security-Policy", csp)
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.Blob(http.StatusOK, "text/html; charset=utf-8", rewritten)
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"):
		rendered, err := am.RenderMarkdownReport(data, filepath.Base(cleanPath))
		if err != nil {
			return err
		}
		c.Response().Header().Set("Content-Security-Policy", csp)
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.Blob(http.StatusOK, "text/html; charset=utf-8", rendered)
	default:
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		return c.Blob(http.StatusOK, contentType, data)
	}
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
