package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"plexobject.com/formicary/internal/ant_config"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/artifacts"

	"plexobject.com/formicary/internal/types"
)

// AsyncCommandExecutor - executes command in the executor
type AsyncCommandExecutor func(
	ctx context.Context,
	cmd string,
	helper bool) (stdout []byte, stderr []byte, exitCode int, exitMessage string, err error)

// ArtifactTransfer interface for transferring artifacts
type ArtifactTransfer interface {
	UploadArtifacts(
		ctx context.Context,
		paths []string,
		expiration time.Time) (artifact *types.Artifact, err error)

	CalculateDigest(
		ctx context.Context,
		paths []string) (digest string, err error)

	UploadCache(
		ctx context.Context,
		id string,
		paths []string,
		expiration time.Time) (artifact *types.Artifact, err error)

	DownloadArtifact(
		ctx context.Context,
		extractedDir string,
		id string) (err error)
}

// UploadCacheAndArtifacts uploads artifacts
func UploadCacheAndArtifacts(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	artifactService artifacts.Service,
	execute AsyncCommandExecutor,
	jobWriter executor.TraceWriter,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	traceWriter executor.TraceWriter,
) (artifacts []*types.Artifact, err error) {
	transferService, err := buildArtifactTransferService(
		antCfg,
		artifactService,
		execute,
		jobWriter,
		taskReq,
		taskResp)
	if err != nil {
		return nil, err
	}

	paths, expiration := taskReq.ExecutorOpts.Artifacts.GetPathsAndExpiration(taskResp.Status.Completed())
	artifacts = make([]*types.Artifact, 0)
	if len(paths) > 0 { // taskReq.ExecutorOpts.Method.SupportsDependentArtifacts()
		artifact, err := uploadArtifacts(
			ctx,
			antCfg,
			transferService,
			paths,
			expiration,
			taskReq,
			taskResp,
			traceWriter,
		)
		if err != nil {
			return nil, err
		}
		if artifact != nil {
			artifacts = append(artifacts, artifact)
		}
	}

	// Check cache
	if taskReq.ExecutorOpts.Method.SupportsCache() && taskReq.ExecutorOpts.Cache.Valid() {
		// uploading cache
		artifact := uploadCache(
			ctx,
			antCfg,
			transferService,
			taskReq.ExecutorOpts.Cache.Paths,
			taskReq.ExecutorOpts.Cache.Expiration(),
			taskReq,
			taskResp,
			traceWriter,
		)
		if artifact != nil {
			artifacts = append(artifacts, artifact)
		}
	}

	return
}

func uploadCache(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	transferService ArtifactTransfer,
	paths []string,
	expiration time.Time,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	traceWriter executor.TraceWriter,
) (artifact *types.Artifact) {
	ctx = context.WithValue(ctx, types.HelperContainerKey, true)
	if taskReq.ExecutorOpts.Cache.NewKeyDigest == "" {
		_ = traceWriter.WriteTraceInfo(ctx, "skipping uploading cache because key not found")
		return nil
	}
	artifactID := taskReq.CacheArtifactID(antCfg.Common.S3.Prefix, taskReq.ExecutorOpts.Cache.NewKeyDigest)
	var err error
	if artifact, err = transferService.UploadCache(ctx, artifactID, paths, expiration); err != nil {
		taskResp.AdditionalError(err.Error(), false)
		_ = traceWriter.WriteTraceError(ctx, err.Error())
		return nil
	}

	artifact.Kind = types.ArtifactKindCache
	if err = artifact.Validate(); err != nil {
		taskResp.AdditionalError(fmt.Sprintf("failed to validate artifact %v due to %v", artifact, err), false)
		_ = traceWriter.WriteTraceError(ctx, err.Error())
		return nil
	}
	artifact.Metadata[types.KeysDigest] = taskReq.ExecutorOpts.Cache.NewKeyDigest

	taskResp.AddArtifact(artifact)
	taskResp.AddContext("CachedArtifactKey", taskReq.ExecutorOpts.Cache.NewKeyDigest)
	_ = traceWriter.WriteTraceInfo(ctx,
		fmt.Sprintf("🌟 uploading cache for %v, id %s, key %s",
			paths, artifact.ID, taskReq.ExecutorOpts.Cache.NewKeyDigest))

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component":    "RequestExecutor",
				"AntID":        antCfg.Common.ID,
				"Bucket":       antCfg.Common.S3.Bucket,
				"ID":           artifact.ID,
				"Sha256":       artifact.SHA256,
				"Sha256Len":    len(artifact.SHA256),
				"OldKeyDigest": taskReq.ExecutorOpts.Cache.KeyDigest,
				"KeyDigest":    taskReq.ExecutorOpts.Cache.NewKeyDigest,
				"Size":         artifact.ContentLength,
				"Request":      taskReq,
				"Response":     taskResp,
			}).Debug("adding cache")
	}
	return
}

func uploadArtifacts(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	transferService ArtifactTransfer,
	paths []string,
	expiration time.Time,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	traceWriter executor.TraceWriter,
) (artifact *types.Artifact, err error) {
	ctx = context.WithValue(ctx, types.HelperContainerKey, true)
	if artifact, err = transferService.UploadArtifacts(ctx, paths, expiration); err != nil {
		taskResp.AdditionalError(err.Error(), true)
		return nil, err
	}

	artifact.Kind = types.ArtifactKindTask
	if err = artifact.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate artifact %v due to %w", artifact, err)
	}

	taskResp.AddArtifact(artifact)
	_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 uploaded artifacts for %v size=%d", paths, artifact.ContentLength))

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutor",
				"AntID":     antCfg.Common.ID,
				"Bucket":    antCfg.Common.S3.Bucket,
				"ID":        artifact.ID,
				"Sha256":    artifact.SHA256,
				"Sha256Len": len(artifact.SHA256),
				"Size":      artifact.ContentLength,
				"Request":   taskReq,
				"Response":  taskResp,
			}).Debug("adding artifact")
	}
	return
}

// SetupCacheAndDownloadArtifacts download and unzip all dependent artifacts in helper container
func SetupCacheAndDownloadArtifacts(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	artifactService artifacts.Service,
	execute AsyncCommandExecutor,
	jobWriter executor.TraceWriter,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	traceWriter executor.TraceWriter) (err error) {
	ctx = context.WithValue(ctx, types.HelperContainerKey, true)
	taskResp.Timings.PreScriptFinishedAt = time.Now()
	defer func() {
		taskResp.Timings.DependentArtifactsDownloadedAt = time.Now()
	}()
	// building service to download artifacts
	transferService, err := buildArtifactTransferService(
		antCfg,
		artifactService,
		execute,
		jobWriter,
		taskReq,
		taskResp)
	if err != nil {
		_ = traceWriter.WriteTraceError(ctx, err.Error())
		return err
	}

	// HTTP does not support dependent artifacts
	if taskReq.ExecutorOpts.Method.SupportsDependentArtifacts() &&
		len(taskReq.ExecutorOpts.DependentArtifactIDs) > 0 {
		err = downloadDependentArtifacts(
			ctx,
			taskReq,
			taskResp,
			execute,
			traceWriter,
			transferService)
	}

	// Downloading Cache for npm, yarn, gradle, etc. (only for docker/kubernetes)
	if taskReq.ExecutorOpts.Cache.Valid() && taskReq.ExecutorOpts.Method.SupportsCache() {
		_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 downloading cache %s...",
			taskReq.ExecutorOpts.Cache.String()))
		return downloadCache(
			ctx,
			antCfg,
			taskReq,
			taskResp,
			execute,
			traceWriter,
			transferService)
	} else if len(taskReq.ExecutorOpts.Cache.Paths) > 0 {
		_ = traceWriter.WriteTraceError(ctx, fmt.Sprintf("🌟 skip downloading cache because no key (key_paths) specified for %s",
			taskReq.ExecutorOpts.Cache.String()))
	}
	return nil
}

func downloadCache(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	execute AsyncCommandExecutor,
	traceWriter executor.TraceWriter,
	transferService ArtifactTransfer) (err error) {
	ctx = context.WithValue(ctx, types.HelperContainerKey, true)
	actualDigest := taskReq.ExecutorOpts.Cache.Key
	if actualDigest == "" && len(taskReq.ExecutorOpts.Cache.KeyPaths) > 0 {
		actualDigest, err = transferService.CalculateDigest(ctx, taskReq.ExecutorOpts.Cache.KeyPaths)
		if err != nil {
			taskResp.AdditionalError(err.Error(), false)
			_ = traceWriter.WriteTraceError(ctx, err.Error())
		}
	}

	if actualDigest == "" {
		_ = traceWriter.WriteTraceInfo(ctx, "failed to find cached artifact")
		return nil // ignoring error
	}

	taskReq.ExecutorOpts.Cache.NewKeyDigest = actualDigest
	artifactID := taskReq.CacheArtifactID(antCfg.Common.S3.Prefix, actualDigest)

	// downloading cache zip file
	if err = transferService.DownloadArtifact(ctx, taskReq.ExecutorOpts.CacheDirectory, artifactID); err != nil {
		_ = traceWriter.WriteTraceError(ctx, err.Error())
		return nil // ignoring error
	}

	_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 downloaded cache artifact %s with key %s", artifactID, actualDigest))

	// running on main container
	for _, p := range taskReq.ExecutorOpts.Cache.Paths {
		cmd := fmt.Sprintf("mkdir -p %s && mv %s %s",
			filepath.Dir(p), filepath.Join(taskReq.ExecutorOpts.CacheDirectory, p), p)
		// Copy cache to current working folder
		if _, stderr, _, _, err := execute(ctx, cmd, false); err != nil {
			_ = traceWriter.WriteTraceError(ctx, fmt.Sprintf("failed to extract cache artifact '%s' due to '%v', stderr=%s",
				p, err, string(stderr)))
		} else {
			_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 extracted cached '%s' from artifact %s", p, artifactID))
		}
	}

	return nil
}

func downloadDependentArtifacts(
	ctx context.Context,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	execute AsyncCommandExecutor,
	traceWriter executor.TraceWriter,
	transferService ArtifactTransfer) (err error) {
	ctx = context.WithValue(ctx, types.HelperContainerKey, true)
	extractedDir := fmt.Sprintf("%s/extracted-artifacts", taskReq.ExecutorOpts.ArtifactsDirectory)
	for _, id := range taskReq.ExecutorOpts.DependentArtifactIDs {
		if err = transferService.DownloadArtifact(ctx, extractedDir, id); err != nil {
			taskResp.AdditionalError(err.Error(), true)
			_ = traceWriter.WriteTraceError(ctx, err.Error())
			return err
		}
		_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 downloading dependent artifact %s", id))
	} // downloaded all files

	// TODO verify download/upload
	// Copy all dependent artifacts to current working folder
	cmd := fmt.Sprintf("touch %s/ignore && cp -R %s/* . && find %s | head -10",
		extractedDir, extractedDir, extractedDir)
	if stdout, stderr, _, _, err := execute(
		ctx,
		cmd,
		false); err == nil {
		if taskReq.ExecutorOpts.Debug {
			_ = traceWriter.WriteTraceInfo(ctx, fmt.Sprintf("🌟 extracted dependent artifact %s", stdout))
		}
	} else {
		taskResp.AdditionalError(fmt.Sprintf("failed to extract dependent artifact due to %v, stderr=%s",
			err, string(stderr)), true)
		_ = traceWriter.WriteTraceError(ctx, err.Error())
	}
	return err
}

// UploadConsoleLog upload console log
func UploadConsoleLog(
	ctx context.Context,
	artifactService artifacts.Service,
	container executor.Executor,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
) error {
	// Gathering console log
	if consoleLog, err := container.GetTrace().Finish(); err != nil {
		return err
	} else if artifact, err := artifactService.SaveBytes(
		ctx,
		taskReq.KeyPath(),
		fmt.Sprintf("%s_console.txt", taskReq.TaskType),
		consoleLog); err != nil {
		return err
	} else {
		artifact.Kind = types.ArtifactKindLogs
		artifact.ExpiresAt = time.Now().Add(types.DefaultArtifactsExpirationDuration)
		taskResp.AddArtifact(artifact)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(
				logrus.Fields{
					"Component": "UploadConsoleLog",
					"Request":   taskReq.Key(),
					"Response":  taskResp.Status,
					"UserID":    taskReq.UserID,
					"Container": container,
					"LogSize":   artifact.ContentLength,
				}).Debugf("uploaded console")
		}
	}
	container.GetTrace().Close()
	return nil
}

func buildArtifactTransferService(
	antCfg *ant_config.AntConfig,
	artifactService artifacts.Service,
	execute AsyncCommandExecutor,
	jobWriter executor.TraceWriter,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse) (ArtifactTransfer, error) {
	switch taskReq.ExecutorOpts.Method {
	case types.Shell:
		fallthrough
	case types.HTTPGet:
		fallthrough
	case types.HTTPPostForm:
		fallthrough
	case types.HTTPPostJSON:
		fallthrough
	case types.HTTPPutJSON:
		fallthrough
	case types.HTTPDelete:
		fallthrough
	case types.WebSocket: // TODO this won't work
		return NewArtifactTransferService(
			artifactService,
			execute,
			taskReq,
			taskResp), nil
	case types.Kubernetes:
		fallthrough
	case types.Docker:
		return NewArtifactTransferHelperContainer(
			antCfg,
			execute,
			jobWriter,
			taskReq,
			taskResp), nil
	default:
		return nil, fmt.Errorf("failed to find artifact transfer service for %s",
			taskReq.ExecutorOpts.Method)
	}
}
