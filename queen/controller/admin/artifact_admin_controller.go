package admin

import (
	"context"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"

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
	webserver.GET("/dashboard/artifacts/:id", ac.getArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "get_admin_artifact"
	webserver.GET("/dashboard/artifacts/:id/download", ac.downloadArtifact, acl.NewPermission(acl.Artifact, acl.View)).Name = "download_admin_artifact"
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
	reader, name, contentType, err := ac.artifactManager.DownloadArtifactBySHA256(context.Background(), qc, id)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.Stream(http.StatusOK, contentType, reader)
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
