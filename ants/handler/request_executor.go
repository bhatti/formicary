package handler

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/ants/logs"

	cutils "plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/web"

	"plexobject.com/formicary/ants/transfer"

	"plexobject.com/formicary/internal/events"

	"plexobject.com/formicary/ants/executor/utils"

	"plexobject.com/formicary/internal/artifacts"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

const tmpArtifacts = "temp_artifacts"
const httpPrefix = "http"
const httpsPrefix = "https"

// RequestExecutor interface
type RequestExecutor interface {
	Execute(
		ctx context.Context,
		taskReq *types.TaskRequest) (taskResp *types.TaskResponse)
}

// RequestExecutorImpl structure
type RequestExecutorImpl struct {
	antCfg          *config.AntConfig
	queueClient     queue.Client
	webClient       web.HTTPClient
	artifactService artifacts.Service
}

// NewRequestExecutor constructor
func NewRequestExecutor(
	antCfg *config.AntConfig,
	queueClient queue.Client,
	webClient web.HTTPClient,
	artifactService artifacts.Service) *RequestExecutorImpl {
	return &RequestExecutorImpl{
		antCfg:          antCfg,
		queueClient:     queueClient,
		webClient:       webClient,
		artifactService: artifactService,
	}
}

// Execute executes a task request
func (re *RequestExecutorImpl) Execute(
	ctx context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse) {
	// Create task-response
	taskResp = types.NewTaskResponse(taskReq)

	taskResp.Timings.ReceivedAt = taskReq.StartedAt

	// prepare executor options and build container based on request method
	container, err := re.preProcess(ctx, taskReq)
	if err != nil {
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = err.Error()
		return
	}
	taskResp.Timings.PodStartedAt = time.Now()
	cmdExecutor := func(
		ctx context.Context,
		cmd string,
		helper bool) (stdout []byte, stderr []byte, exitCode int, exitMessage string, err error) {
		return re.asyncExecuteCommand(
			ctx,
			container,
			cmd,
			taskReq.Variables,
			helper)
	}

	if len(taskReq.BeforeScript) > 0 {
		_ = container.WriteTraceInfo(fmt.Sprintf("\U0001F9F0 executing pre-script for task '%s' of job '%s' with request-id '%d' starting...",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID))
	}
	// prescript
	if err := re.execute(
		ctx,
		container,
		taskReq,
		taskResp,
		taskReq.BeforeScript,
		true); err != nil {
		// Execute all commands in script
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = err.Error()
	} else if err := transfer.SetupCacheAndDownloadArtifacts(
		ctx,
		re.antCfg,
		re.artifactService,
		cmdExecutor,
		container,
		taskReq,
		taskResp,
		container); err != nil {
		// Download dependent artifacts first if needed
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = err.Error()
	} else if err := re.execute(
		ctx,
		container,
		taskReq,
		taskResp,
		taskReq.Script,
		true); err != nil {
		// Execute all commands in script
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = err.Error()
	} else {
		taskResp.Status = types.COMPLETED
	}

	taskResp.Timings.ScriptFinishedAt = time.Now()
	if len(taskReq.AfterScript) > 0 {
		_ = container.WriteTraceInfo(fmt.Sprintf("🗳️ executing post-script for task '%s' of job '%s' with request-id '%d' starting...",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID))
	}
	// Executing post-script regardless the task fails or succeeds
	if err := re.execute(
		ctx,
		container,
		taskReq,
		taskResp,
		taskReq.AfterScript,
		false); err != nil {
		taskResp.AdditionalError(err.Error(), false)
	}
	taskResp.Timings.PostScriptFinishedAt = time.Now()
	// copy applied limits
	taskResp.AppliedCost = taskReq.ExecutorOpts.AppliedCost

	// let post-process complete with fresh context in case it's cancelled
	re.postProcess(context.Background(), taskReq, taskResp, container)
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// execute each command in script
func (re *RequestExecutorImpl) execute(
	ctx context.Context,
	container executor.Executor,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	cmds []string,
	failOnError bool) (err error) {
	// timeout only applies to execution of script
	var cancel context.CancelFunc
	if taskReq.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, taskReq.Timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	doExecute := func(cmd string) ([]byte, error) {
		stdout, stderr, exitCode, exitMessage, err := re.asyncExecuteCommand(
			ctx,
			container,
			cmd,
			taskReq.Variables,
			false)
		taskResp.ExitCode = strconv.Itoa(exitCode)
		taskResp.ExitMessage = exitMessage
		if len(stderr) > 0 {
			// it's already logged on console
			//container.WriteTraceError(string(stderr))
		}
		return stdout, err
	}

	var lastError error
	for i, cmd := range cmds {
		stdout, err := doExecute(cmd)
		if err != nil && failOnError {
			return err
		} else if err != nil {
			lastError = err
			_ = container.WriteTraceError(err.Error())
		}

		// Note: this only works for SHELL/HTTP but containers will need to use it explicitly
		if taskReq.ExecutorOpts.Method.SupportsCaptureStdout() {
			path, err := addArtifactToPath(taskReq, i, cmd, stdout)
			if err != nil && failOnError {
				return err
			} else if err != nil {
				lastError = err
				_ = container.WriteTraceError(err.Error())
			}
			if path != "" {
				taskReq.ExecutorOpts.Artifacts.Paths = append(taskReq.ExecutorOpts.Artifacts.Paths, path)
			}
		}
	}

	if lastError != nil {
		return lastError
	}
	return ctx.Err()
}

func addArtifactToPath(taskReq *types.TaskRequest, i int, cmd string, stdout []byte) (string, error) {
	if len(stdout) == 0 {
		return "", nil
	}
	tmpFile, err := os.Create(fmt.Sprintf("%d_%s_%d.stdout",
		taskReq.JobRequestID, cutils.MakeDNS1123Compatible(taskReq.TaskType), i))
	if err != nil {
		return "", err
	}
	if _, err = tmpFile.Write(stdout); err != nil {
		return "", fmt.Errorf("failed to write output for %s due to %v", cmd, err)
	}
	ioutil.NopCloser(tmpFile)
	return tmpFile.Name(), nil
}

func (re *RequestExecutorImpl) asyncExecuteCommand(
	ctx context.Context,
	container executor.Executor,
	cmd string,
	variables map[string]interface{},
	helper bool) (stdout []byte, stderr []byte, exitCode int, exitMessage string, err error) {
	var runner executor.CommandRunner
	if helper {
		runner, err = container.AsyncHelperExecute(ctx, cmd, variables)
	} else {
		runner, err = container.AsyncExecute(ctx, cmd, variables)
	}
	if err != nil {
		if runner != nil {
			exitCode = runner.GetExitCode()
			exitMessage = runner.GetExitMessage()
		}
		return
	}

	stdout, stderr, err = runner.Await(ctx)
	exitCode = runner.GetExitCode()
	exitMessage = runner.GetExitMessage()
	return
}

// save ant related metadata in context for debugging
func (re *RequestExecutorImpl) preProcess(
	ctx context.Context,
	taskReq *types.TaskRequest) (container executor.Executor, err error) {
	//taskReq.ExecutorOpts.Debug = true

	taskReq.SecretConfigs = append(taskReq.SecretConfigs,
		"AWS_ENDPOINT", "AWS_ACCESS_KEY_ID", "AWS_URL",
		"AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION",
		re.antCfg.S3.Endpoint, re.antCfg.S3.AccessKeyID,
		re.antCfg.S3.SecretAccessKey, re.antCfg.S3.Region)

	logStreamer, err := logs.NewLogStreamer(
		ctx,
		re.antCfg,
		taskReq,
		re.queueClient,
	)
	if err != nil {
		return nil, err
	}
	// Add helper container if artifacts needed

	taskReq.ExecutorOpts.PodLabels[types.AntID] = re.antCfg.ID

	// Add variables as environment variables
	for k, v := range taskReq.Variables {
		taskReq.ExecutorOpts.Environment[k] = fmt.Sprintf("%v", v)
	}

	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	if re.antCfg.S3.UseSSL {
		taskReq.ExecutorOpts.Environment["AWS_URL"] = fmt.Sprintf("%s://%s", httpsPrefix, re.antCfg.S3.Endpoint)
	} else {
		taskReq.ExecutorOpts.Environment["AWS_URL"] = fmt.Sprintf("%s://%s", httpPrefix, re.antCfg.S3.Endpoint)
	}
	taskReq.ExecutorOpts.Environment["AWS_ENDPOINT"] = re.antCfg.S3.Endpoint
	taskReq.ExecutorOpts.Environment["AWS_ACCESS_KEY_ID"] = re.antCfg.S3.AccessKeyID
	taskReq.ExecutorOpts.Environment["AWS_SECRET_ACCESS_KEY"] = re.antCfg.S3.SecretAccessKey
	taskReq.ExecutorOpts.Environment["AWS_DEFAULT_REGION"] = re.antCfg.S3.Region

	if taskReq.ExecutorOpts.Method == types.Kubernetes {
		taskReq.ExecutorOpts.HelperContainer.Image = re.antCfg.Kubernetes.HelperImage
		taskReq.ExecutorOpts.MainContainer.AddEmptyKubernetesVolume(
			"artifacts", taskReq.ExecutorOpts.ArtifactsDirectory)
		taskReq.ExecutorOpts.HelperContainer.AddEmptyKubernetesVolume(
			"artifacts", taskReq.ExecutorOpts.ArtifactsDirectory)
		taskReq.ExecutorOpts.MainContainer.AddEmptyKubernetesVolume(
			"cache", taskReq.ExecutorOpts.CacheDirectory)
		taskReq.ExecutorOpts.HelperContainer.AddEmptyKubernetesVolume(
			"cache", taskReq.ExecutorOpts.CacheDirectory)
		for _, s := range taskReq.ExecutorOpts.Services {
			s.AddEmptyKubernetesVolume(
				"artifacts", taskReq.ExecutorOpts.ArtifactsDirectory)
			s.AddEmptyKubernetesVolume(
				"cache", taskReq.ExecutorOpts.CacheDirectory)
		}
	} else if taskReq.ExecutorOpts.Method == types.Docker {
		taskReq.ExecutorOpts.HelperContainer.Image = re.antCfg.Docker.HelperImage
		//artDir, err := ioutil.TempDir(os.TempDir(), tmpArtifacts)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to create artDir directory due to %s", err.Error())
		//}
		//if _, err := os.Stat(artDir); os.IsNotExist(err) {
		//	_ = os.MkdirAll(artDir, os.ModePerm)
		//}
		taskReq.ExecutorOpts.MainContainer.GetDockerVolumeNames()[fmt.Sprintf("%s-artifacts", taskReq.ExecutorOpts.Name)] = taskReq.ExecutorOpts.ArtifactsDirectory
		taskReq.ExecutorOpts.MainContainer.GetDockerVolumeNames()[fmt.Sprintf("%s-cache", taskReq.ExecutorOpts.Name)] = taskReq.ExecutorOpts.CacheDirectory

		//taskReq.ExecutorOpts.MainContainer.VolumesFrom = []string{
		//	taskReq.ExecutorOpts.ArtifactsDirectory,
		//}
	}
	if container, err = utils.BuildExecutor(
		ctx,
		re.antCfg,
		logStreamer,
		re.webClient,
		taskReq.ExecutorOpts); err != nil {
		return
	}

	if sendErr := sendContainerEvent(
		ctx,
		re.antCfg,
		re.queueClient,
		taskReq.UserID,
		taskReq.ExecutorOpts.Method,
		types.STARTED,
		container); sendErr != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.ID,
				"Container": container,
				"Error":     sendErr,
			}).Warnf("failed to send lifecycle event container")
	}
	_ = container.WriteTraceInfo(fmt.Sprintf("🚀 task '%s' of job '%s' with request-id '%d' starting...",
		taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID))
	if taskReq.ExecutorOpts.Debug {
		_ = container.WriteTraceInfo(fmt.Sprintf("env variables: %v", taskReq.ExecutorOpts.Environment))
	}

	return
}

func (re *RequestExecutorImpl) postProcess(
	ctx context.Context,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	container executor.Executor) {
	// upload artifacts unless job is cancelled
	if !taskReq.Cancelled {
		cmdExecutor := func(
			ctx context.Context,
			cmd string,
			helper bool) (stdout []byte, stderr []byte, exitCode int, exitMessage string, err error) {
			return re.asyncExecuteCommand(
				ctx,
				container,
				cmd,
				taskReq.Variables,
				helper)
		}

		// upload artifacts and cache if needed
		if uploadedArtifacts, err := transfer.UploadCacheAndArtifacts(
			ctx,
			re.antCfg,
			re.artifactService,
			cmdExecutor,
			container,
			taskReq,
			taskResp,
			container); err != nil {
			re.additionalError(
				container,
				taskReq,
				taskResp,
				fmt.Errorf("failed to upload artifacts due to %v",
					err), true)
		} else if len(uploadedArtifacts) > 0 {
			for _, artifact := range uploadedArtifacts {
				_ = container.WriteTraceInfo(
					fmt.Sprintf("🌟 uploaded artifact Bucket=%s ID=%s SHA256=%s Size=%d",
						re.antCfg.S3.Bucket, artifact.ID, artifact.SHA256, artifact.ContentLength))
			}
		}
	}
	taskResp.Timings.ArtifactsUploadedAt = time.Now()
	// Stopping container
	now := time.Now()
	if err := container.Stop(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.ID,
				"Request":   taskReq,
				"Response":  taskResp,
				"UserID":    taskReq.UserID,
				"Container": container,
				"Elapsed":   time.Since(now).String(),
				"Error":     err,
			}).Warnf("🛑 failed to stop container")
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.ID,
				"Request":   taskReq,
				"Response":  taskResp,
				"UserID":    taskReq.UserID,
				"Container": container,
				"Elapsed":   time.Since(now).String(),
			}).Info("🛑 stopped container")
	}

	taskResp.Timings.PodShutdownAt = time.Now()
	removeTemporaryFiles(taskReq)

	elapsed := time.Now().Sub(taskReq.StartedAt).String()
	if taskResp.Status.Failed() {
		_ = container.WriteTraceError(fmt.Sprintf("🏄 task '%s' of job '%s' with request-id '%d' failed Error=%s Duration=%s",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskResp.ErrorMessage, elapsed))
	} else {
		_ = container.WriteTraceSuccess(fmt.Sprintf("🏄 task '%s' of job '%s' with request-id '%d' completed Duration=%s",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, elapsed))
	}

	_ = container.WriteTraceSuccess(fmt.Sprintf("⏱ task '%s' of job '%s' with request-id '%d' stats: %s",
		taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskResp.Timings.String()))

	// upload console
	if !taskReq.Cancelled {
		if err := transfer.UploadConsoleLog(
			ctx,
			re.artifactService,
			container,
			taskReq,
			taskResp); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "RequestExecutorImpl",
					"AntID":     re.antCfg.ID,
					"Request":   taskReq,
					"Response":  taskResp,
					"UserID":    taskReq.UserID,
					"Container": container,
					"Error":     err,
				}).Warn("failed to upload console")
		}
	}
	// update context with executor details
	re.updateResponseContext(taskReq, taskResp, container)

	if sendErr := sendContainerEvent(
		ctx,
		re.antCfg,
		re.queueClient,
		taskReq.UserID,
		taskReq.ExecutorOpts.Method,
		types.COMPLETED,
		container); sendErr != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutor",
				"AntID":     re.antCfg.ID,
				"Container": container,
				"Error":     sendErr,
			}).Warnf("failed to send stop lifecycle event container by request-executor")
	}

}

// remove any temporary files created if using SHELL or HTTP
func removeTemporaryFiles(taskReq *types.TaskRequest) {
	if taskReq.ExecutorOpts.Method == types.Docker {
		for _, m := range taskReq.ExecutorOpts.MainContainer.GetDockerMounts() {
			if strings.Contains(m.Source, tmpArtifacts) {
				err := os.RemoveAll(m.Source)
				logrus.WithFields(logrus.Fields{
					"Component": "RequestExecutorImpl",
					"File":      m.Source,
					"Error":     err,
				}).Warn("removing temporary file")
			}
		}
	}

	// remove any output files created
	for _, p := range taskReq.ExecutorOpts.Artifacts.Paths {
		if err := os.Remove(p); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "RequestExecutorImpl",
				"File":      p,
				"Error":     err,
			}).Debugf("failed to remove local artifact path")
		}
	}
	// remove any other files with job-request prefix
	if files, err := filepath.Glob(fmt.Sprintf("%d_*", taskReq.JobRequestID)); err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}
}

// save ant related metadata in context for debugging
func (re *RequestExecutorImpl) updateResponseContext(
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	container executor.Executor) {
	taskResp.AntID = re.antCfg.ID
	taskResp.Host, _ = os.Hostname()
	taskResp.AddContext("Image", taskReq.ExecutorOpts.MainContainer.Image)
	taskResp.AddContext("HelperImage", taskReq.ExecutorOpts.HelperContainer.Image)
	if taskReq.ExecutorOpts.Method == types.Kubernetes {
		taskResp.AddContext("Namespace", re.antCfg.Kubernetes.Namespace)
		if re.antCfg.Kubernetes.Host != "" {
			taskResp.AddContext("KubernetesHost", re.antCfg.Kubernetes.Host)
		}
	} else if taskReq.ExecutorOpts.Method == types.Docker {
		taskResp.AddContext("DockerHost", re.antCfg.Docker.Host)
		taskResp.AddContext("DockerServer", re.antCfg.Docker.Server)
	}

	taskResp.AddJobContext(fmt.Sprintf("%s-status", taskReq.TaskType), taskResp.Status)
	taskResp.AddContext("ContainerID", container.GetID())
	taskResp.AddContext("ContainerName", container.GetName())
	taskResp.AddContext("ContainerState", container.GetState())
	if container.GetHost() != "" {
		taskResp.AddContext("ContainerHost", container.GetHost())
	}
	if container.GetContainerIP() != "" {
		taskResp.AddContext("ContainerIP", container.GetContainerIP())
	}
}

// adds additional error for tracking
func (re *RequestExecutorImpl) additionalError(
	container executor.Executor,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	err error,
	fatal bool) {
	_ = container.WriteTraceError("💣 " + err.Error())
	logrus.WithFields(
		logrus.Fields{
			"Component":    "RequestExecutorImpl",
			"AntID":        re.antCfg.ID,
			"Error":        err,
			"Request":      taskReq,
			"RequestID":    taskResp.JobRequestID,
			"UserID":       taskReq.UserID,
			"TaskType":     taskResp.TaskType,
			"Status":       taskResp.Status,
			"Message":      taskResp.ExitCode,
			"ErrorMessage": taskResp.ErrorMessage,
			"Container":    container,
		}).Warn(err.Error())
	taskResp.AdditionalError(err.Error(), fatal)
}

func sendContainerEvent(
	ctx context.Context,
	antCfg *config.AntConfig,
	queueClient queue.Client,
	userID string,
	method types.TaskMethod,
	status types.RequestState,
	container executor.Info) (err error) {
	var b []byte
	if b, err = events.NewContainerLifecycleEvent(
		"RequestExecutorImpl",
		userID,
		antCfg.ID,
		method,
		container.GetName(),
		container.GetID(),
		status,
		container.GetLabels(),
		container.GetStartedAt(),
		container.GetEndedAt()).Marshal(); err == nil {
		if _, err = queueClient.Publish(
			ctx,
			antCfg.GetContainerLifecycleTopic(),
			make(map[string]string),
			b,
			false); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "RequestExecutorImpl",
					"AntID":     antCfg.ID,
					"Container": container,
					"Error":     err,
				}).Warnf("failed to send lifecycle event container")
		}
	}
	return
}
