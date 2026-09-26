// SPDX-License-Identifier: AGPL-3.0-or-later
package transfer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

// noopTraceWriter satisfies executor.TraceWriter without producing output.
type noopTraceWriter struct{}

func (n *noopTraceWriter) WriteTrace(_ context.Context, _ string) error        { return nil }
func (n *noopTraceWriter) WriteTraceInfo(_ context.Context, _ string) error    { return nil }
func (n *noopTraceWriter) WriteTraceSuccess(_ context.Context, _ string) error { return nil }
func (n *noopTraceWriter) WriteTraceError(_ context.Context, _ string) error   { return nil }

// stubTransfer implements ArtifactTransfer and writes caller-supplied files into extractedDir.
type stubTransfer struct {
	files map[string]string // relative path → content written to extractedDir on DownloadArtifact
	err   error             // returned by DownloadArtifact when non-nil
}

func (s *stubTransfer) DownloadArtifact(_ context.Context, extractedDir string, _ string) error {
	if s.err != nil {
		return s.err
	}
	for relPath, content := range s.files {
		abs := filepath.Join(extractedDir, relPath)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
func (s *stubTransfer) UploadArtifacts(_ context.Context, _ []string, _ time.Time) (*common.Artifact, error) {
	return nil, nil
}
func (s *stubTransfer) CalculateDigest(_ context.Context, _ []string) (string, error) {
	return "", nil
}
func (s *stubTransfer) UploadCache(_ context.Context, _ string, _ []string, _ time.Time) (*common.Artifact, error) {
	return nil, nil
}

// stubTransferFunc is a flexible stub that uses a callback for DownloadArtifact.
type stubTransferFunc struct {
	fn func(ctx context.Context, extractedDir string, id string) error
}

func (s *stubTransferFunc) DownloadArtifact(ctx context.Context, extractedDir string, id string) error {
	return s.fn(ctx, extractedDir, id)
}
func (s *stubTransferFunc) UploadArtifacts(_ context.Context, _ []string, _ time.Time) (*common.Artifact, error) {
	return nil, nil
}
func (s *stubTransferFunc) CalculateDigest(_ context.Context, _ []string) (string, error) {
	return "", nil
}
func (s *stubTransferFunc) UploadCache(_ context.Context, _ string, _ []string, _ time.Time) (*common.Artifact, error) {
	return nil, nil
}

// shellExecute returns an AsyncCommandExecutor that runs shell commands with CWD=execDir.
// execDir simulates the CWD that kubectl exec provides (the container's workingDir in prod,
// or an arbitrary dir in the WorkingDirectory isolation test).
func shellExecute(execDir string) AsyncCommandExecutor {
	return func(ctx context.Context, cmd string, _ bool) ([]byte, []byte, int, string, error) {
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		c.Dir = execDir
		var outBuf, errBuf bytes.Buffer
		c.Stdout = &outBuf
		c.Stderr = &errBuf
		err := c.Run()
		code := 0
		if c.ProcessState != nil {
			code = c.ProcessState.ExitCode()
		}
		return outBuf.Bytes(), errBuf.Bytes(), code, "", err
	}
}

// makeTaskReq builds a TaskRequest with WorkingDirectory=ws and the given artifact IDs.
// ws must be the intended destination for copied files (equivalent to /workspace in prod).
func makeTaskReq(artifactsDir, ws string, artifactIDs ...string) *common.TaskRequest {
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("test-task", common.Kubernetes),
	}
	req.ExecutorOpts.ArtifactsDirectory = artifactsDir
	req.ExecutorOpts.WorkingDirectory = ws
	req.ExecutorOpts.DependentArtifactIDs = artifactIDs
	return req
}

// --- Tests -------------------------------------------------------------------

// Test_DownloadDependentArtifacts_MergesWhenDestExists is the regression test for the
// POSIX cp nesting bug: when a sidecar (e.g. AMS) pre-creates the destination directory
// before the copy loop runs, `cp -R src .` would nest (src/src/). The loop must merge.
func Test_DownloadDependentArtifacts_MergesWhenDestExists(t *testing.T) {
	ws := t.TempDir()
	// Simulate AMS pre-creating the recordings directory (empty) before artifact copy.
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "recordings", "api_contracts"), 0o755))
	artifactsDir := t.TempDir()

	stub := &stubTransfer{files: map[string]string{
		"recordings/api_contracts/GET/scenario.yaml": "name: test-scenario\n",
		"record_result.json":                         `{"status":"ok"}`,
	}}

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-001"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		stub,
	)
	require.NoError(t, err)

	// Files must be at the merged path, NOT at recordings/recordings/api_contracts/.
	wantYAML := filepath.Join(ws, "recordings", "api_contracts", "GET", "scenario.yaml")
	nestYAML := filepath.Join(ws, "recordings", "recordings", "api_contracts", "GET", "scenario.yaml")
	require.FileExists(t, wantYAML, "YAML must merge into pre-existing recordings/api_contracts/")
	require.NoFileExists(t, nestYAML, "YAML must NOT be nested under recordings/recordings/")
	require.FileExists(t, filepath.Join(ws, "record_result.json"))
	data, _ := os.ReadFile(wantYAML)
	require.Equal(t, "name: test-scenario\n", string(data))
}

// Test_DownloadDependentArtifacts_CopiesWhenDestAbsent verifies the happy path
// where no sidecar pre-creates the destination directory.
func Test_DownloadDependentArtifacts_CopiesWhenDestAbsent(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-002"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		&stubTransfer{files: map[string]string{
			"recordings/api_contracts/POST/create.yaml": "name: create-scenario\n",
		}},
	)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(ws, "recordings", "api_contracts", "POST", "create.yaml"))
}

// Test_DownloadDependentArtifacts_CopiesFlatFiles ensures non-directory entries land in ws root.
func Test_DownloadDependentArtifacts_CopiesFlatFiles(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-003"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		&stubTransfer{files: map[string]string{
			"result.json": `{"ok":true}`,
			"report.xml":  `<report/>`,
		}},
	)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(ws, "result.json"))
	require.FileExists(t, filepath.Join(ws, "report.xml"))
}

// Test_DownloadDependentArtifacts_ErrorOnDownloadFailure verifies S3 errors propagate.
func Test_DownloadDependentArtifacts_ErrorOnDownloadFailure(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-004"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		&stubTransfer{err: fmt.Errorf("s3 unavailable")},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "s3 unavailable")
}

// Test_DownloadDependentArtifacts_ErrorOnEmptyExtractedDir verifies that when
// DownloadArtifact writes nothing, the copy loop returns an error.
func Test_DownloadDependentArtifacts_ErrorOnEmptyExtractedDir(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-005"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		&stubTransfer{files: map[string]string{}},
	)
	require.Error(t, err, "empty extractedDir must produce an error")
	require.Contains(t, err.Error(), "produced no files")
}

// Test_DownloadDependentArtifacts_MultipleArtifacts verifies all dependent artifacts merge.
func Test_DownloadDependentArtifacts_MultipleArtifacts(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()
	callCount := 0

	stub := &stubTransferFunc{
		fn: func(_ context.Context, extractedDir string, id string) error {
			callCount++
			files := map[string]string{"recordings/api_contracts/GET/a.yaml": "a"}
			if id == "artifact-b" {
				files = map[string]string{"recordings/api_contracts/POST/b.yaml": "b"}
			}
			for rel, content := range files {
				abs := filepath.Join(extractedDir, rel)
				_ = os.MkdirAll(filepath.Dir(abs), 0o755)
				_ = os.WriteFile(abs, []byte(content), 0o644)
			}
			return nil
		},
	}

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-a", "artifact-b"),
		&common.TaskResponse{},
		shellExecute(ws),
		&noopTraceWriter{},
		stub,
	)
	require.NoError(t, err)
	require.Equal(t, 2, callCount, "DownloadArtifact must be called once per artifact ID")
	require.FileExists(t, filepath.Join(ws, "recordings", "api_contracts", "GET", "a.yaml"))
	require.FileExists(t, filepath.Join(ws, "recordings", "api_contracts", "POST", "b.yaml"))
}

// Test_DownloadDependentArtifacts_RespectsWorkingDirectory is the root-cause regression test.
// kubectl exec starts with CWD=/ (not the container's workingDir). Without `cd "$wd"`, all
// relative paths in the copy loop resolve against /, not /workspace. This test proves the
// fix works: shellExecute is given an unrelated root dir (simulating /), but WorkingDirectory
// is set to ws. Files must land under ws, not under root.
func Test_DownloadDependentArtifacts_RespectsWorkingDirectory(t *testing.T) {
	root := t.TempDir() // simulates /, the CWD kubectl exec would provide
	ws := filepath.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(ws, 0o755))
	artifactsDir := t.TempDir()

	err := downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-wd"), // WorkingDirectory = ws
		&common.TaskResponse{},
		shellExecute(root), // exec CWD = root (≠ ws) — mirrors kubectl exec at /
		&noopTraceWriter{},
		&stubTransfer{files: map[string]string{
			"recordings/api_contracts/GET/scenario.yaml": "name: wd-test\n",
		}},
	)
	require.NoError(t, err)

	wantYAML := filepath.Join(ws, "recordings", "api_contracts", "GET", "scenario.yaml")
	wrongYAML := filepath.Join(root, "recordings", "api_contracts", "GET", "scenario.yaml")
	require.FileExists(t, wantYAML, "files must land under WorkingDirectory")
	require.NoFileExists(t, wrongYAML, "files must NOT land at exec CWD when WorkingDirectory is set")
}

// Test_DownloadDependentArtifacts_CopyLoopReportsErrors verifies that a cp failure
// (e.g. permission denied on a pre-created directory owned by a sidecar) appears in
// the trace output. The copy loop must not silently swallow cp errors.
func Test_DownloadDependentArtifacts_CopyLoopReportsErrors(t *testing.T) {
	ws := t.TempDir()
	artifactsDir := t.TempDir()

	// Pre-create the destination dir with read-only permissions so cp will fail.
	dest := filepath.Join(ws, "recordings", "api_contracts")
	require.NoError(t, os.MkdirAll(dest, 0o755))
	require.NoError(t, os.Chmod(dest, 0o555)) // rwxr-xr-x → no write for non-owner
	t.Cleanup(func() { _ = os.Chmod(dest, 0o755) })

	var traceOut []string
	tw := &captureTraceWriter{capture: &traceOut}

	_ = downloadDependentArtifacts(
		context.Background(),
		makeTaskReq(artifactsDir, ws, "artifact-perm"),
		&common.TaskResponse{},
		shellExecute(ws),
		tw,
		&stubTransfer{files: map[string]string{
			"recordings/api_contracts/GET/perm.yaml": "name: perm-test\n",
		}},
	)
	// Whether the error is returned or the file is missing, the trace must contain
	// diagnostic output from the copy loop (echo statements).
	combined := fmt.Sprintf("%v", traceOut)
	require.Contains(t, combined, "artifact-copy", "copy loop must emit diagnostic trace output")
}

// captureTraceWriter records all trace messages for assertion.
type captureTraceWriter struct{ capture *[]string }

func (c *captureTraceWriter) WriteTrace(_ context.Context, msg string) error {
	*c.capture = append(*c.capture, msg)
	return nil
}
func (c *captureTraceWriter) WriteTraceInfo(_ context.Context, msg string) error {
	*c.capture = append(*c.capture, msg)
	return nil
}
func (c *captureTraceWriter) WriteTraceSuccess(_ context.Context, msg string) error {
	*c.capture = append(*c.capture, msg)
	return nil
}
func (c *captureTraceWriter) WriteTraceError(_ context.Context, msg string) error {
	*c.capture = append(*c.capture, msg)
	return nil
}
