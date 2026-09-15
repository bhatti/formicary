package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	echo "github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/artifacts"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

func Test_ShouldDownloadFileFromJobArtifactAdmin(t *testing.T) {
	// GIVEN admin artifact controller with a zip artifact whose JobRequestID is stamped.
	// Upload via the manager directly because the admin controller's uploadArtifact handler
	// requires a multipart form that the stub context does not provide.
	mgr := newTestAdminArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactAdminController(mgr, webServer)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>admin-job-report</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	qc := common.NewQueryContext(nil, "")
	artifact, err := mgr.UploadArtifact(context.Background(), qc,
		io.NopCloser(bytes.NewReader(zipBuf.Bytes())), make(map[string]string))
	require.NoError(t, err)

	// Stamp the artifact with a job request ID and task type.
	const jobID = "admin-ctrl-job-001"
	artifact.JobRequestID = jobID
	artifact.TaskType = "audit-prs"
	_, err = mgr.UpdateArtifact(context.Background(), qc, artifact)
	require.NoError(t, err)

	// WHEN downloading by job ID (no task filter)
	rec := httptest.NewRecorder()
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.SetResponse(echo.NewResponse(rec, echo.New()))
	dlCtx.Params["job_id"] = jobID
	dlCtx.Params["file"] = "reports/pr_audit_report.html"
	err = ctrl.downloadJobArtifact(dlCtx)

	// THEN it should stream the HTML file
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

	// THEN it should also stream the HTML file
	require.NoError(t, err)
	require.Contains(t, rec2.Header().Get("Content-Disposition"), "pr_audit_report.html")
}

func Test_ShouldFailDownloadJobArtifactMissingFileParamAdmin(t *testing.T) {
	// GIVEN admin artifact controller
	mgr := newTestAdminArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactAdminController(mgr, webServer)

	// WHEN calling without ?file= param
	dlCtx := web.NewStubContext(&http.Request{URL: &url.URL{}})
	dlCtx.Params["job_id"] = "any-job-id"
	err := ctrl.downloadJobArtifact(dlCtx)

	// THEN it should return an error
	require.Error(t, err)
	require.Contains(t, err.Error(), "'file'")
}

func newTestAdminArtifactManager(serverCfg *config.ServerConfig, t *testing.T) *manager.ArtifactManager {
	t.Helper()
	artifactService, err := artifacts.NewStub(serverCfg.Common.S3)
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	artifactRepository.Clear()
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	mgr, err := manager.NewArtifactManager(serverCfg, logRepository, artifactRepository, artifactService)
	require.NoError(t, err)
	return mgr
}
