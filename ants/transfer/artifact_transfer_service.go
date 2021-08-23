package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"plexobject.com/formicary/internal/artifacts"

	"plexobject.com/formicary/internal/types"
)

// ArtifactTransferService structure
type ArtifactTransferService struct {
	artifactService artifacts.Service
	execute         AsyncCommandExecutor
	taskReq         *types.TaskRequest
	taskResp        *types.TaskResponse
}

// NewArtifactTransferService constructor
func NewArtifactTransferService(
	artifactService artifacts.Service,
	execute AsyncCommandExecutor,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse) ArtifactTransfer {
	return &ArtifactTransferService{
		artifactService: artifactService,
		execute:         execute,
		taskReq:         taskReq,
		taskResp:        taskResp,
	}
}

// UploadCache uploads artifacts
func (t *ArtifactTransferService) UploadCache(
	ctx context.Context,
	id string,
	paths []string,
	expiration time.Time) (artifact *types.Artifact, err error) {
	return t.uploadArtifacts(
		ctx,
		"",
		id,
		fmt.Sprintf("%s_cache.zip", t.taskReq.TaskType),
		paths,
		expiration)
}

// UploadArtifacts uploads artifacts
func (t *ArtifactTransferService) UploadArtifacts(
	ctx context.Context,
	paths []string,
	expiration time.Time) (artifact *types.Artifact, err error) {
	return t.uploadArtifacts(
		ctx,
		t.taskReq.KeyPath(),
		"",
		fmt.Sprintf("%s.zip", t.taskReq.TaskType),
		paths,
		expiration)
}

func (t *ArtifactTransferService) uploadArtifacts(
	ctx context.Context,
	prefix string,
	id string,
	name string,
	paths []string,
	expiration time.Time) (artifact *types.Artifact, err error) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "artifacts.zip")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if err = artifacts.ZipFiles(tmpFile, paths); err != nil {
		return nil, err
	}
	_ = tmpFile.Close()
	artifact = &types.Artifact{
		ID:          id,
		Name:        name,
		ContentType: "application/zip",
		Metadata:    map[string]string{},
		Tags:        map[string]string{},
		ExpiresAt:   expiration,
	}

	if err = t.artifactService.SaveFile(
		ctx,
		prefix,
		artifact,
		tmpFile.Name()); err != nil {
		return nil, err
	}

	return
}

// CalculateDigest calculates digest of artifact paths
func (t *ArtifactTransferService) CalculateDigest(_ context.Context, paths []string) (digest string, err error) {
	if paths == nil || len(paths) == 0 {
		return "", fmt.Errorf("no paths specified for digest")
	}
	hasher := sha256.New()
	for _, p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return "", err
		}
		hasher.Write(data)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// DownloadArtifact downloads dependent artifacts
func (t *ArtifactTransferService) DownloadArtifact(
	ctx context.Context,
	extractedDir string,
	id string) (err error) {
	reader, err := t.artifactService.Get(
		ctx,
		id)
	if err != nil {
		return err
	}
	defer ioutil.NopCloser(reader)
	tmpFile, err := ioutil.TempFile(os.TempDir(), "artifact")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		return err
	}
	_ = tmpFile.Close()
	if err = os.MkdirAll(extractedDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create extracted-dir %s due to %v", extractedDir, err)
	}

	return artifacts.UnzipFile(tmpFile.Name(), extractedDir)
}
