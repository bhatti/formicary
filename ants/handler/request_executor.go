package handler

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plexobject.com/formicary/internal/ant_config"
	"reflect"
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
	antCfg          *ant_config.AntConfig
	queueClient     queue.Client
	webClient       web.HTTPClient
	artifactService artifacts.Service
}

// NewRequestExecutor constructor
func NewRequestExecutor(
	antCfg *ant_config.AntConfig,
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

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.Common.ID,
				"Task":      taskReq.String(),
			}).Debugf("pre-processing...")
	}
	// prepare executor options and build container based on request method
	container, err := re.preProcess(ctx, taskReq)
	if err != nil {
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = taskReq.Mask(err.Error())
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
		_ = container.WriteTraceInfo(ctx,
			fmt.Sprintf("\U0001F9F0 executing pre-script for task '%s' of job '%s' with request-id '%s' name '%s'...",
				taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName()))
	}
	// prescript
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.Common.ID,
				"Task":      taskReq.String(),
			}).Debugf("executing...")
	}
	if err := re.execute(
		ctx,
		container,
		taskReq,
		taskResp,
		taskReq.BeforeScript,
		true); err != nil {
		// Execute all commands in script
		taskResp.Status = types.FAILED
		taskResp.ErrorMessage = taskReq.Mask(err.Error())
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
		taskResp.ErrorMessage = taskReq.Mask(err.Error())
	} else {
		taskResp.Status = types.COMPLETED
	}

	taskResp.Timings.ScriptFinishedAt = time.Now()
	if len(taskReq.AfterScript) > 0 {
		_ = container.WriteTraceInfo(
			ctx,
			fmt.Sprintf("📯 executing post-script for task '%s' of job '%s' with request-id '%s', name: '%s' ...",
				taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName()))
	}
	// Executing post-script regardless the task fails or succeeds
	if taskReq.ExecutorOpts.Debug && taskReq.ExecutorOpts.Privileged {
		// TODO check /run/secrets/kubernetes.io/serviceaccount
		taskReq.AfterScript = append(taskReq.AfterScript, "echo system memory in bytes && cat /sys/fs/cgroup/memory/memory.usage_in_bytes && echo cpu usage in nanoseconds && cat /sys/fs/cgroup/cpu/cpuacct.usage && df -k")
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.Common.ID,
				"Task":      taskReq.String(),
			}).Debugf("post-processing...")
	}
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
	taskResp.CostFactor = taskReq.ExecutorOpts.CostFactor

	// let post-process complete with fresh context in case it's cancelled
	re.postProcess(context.Background(), taskReq, taskResp, container)
	return
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
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
		taskReq.ExecutorOpts.ExecuteCommandWithoutShell = checkCommandCanExecuteWithoutShell(cmd)
		if ctx.Value(types.HelperContainerKey) == nil && taskReq.ExecutorOpts.Debug && taskReq.ExecutorOpts.Privileged {
			_ = container.WriteTraceInfo(
				ctx,
				fmt.Sprintf("🏃 executing command '%s' of task '%s' of job '%s' and request-id '%s', name '%s' ...",
					cmd, taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName()))
		}
		// executing...
		stdout, stderr, exitCode, exitMessage, err := re.asyncExecuteCommand(
			ctx,
			container,
			cmd,
			taskReq.Variables,
			false)
		// debugging...
		if ctx.Value(types.HelperContainerKey) == nil {
			if err == nil {
				_ = container.WriteTraceInfo(
					ctx,
					fmt.Sprintf("️🎉 executed successfully command '%s' of task '%s' of job '%s' and request-id '%s' name '%s', exit=%d stdout-len=%d",
						cmd, taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName(), exitCode, len(stdout)))
			} else {
				_ = container.WriteTraceInfo(
					ctx,
					fmt.Sprintf("😞 executed unsucessfully for command '%s' of task '%s' of job '%s' and request-id '%s' name '%s', exit=%d, error=%s stderr-len=%d",
						cmd, taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName(), exitCode, err, len(stderr)))
			}
		}
		if taskResp.ExitCode == "" || exitCode > 0 {
			taskResp.ExitCode = strconv.Itoa(exitCode)
		}
		if taskResp.ExitMessage == "" {
			taskResp.ExitMessage = taskReq.Mask(exitMessage)
		}
		taskReq.ExecutorOpts.ExecuteCommandWithoutShell = false
		if err != nil && taskResp.FailedCommand == "" {
			taskResp.FailedCommand = taskReq.Mask(cmd)
		}
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
			_ = container.WriteTraceError(ctx, err.Error())
		} else if taskReq.ExecutorOpts.ReportStdout {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(
					logrus.Fields{
						"Component": "RequestExecutorImpl",
						"AntID":     re.antCfg.Common.ID,
						"Container": container,
						"CMD":       cmd,
						"Len":       len(stdout),
					}).Debugf("adding stdout")
			}
			taskResp.Stdout = append(taskResp.Stdout, string(stdout))
		}

		// Note: this only works for SHELL/HTTP but containers will need to use it explicitly
		if taskReq.ExecutorOpts.Method.SupportsCaptureStdout() {
			path, err := addArtifactToPath(taskReq, i, cmd, stdout)
			if err != nil && failOnError {
				return err
			} else if err != nil {
				lastError = err
				_ = container.WriteTraceError(ctx, err.Error())
			}
			if path != "" {
				_ = container.WriteTraceInfo(
					ctx,
					fmt.Sprintf("📂 Adding output to artifact path %s with size %d\n", path, len(stdout)))
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
	fileName := fmt.Sprintf("%s_%s_%d.stdout",
		taskReq.JobRequestID, cutils.MakeDNS1123Compatible(taskReq.TaskType), i)
	tmpFile, err := os.Create(fileName)
	if err != nil {
		tmpFile, err = os.CreateTemp("", fileName)
	}
	if err != nil {
		return "", fmt.Errorf("failed to create artfact for %s due to %s", taskReq.ContainerName(), err)
	}
	if _, err = tmpFile.Write(stdout); err != nil {
		return "", fmt.Errorf("failed to write output for %s due to %w", cmd, err)
	}
	io.NopCloser(tmpFile)
	return tmpFile.Name(), nil
}

func (re *RequestExecutorImpl) asyncExecuteCommand(
	ctx context.Context,
	exec executor.Executor,
	cmd string,
	variables map[string]types.VariableValue,
	helper bool) (stdout []byte, stderr []byte, exitCode int, exitMessage string, err error) {
	var runner executor.CommandRunner
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component":    "RequestExecutorImpl",
				"AntID":        re.antCfg.Common.ID,
				"ExecutorID":   exec.GetID(),
				"ExecutorName": exec.GetName(),
				"ExecutorType": reflect.TypeOf(exec).String(),
				"Command":      cmd,
				"Helper":       helper,
			}).Debugf("async-executing...")
	}
	if helper {
		ctx = context.WithValue(ctx, types.HelperContainerKey, true)
		runner, err = exec.AsyncHelperExecute(ctx, cmd, variables)
	} else {
		runner, err = exec.AsyncExecute(ctx, cmd, variables)
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

	taskReq.ExecutorOpts.PodLabels[types.AntID] = re.antCfg.Common.ID

	// Add variables as environment variables
	for k, v := range taskReq.Variables {
		taskReq.ExecutorOpts.Environment[k] = fmt.Sprintf("%v", v)
	}

	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	if re.antCfg.Common.S3.UseSSL {
		taskReq.ExecutorOpts.HelperEnvironment["AWS_URL"] = fmt.Sprintf("%s://%s", httpsPrefix,
			re.antCfg.Common.S3.Endpoint)
	} else {
		taskReq.ExecutorOpts.HelperEnvironment["AWS_URL"] = fmt.Sprintf("%s://%s", httpPrefix,
			re.antCfg.Common.S3.Endpoint)
	}
	taskReq.ExecutorOpts.HelperEnvironment["AWS_ENDPOINT"] = re.antCfg.Common.S3.Endpoint
	taskReq.ExecutorOpts.HelperEnvironment["AWS_ACCESS_KEY_ID"] = re.antCfg.Common.S3.AccessKeyID
	taskReq.ExecutorOpts.HelperEnvironment["AWS_SECRET_ACCESS_KEY"] = re.antCfg.Common.S3.SecretAccessKey
	taskReq.ExecutorOpts.HelperEnvironment["AWS_DEFAULT_REGION"] = re.antCfg.Common.S3.Region

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
		//	return nil, fmt.Errorf("failed to create artDir directory due to %w", err)
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
				"AntID":     re.antCfg.Common.ID,
				"Container": container,
				"Error":     sendErr,
			}).Warnf("failed to send lifecycle event container")
	}
	_ = container.WriteTraceInfo(
		ctx,
		fmt.Sprintf("🚀 starting task '%s' of job '%s' with request-id '%s' name: '%s' ...",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName()))
	if taskReq.ExecutorOpts.Debug {
		_ = container.WriteTraceInfo(
			ctx,
			fmt.Sprintf("env variables: %v", taskReq.ExecutorOpts.Environment))
	}

	return
}

func (re *RequestExecutorImpl) postProcess(
	ctx context.Context,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	container executor.Executor) {
	// upload artifacts unless job is cancelled
	if !taskReq.Cancelled && container.GetState() != executor.ContainerFailed {
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
				ctx,
				container,
				taskReq,
				taskResp,
				fmt.Errorf("failed to upload artifacts due to '%v'",
					err), true)
		} else if len(uploadedArtifacts) > 0 {
			for _, artifact := range uploadedArtifacts {
				_ = container.WriteTraceInfo(
					ctx,
					fmt.Sprintf("🗄️ uploaded artifact Bucket=%s ID=%s SHA256=%s Size=%d",
						re.antCfg.Common.S3.Bucket, artifact.ID, artifact.SHA256, artifact.ContentLength))
			}
		}
	} else {
		_ = container.WriteTraceInfo(
			ctx,
			fmt.Sprintf("🚫 skipped uploading artifacts, cancelled=%v, container=%s",
				taskReq.Cancelled, container.GetState()))
	}
	taskResp.Timings.ArtifactsUploadedAt = time.Now()
	// Stopping container
	now := time.Now()
	if err := container.Stop(ctx); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.Common.ID,
				"Request":   taskReq,
				"Response":  taskResp,
				"ExitCode":  taskResp.ExitCode,
				"ErrorCode": taskResp.ErrorCode,
				"UserID":    taskReq.UserID,
				"Container": container,
				"Elapsed":   time.Since(now).String(),
				"Error":     err,
			}).Warnf("🛑 failed to stop container post-process")
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestExecutorImpl",
				"AntID":     re.antCfg.Common.ID,
				"Request":   taskReq,
				"Response":  taskResp,
				"ExitCode":  taskResp.ExitCode,
				"ErrorCode": taskResp.ErrorCode,
				"UserID":    taskReq.UserID,
				"Container": container,
				"Elapsed":   time.Since(now).String(),
			}).Info("🛑 stopped container post-process")
	}

	taskResp.Timings.PodShutdownAt = time.Now()
	removeTemporaryFiles(taskReq)

	elapsed := time.Now().Sub(taskReq.StartedAt).String()
	if taskResp.Status.Failed() {
		_ = container.WriteTraceError(
			ctx,
			fmt.Sprintf("⛔ task '%s' of job '%s' with request-id '%s' name '%s' failed Error=%s Exit=%s Duration=%s",
				taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName(), taskResp.ErrorMessage, taskResp.ExitCode, elapsed))
	} else {
		_ = container.WriteTraceSuccess(
			ctx,
			fmt.Sprintf("🙌 task '%s' of job '%s' with request-id '%s' name '%s' completed Duration=%s",
				taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName(), elapsed))
	}

	_ = container.WriteTraceSuccess(
		ctx,
		fmt.Sprintf("⏱ task '%s' of job '%s' with request-id '%s' name '%s' exit: %s error-code: %s stats: %s",
			taskReq.TaskType, taskReq.JobType, taskReq.JobRequestID, taskReq.ContainerName(), taskResp.ExitCode,
			taskResp.ErrorCode, taskResp.Timings.String()))

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
					"AntID":     re.antCfg.Common.ID,
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
				"AntID":     re.antCfg.Common.ID,
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
	if files, err := filepath.Glob(fmt.Sprintf("%s_*", taskReq.JobRequestID)); err == nil {
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
	taskResp.AntID = re.antCfg.Common.ID
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
	ctx context.Context,
	container executor.Executor,
	taskReq *types.TaskRequest,
	taskResp *types.TaskResponse,
	err error,
	fatal bool) {
	_ = container.WriteTraceError(ctx, "💣 "+err.Error())
	logrus.WithFields(
		logrus.Fields{
			"Component":    "RequestExecutorImpl",
			"AntID":        re.antCfg.Common.ID,
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
	antCfg *ant_config.AntConfig,
	queueClient queue.Client,
	userID string,
	method types.TaskMethod,
	status types.RequestState,
	container executor.Info) (err error) {
	var b []byte
	if b, err = events.NewContainerLifecycleEvent(
		"RequestExecutorImpl",
		userID,
		antCfg.Common.ID,
		method,
		container.GetName(),
		container.GetID(),
		status,
		container.GetLabels(),
		container.GetStartedAt(),
		container.GetEndedAt()).Marshal(); err == nil {
		if _, err = queueClient.Publish(
			ctx,
			antCfg.Common.GetContainerLifecycleTopic(),
			b,
			queue.NewMessageHeaders(
				queue.DisableBatchingKey, "true",
				"ContainerID", container.GetID(),
				"UserID", userID,
			),
		); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "RequestExecutorImpl",
					"AntID":     antCfg.Common.ID,
					"Container": container,
					"Error":     err,
				}).Warnf("failed to send lifecycle event container")
		}
	}
	return
}

func checkCommandCanExecuteWithoutShell(cmd string) bool {
	for i := 0; i < len(cmd); i++ {
		if cmd[i] == '&' ||
			cmd[i] == '|' ||
			cmd[i] == '$' ||
			cmd[i] == '>' ||
			cmd[i] == '<' ||
			(cmd[i] >= '0' && cmd[i] <= '9') {
			return false
		}
	}
	return true
}
