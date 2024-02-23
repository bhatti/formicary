package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/ants/executor"

	"plexobject.com/formicary/ants/config"

	"plexobject.com/formicary/internal/types"
)

// ArtifactTransferHelperContainer structure
type ArtifactTransferHelperContainer struct {
	antCfg    *config.AntConfig
	execute   AsyncCommandExecutor
	jobWriter executor.TraceWriter
	taskReq   *types.TaskRequest
	taskResp  *types.TaskResponse
}

// NewArtifactTransferHelperContainer constructor
func NewArtifactTransferHelperContainer(
	antCfg *config.AntConfig,
	execute AsyncCommandExecutor,
	jobWriter executor.TraceWriter,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse) ArtifactTransfer {
	return &ArtifactTransferHelperContainer{
		antCfg:    antCfg,
		execute:   execute,
		jobWriter: jobWriter,
		taskReq:   taskReq,
		taskResp:  taskResp,
	}
}

// UploadCache uploads artifacts
func (t *ArtifactTransferHelperContainer) UploadCache(
	ctx context.Context,
	id string,
	paths []string,
	expiration time.Time) (artifact *types.Artifact, err error) {
	cacheDir := t.taskReq.ExecutorOpts.CacheDirectory
	name := fmt.Sprintf("%s_cache.zip", t.taskReq.TaskType)
	return t.uploadArtifacts(ctx, id, name, paths, expiration, cacheDir)
}

// CalculateDigest calculates digest of artifacts
func (t *ArtifactTransferHelperContainer) CalculateDigest(ctx context.Context, paths []string) (actualDigest string, err error) {
	if paths == nil || len(paths) == 0 {
		return "", fmt.Errorf("no paths specified for digest")
	}
	// Copy keys to helper artifacts directory so that we can run digest from helper container
	joinedPaths := strings.Join(paths, " ")
	cmd := fmt.Sprintf("cp %s %s", joinedPaths, t.taskReq.ExecutorOpts.CacheDirectory)
	if _, stderr, _, _, err := t.execute(ctx, cmd, false); err != nil {
		return "", fmt.Errorf("failed to copy key files for calculating cache key due to %w, stderr=%s",
			err, string(stderr))
	}

	targetPaths := utils.ReplaceDirPath(paths, t.taskReq.ExecutorOpts.CacheDirectory)
	targetPathsJoin := strings.Join(targetPaths, " ")

	// calculating digest on helper container
	cmd = fmt.Sprintf("cat %s|sha256sum|awk '{print $1}'", targetPathsJoin)
	var stdout, stderr []byte
	if stdout, stderr, _, _, err = t.execute(ctx, cmd, true); err == nil {
		actualDigest = strings.TrimSpace(string(stdout))
		return actualDigest, nil
	}
	return "", fmt.Errorf("failed to calculate cache key due to %w, stderr=%s",
		err, string(stderr))
}

// UploadArtifacts uploads artifacts
func (t *ArtifactTransferHelperContainer) UploadArtifacts(
	ctx context.Context,
	paths []string,
	expiration time.Time) (artifact *types.Artifact, err error) {
	artifactsDir := t.taskReq.ExecutorOpts.ArtifactsDirectory
	id := fmt.Sprintf("%s%s.zip",
		utils.NormalizePrefix(t.antCfg.Common.S3.Prefix), t.taskReq.KeyPath())
	name := fmt.Sprintf("%s.zip", t.taskReq.TaskType)
	return t.uploadArtifacts(ctx, id, name, paths, expiration, artifactsDir)
}

// docker run -it --rm -v /home/shahzad:/download --entrypoint /bin/bash amazon/aws-cli
func (t *ArtifactTransferHelperContainer) uploadArtifacts(
	ctx context.Context,
	id string,
	name string,
	paths []string,
	expiration time.Time,
	dir string) (artifact *types.Artifact, err error) {
	var names strings.Builder

	for _, p := range paths {
		cmd := fmt.Sprintf("mv %s %s", p, dir)
		if _, stderr, _, _, err := t.execute(
			ctx,
			cmd,
			false); err != nil {
			_ = t.jobWriter.WriteTrace(ctx,
				fmt.Sprintf("⛔ failed to copy artifact %s due to %v, stderr=%s",
					p, err, string(stderr)))
		} else {
			names.WriteString(p + " ")
		}
	}

	// TODO verify download/upload
	// zip all artifacts and copy them to S3
	zipFile := filepath.Join(dir, name)
	zipCmd := fmt.Sprintf("cd %s && ls -l && python /usr/lib64/python2.7/zipfile.py -c %s %s && python /usr/lib64/python2.7/zipfile.py -l %s",
		dir, zipFile, names.String(), zipFile)

	var stdout, stderr []byte
	if stdout, stderr, _, _, err = t.execute(
		ctx,
		zipCmd,
		true); err != nil {
		return nil, fmt.Errorf("failed to zip artifacts %s [%s] due to %w, stderr=%s",
			zipFile, zipCmd, err, string(stderr))
	}

	shaCmd := fmt.Sprintf("sha256sum %s && ls -l %s && python /usr/lib64/python2.7/zipfile.py -l %s|head -10",
		zipFile, zipFile, zipFile)

	if stdout, stderr, _, _, err = t.execute(
		ctx,
		shaCmd,
		true); err != nil {
		return nil, fmt.Errorf("failed to zip artifacts %s [%s] due to %w, stderr=%s",
			zipFile, shaCmd, err, string(stderr))
	}

	parts := strings.Split(string(stdout), " ")
	if len(parts) <= 6 {
		return nil, fmt.Errorf("unexpected parts found in upload %v",
			parts)
	}

	sha256 := parts[0]
	size := 0

	if len(parts) > 6 {
		size, err = strconv.Atoi(parts[6])
		if err != nil {
			return nil, fmt.Errorf("failed to parse parts %v due to %w", parts, err)
		}
	}

	if len(sha256) > 64 {
		diff := len(sha256) - 64
		sha256 = sha256[diff:]
	}

	uploadCmd := fmt.Sprintf("ls -l %s && aws s3 --endpoint-url $AWS_URL cp %s s3://%s/%s",
		zipFile, zipFile, t.antCfg.Common.S3.Bucket, id)
	if expiration.Unix() > time.Now().Unix() {
		uploadCmd += fmt.Sprintf(" --expires %s", expiration.Format(time.RFC3339))
	}

	// upload artifact
	if _, _, _, _, err := t.execute(
		ctx,
		uploadCmd,
		true); err != nil {
		return nil, fmt.Errorf("failed to upload %s due to %w, stderr=%s",
			id, err, string(stderr))
	}

	// Add artifacts to response
	artifact = &types.Artifact{
		Name:          name,
		Bucket:        t.antCfg.Common.S3.Bucket,
		ID:            id,
		SHA256:        sha256,
		ContentLength: int64(size),
		ContentType:   "application/zip",
		Metadata:      make(map[string]string),
		Tags:          make(map[string]string),
		ExpiresAt:     expiration,
	}

	return artifact, nil
}

// DownloadArtifact downloads dependent artifacts
func (t *ArtifactTransferHelperContainer) DownloadArtifact(
	ctx context.Context,
	extractedDir string,
	id string) (err error) {
	// TODO verify download/upload
	cmds := []string{
		fmt.Sprintf("mkdir -p %s && aws s3 --endpoint-url $AWS_URL cp s3://%s/%s all_artifacts.zip && ls -l all_artifacts.zip",
			extractedDir, t.antCfg.Common.S3.Bucket, id),
		fmt.Sprintf("python /usr/lib64/python2.7/zipfile.py -e all_artifacts.zip %s", extractedDir),
		fmt.Sprintf("rm all_artifacts.zip && find %s | head -10", extractedDir),
	}

	for _, cmd := range cmds {
		if _, stderr, _, _, err := t.execute(
			ctx,
			cmd,
			true); err != nil {
			return fmt.Errorf("failed to download dependent artifact '%s' due to %w, stderr=%s",
				id, err, string(stderr))
		}
	}
	return nil
}
