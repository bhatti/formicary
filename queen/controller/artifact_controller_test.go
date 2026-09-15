package controller

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	echo "github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

func Test_InitializeSwaggerStructsForArtifact(t *testing.T) {
	_ = artifactsQueryParamsBody{}
	_ = artifactsQueryResponseBody{}
	_ = artifactUploadParams{}
	_ = artifactResponseBody{}
	_ = artifactIDParamsBody{}
	_ = artifactUploadParams{}
	_ = artifactResponseBody{}
	_ = stringResponseBody{}
	_ = byteResponseBody{}
}

func Test_ShouldQueryArtifacts(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	_ = ctrl.uploadArtifact(ctx)

	// WHEN querying artifacts
	req = &http.Request{URL: &url.URL{}}
	ctx = web.NewStubContext(req)
	err := ctrl.queryArtifacts(ctx)

	// THEN it should not fail and return artifacts
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*types.Artifact)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldUploadAndGetArtifact(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	// WHEN uploading artifact via post-body
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	err := ctrl.uploadArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact := ctx.Result.(*types.Artifact)
	require.NotEqual(t, "", artifact.ID)

	// WHEN getting artifact by id
	ctx.Params["id"] = artifact.ID
	err = ctrl.getArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact = ctx.Result.(*types.Artifact)
}

func Test_ShouldUploadAndDeleteArtifact(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)

	// WHEN uploading artifact via post-body
	err := ctrl.uploadArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact := ctx.Result.(*types.Artifact)
	require.NotEqual(t, "", artifact.ID)

	// WHEN deleting artifact by id
	ctx.Params["id"] = artifact.ID
	err = ctrl.deleteArtifact(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldDownloadFileFromArtifactZip(t *testing.T) {
	// GIVEN artifact controller with a zip artifact containing an HTML report
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>pr audit</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	uploadReq := &http.Request{Body: io.NopCloser(bytes.NewReader(zipBuf.Bytes()))}
	uploadCtx := web.NewStubContext(uploadReq)
	require.NoError(t, ctrl.uploadArtifact(uploadCtx))
	artifact := uploadCtx.Result.(*types.Artifact)

	// WHEN downloading with ?file=reports/pr_audit_report.html
	rec := httptest.NewRecorder()
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.SetResponse(echo.NewResponse(rec, echo.New()))
	dlCtx.Params["id"] = artifact.SHA256
	dlCtx.Params["file"] = "reports/pr_audit_report.html"
	err = ctrl.downloadArtifact(dlCtx)

	// THEN it should stream the HTML file with the correct disposition header
	require.NoError(t, err)
	require.Contains(t, rec.Header().Get("Content-Disposition"), "pr_audit_report.html")
}

func Test_ShouldFailDownloadMissingFileFromArtifact(t *testing.T) {
	// GIVEN artifact controller with a zip artifact
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	var zipBuf bytes.Buffer
	require.NoError(t, zip.NewWriter(&zipBuf).Close())

	uploadReq := &http.Request{Body: io.NopCloser(bytes.NewReader(zipBuf.Bytes()))}
	uploadCtx := web.NewStubContext(uploadReq)
	require.NoError(t, ctrl.uploadArtifact(uploadCtx))
	artifact := uploadCtx.Result.(*types.Artifact)

	// WHEN downloading a file that doesn't exist in the zip
	rec := httptest.NewRecorder()
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.SetResponse(echo.NewResponse(rec, echo.New()))
	dlCtx.Params["id"] = artifact.SHA256
	dlCtx.Params["file"] = "nonexistent/file.html"
	err := ctrl.downloadArtifact(dlCtx)

	// THEN it should return a not-found error
	require.Error(t, err)
}

func Test_ShouldDownloadFileFromJobArtifact(t *testing.T) {
	// GIVEN artifact controller with a zip artifact whose JobRequestID is stamped
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>job-report</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	uploadReq := &http.Request{Body: io.NopCloser(bytes.NewReader(zipBuf.Bytes()))}
	uploadCtx := web.NewStubContext(uploadReq)
	require.NoError(t, ctrl.uploadArtifact(uploadCtx))
	artifact := uploadCtx.Result.(*types.Artifact)

	// Use the same empty QC that web.NewStubContext produces (no logged-in user).
	qc := types.NewQueryContext(nil, "")
	const jobID = "ctrl-job-001"
	artifact.JobRequestID = jobID
	_, err = mgr.UpdateArtifact(context.Background(), qc, artifact)
	require.NoError(t, err)

	// Stamp with task type for task-filtered test
	artifact.TaskType = "audit-prs"
	_, err = mgr.UpdateArtifact(context.Background(), qc, artifact)
	require.NoError(t, err)

	// WHEN downloading by job ID with ?file= (no task filter)
	rec := httptest.NewRecorder()
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.SetResponse(echo.NewResponse(rec, echo.New()))
	dlCtx.Params["job_id"] = jobID
	dlCtx.Params["file"] = "reports/pr_audit_report.html"
	err = ctrl.downloadJobArtifact(dlCtx)

	// THEN it should stream the file
	require.NoError(t, err)
	require.Contains(t, rec.Header().Get("Content-Disposition"), "pr_audit_report.html")

	// WHEN downloading with ?task= filter
	rec2 := httptest.NewRecorder()
	dlCtx2 := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx2.SetResponse(echo.NewResponse(rec2, echo.New()))
	dlCtx2.Params["job_id"] = jobID
	dlCtx2.Params["task"] = "audit-prs"
	dlCtx2.Params["file"] = "reports/pr_audit_report.html"
	err = ctrl.downloadJobArtifact(dlCtx2)

	// THEN it should also stream the file
	require.NoError(t, err)
	require.Contains(t, rec2.Header().Get("Content-Disposition"), "pr_audit_report.html")
}

func Test_ShouldFailDownloadJobArtifactMissingFileParam(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	// WHEN calling without ?file= param
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.Params["job_id"] = "any-job-id"
	err := ctrl.downloadJobArtifact(dlCtx)

	// THEN it should return an error
	require.Error(t, err)
	require.Contains(t, err.Error(), "'file'")
}

func newTestArtifactManager(serverCfg *config.ServerConfig, t *testing.T) *manager.ArtifactManager {
	artifactService, err := artifacts.NewStub(serverCfg.Common.S3)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	artifactRepository, err := repository.NewTestArtifactRepository()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	artifactRepository.Clear()
	logRepository, err := repository.NewTestLogEventRepository()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	art := types.NewArtifact("bucket", ulid.Make().String(), "group", "kind", "101", "sha", 100)
	art.ID = ulid.Make().String()
	_, _ = artifactRepository.Save(art)
	mgr, err := manager.NewArtifactManager(
		serverCfg,
		logRepository,
		artifactRepository,
		artifactService)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	return mgr
}
