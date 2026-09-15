package manager

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
)

func Test_ShouldExpireArtifacts(t *testing.T) {
	// GIVEN artifact-manager

	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)

	mgr := newTestArtifactManager(t, err, serverCfg)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	serverCfg.DefaultArtifactExpiration = time.Millisecond
	for i := 0; i < 10; i++ {
		in := io.NopCloser(strings.NewReader("test"))
		_, err := mgr.UploadArtifact(
			context.Background(),
			qc,
			in,
			make(map[string]string))
		require.NoError(t, err)
	}
	time.Sleep(2 * time.Millisecond)
	// WHEN expiring
	expired, _, err := mgr.ExpireArtifacts(
		context.Background(),
		qc,
		time.Millisecond,
		10000)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 10, expired)
}

func Test_ShouldUploadArtifacts(t *testing.T) {
	// GIVEN artifact-manager
	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	mgr := newTestArtifactManager(t, err, serverCfg)

	// WHEN querying artifacts
	_, total, err := mgr.QueryArtifacts(
		context.Background(),
		qc,
		make(map[string]interface{}),
		0,
		100,
		make([]string, 0))

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// GIVEN uploaded artifact
	in := io.NopCloser(strings.NewReader("test"))
	art, err := mgr.UploadArtifact(
		context.Background(),
		qc,
		in,
		make(map[string]string))
	require.NoError(t, err)

	// WHEN getting artifact
	loaded, err := mgr.GetArtifact(
		context.Background(),
		qc,
		art.ID)

	// THEN it should not fail and return valid artifact
	require.NoError(t, err)
	require.Equal(t, art.ID, loaded.ID)

	// WHEN deleting artifact
	err = mgr.DeleteArtifact(context.Background(), qc, art.ID)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN getting artifact after delete
	_, err = mgr.GetArtifact(
		context.Background(),
		qc,
		art.ID)

	// THEN it should fail
	require.Error(t, err)
}

func Test_ShouldExtractFileFromArtifact(t *testing.T) {
	// GIVEN artifact manager with a zip artifact containing a known file
	serverCfg := config.TestServerConfig()
	require.NoError(t, serverCfg.Validate())
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	mgr := newTestArtifactManager(t, err, serverCfg)

	// build a zip with a single file inside it
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>audit</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	in := io.NopCloser(bytes.NewReader(zipBuf.Bytes()))
	art, err := mgr.UploadArtifact(context.Background(), qc, in, make(map[string]string))
	require.NoError(t, err)

	// WHEN extracting the HTML file
	rc, name, ct, err := mgr.ExtractFileFromArtifact(context.Background(), qc, art.SHA256, "reports/pr_audit_report.html")

	// THEN it should return the file contents with correct metadata
	require.NoError(t, err)
	defer rc.Close()
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "pr_audit_report.html", name)
	require.Contains(t, ct, "text/html")
	require.Contains(t, string(content), "audit")
}

func Test_ShouldFailExtractMissingFile(t *testing.T) {
	// GIVEN artifact manager with a zip artifact
	serverCfg := config.TestServerConfig()
	require.NoError(t, serverCfg.Validate())
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	mgr := newTestArtifactManager(t, err, serverCfg)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	require.NoError(t, zw.Close())

	in := io.NopCloser(bytes.NewReader(zipBuf.Bytes()))
	art, err := mgr.UploadArtifact(context.Background(), qc, in, make(map[string]string))
	require.NoError(t, err)

	// WHEN requesting a file that doesn't exist in the zip
	_, _, _, err = mgr.ExtractFileFromArtifact(context.Background(), qc, art.SHA256, "missing/file.html")

	// THEN it should return a not-found error
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing/file.html")
}

func Test_ShouldExtractFileFromJobArtifact(t *testing.T) {
	// GIVEN artifact manager with a zip artifact whose JobRequestID is known
	serverCfg := config.TestServerConfig()
	require.NoError(t, serverCfg.Validate())
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	mgr := newTestArtifactManager(t, err, serverCfg)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>job-audit</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	in := io.NopCloser(bytes.NewReader(zipBuf.Bytes()))
	art, err := mgr.UploadArtifact(context.Background(), qc, in, make(map[string]string))
	require.NoError(t, err)

	// stamp the artifact with a job request ID (simulates what formicary does post-pod)
	const jobID = "test-job-request-001"
	art.JobRequestID = jobID
	_, err = mgr.UpdateArtifact(context.Background(), qc, art)
	require.NoError(t, err)

	// WHEN extracting by job ID (no task filter)
	rc, name, ct, err := mgr.ExtractFileFromJobArtifact(context.Background(), qc, jobID, "", "reports/pr_audit_report.html")

	// THEN it should return the file with correct metadata
	require.NoError(t, err)
	defer rc.Close()
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "pr_audit_report.html", name)
	require.Contains(t, ct, "text/html")
	require.Contains(t, string(content), "job-audit")
}

func Test_ShouldExtractFileFromJobArtifactWithTaskType(t *testing.T) {
	// GIVEN artifact manager with a zip artifact stamped with job ID and task type
	serverCfg := config.TestServerConfig()
	require.NoError(t, serverCfg.Validate())
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	mgr := newTestArtifactManager(t, err, serverCfg)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("reports/pr_audit_report.html")
	require.NoError(t, err)
	_, err = fw.Write([]byte("<html><body>task-filtered</body></html>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	in := io.NopCloser(bytes.NewReader(zipBuf.Bytes()))
	art, err := mgr.UploadArtifact(context.Background(), qc, in, make(map[string]string))
	require.NoError(t, err)

	const jobID = "test-job-request-002"
	art.JobRequestID = jobID
	art.TaskType = "audit-prs"
	_, err = mgr.UpdateArtifact(context.Background(), qc, art)
	require.NoError(t, err)

	// WHEN extracting by job ID with matching task type
	rc, name, ct, err := mgr.ExtractFileFromJobArtifact(context.Background(), qc, jobID, "audit-prs", "reports/pr_audit_report.html")

	// THEN it should return the file
	require.NoError(t, err)
	defer rc.Close()
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "pr_audit_report.html", name)
	require.Contains(t, ct, "text/html")
	require.Contains(t, string(content), "task-filtered")

	// WHEN extracting with a non-matching task type
	_, _, _, err = mgr.ExtractFileFromJobArtifact(context.Background(), qc, jobID, "wrong-task", "reports/pr_audit_report.html")

	// THEN it should return not-found
	require.Error(t, err)
}

func Test_ShouldFailExtractJobArtifactNotFound(t *testing.T) {
	// GIVEN artifact manager with no artifact for the given job ID
	serverCfg := config.TestServerConfig()
	require.NoError(t, serverCfg.Validate())
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	mgr := newTestArtifactManager(t, err, serverCfg)

	// WHEN requesting a file for a non-existent job
	_, _, _, err = mgr.ExtractFileFromJobArtifact(context.Background(), qc, "nonexistent-job-999", "", "reports/pr_audit_report.html")

	// THEN it should return a not-found error
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent-job-999")
}

func newTestArtifactManager(t *testing.T, err error, serverCfg *config.ServerConfig) *ArtifactManager {
	artifactService, err := artifacts.NewStub(serverCfg.Common.S3)
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)

	mgr, err := NewArtifactManager(
		serverCfg,
		logRepository,
		artifactRepository,
		artifactService)
	require.NoError(t, err)
	return mgr
}
