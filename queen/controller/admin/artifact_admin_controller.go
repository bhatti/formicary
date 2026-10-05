package admin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/manager"
)

// ArtifactAdminController structure
type ArtifactAdminController struct {
	artifactManager *manager.ArtifactManager
	webserver       web.Server
}

// NewArtifactAdminController admin dashboard for managing artifacts
func NewArtifactAdminController(
	artifactManager *manager.ArtifactManager,
	webserver web.Server) *ArtifactAdminController {
	ac := &ArtifactAdminController{
		artifactManager: artifactManager,
		webserver:       webserver,
	}
	webserver.GET("/dashboard/artifacts", ac.queryArtifacts, acl.NewPermission(acl.Artifact, acl.Query)).Name = "query_admin_artifacts"
	// Static "by-job" segment must be registered before parametric "/:id" routes.
	webserver.GET("/dashboard/artifacts/by-job/:job_id/download/raw", ac.downloadJobRawArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_admin_job_raw_artifact"
	webserver.GET("/dashboard/artifacts/:id", ac.getArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "get_admin_artifact"
	webserver.GET("/dashboard/artifacts/:id/download", ac.downloadArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_admin_artifact"
	webserver.GET("/dashboard/artifacts/:id/download/raw", ac.downloadRawArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_admin_raw_artifact"
	webserver.POST("/dashboard/artifacts/:id/delete", ac.deleteArtifact, acl.NewPermission(acl.Artifact, acl.Delete)).Name = "delete_admin_artifact"
	webserver.POST("/dashboard/artifacts", ac.uploadArtifact, acl.NewPermission(acl.Artifact, acl.Upload)).Name = "post_admin_artifact"
	return ac
}

// ********************************* HTTP Handlers ***********************************
// uploadArtifact - artifacts
func (ac *ArtifactAdminController) uploadArtifact(c web.APIContext) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["files"]

	res := make([]*common.Artifact, 0)
	params := make(map[string]string)
	for k, v := range c.Request().Header {
		params[k] = v[0]
	}
	for k, v := range c.Request().Form {
		params[k] = v[0]
	}
	qc := web.BuildQueryContext(c)
	for _, file := range files {
		artifact, err := ac.saveArtifact(qc, file, params)
		if err != nil {
			return err
		}
		res = append(res, artifact)
	}

	return c.Redirect(http.StatusFound, "/dashboard/artifacts")
}

func (ac *ArtifactAdminController) downloadArtifact(c web.APIContext) error {
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

// downloadJobRawArtifact serves a file from the most recent artifact ZIP for a job inline.
// Required: ?file=<zip-relative-path>. HTML is served with URL rewriting; Markdown as HTML.
func (ac *ArtifactAdminController) downloadJobRawArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	jobID := c.Param("job_id")
	filePath := c.QueryParam("file")
	if filePath == "" {
		return fmt.Errorf("query param 'file' is required")
	}
	cleanPath := path.Clean(filePath)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
		return common.NewValidationError("invalid file path")
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
	reportBase := "/dashboard/artifacts/by-job/" + jobID + "/download/raw"
	return controller.ServeReportFile(c, data, contentType, cleanPath, reportBase, ac.artifactManager)
}

// downloadRawArtifact serves a file from a specific artifact inline.
// With ?file=<path> it extracts from the ZIP; without it, serves the full artifact
// if it is a text/HTML type.
func (ac *ArtifactAdminController) downloadRawArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	filePath := c.QueryParam("file")

	if filePath != "" {
		cleanPath := path.Clean(filePath)
		if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
			return common.NewValidationError("invalid file path")
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
		reportBase := "/dashboard/artifacts/" + id + "/download/raw"
		return controller.ServeReportFile(c, data, contentType, cleanPath, reportBase, ac.artifactManager)
	}

	// No ?file= — serve full artifact inline (existing behaviour)
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
	return common.NewValidationError(fmt.Sprintf("cannot return artifact %s of content-type %s", name, contentType))
}


func (ac *ArtifactAdminController) getArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	art, err := ac.artifactManager.GetArtifact(context.Background(), qc, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, art)
}

func (ac *ArtifactAdminController) saveArtifact(
	qc *common.QueryContext,
	file *multipart.FileHeader,
	params map[string]string) (*common.Artifact, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer ioutil.NopCloser(src)

	return ac.artifactManager.UploadArtifact(context.Background(), qc, src, params)
}

// queryArtifacts - queries artifact
func (ac *ArtifactAdminController) queryArtifacts(c web.APIContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	qc := web.BuildQueryContext(c)
	records, total, err := ac.artifactManager.QueryArtifacts(context.Background(), qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/artifacts?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    records,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "artifacts/index", res)
}

// deleteArtifact - deletes artifact by id
func (ac *ArtifactAdminController) deleteArtifact(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := ac.artifactManager.DeleteArtifact(context.Background(), qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/artifacts")
}
