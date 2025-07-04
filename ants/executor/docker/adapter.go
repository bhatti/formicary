package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/registry"
	"io"
	"plexobject.com/formicary/internal/ant_config"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/executor"
	domain "plexobject.com/formicary/internal/types"
)

// ExecuteInfo struct
type ExecuteInfo struct {
	ID        string
	IPAddress string
	HostName  string
	Hijack    types.HijackedResponse
}

// Adapter for docker APIs
type Adapter interface {
	Pull(ctx context.Context, image string) (io.ReadCloser, error)
	Stop(
		ctx context.Context,
		id string,
		opts *domain.ExecutorOptions,
		timeout time.Duration) error
	IsExecuteRunning(ctx context.Context, id string) (bool, int, error)
	Build(
		ctx context.Context,
		opts *domain.ExecutorOptions,
		name string,
		image string,
		entrypoint []string,
		helper bool) (string, error)
	List(ctx context.Context) ([]executor.Info, error)
	Execute(
		ctx context.Context,
		opts *domain.ExecutorOptions,
		containerID string,
		cmd string,
		executeCommandWithoutShell bool,
		helper bool) (ExecuteInfo, error)
	GetLogs(ctx context.Context, name string, waitForNotRunning bool) (io.ReadCloser, error)
	GetRuntimeInfo(ctx context.Context, container string) string
}

// Utils defines helper methods using docker API
type Utils struct {
	config       *ant_config.DockerConfig
	cli          *client.Client
	attachOption bool
}

// NewDockerUtils creates new adapter for docker
func NewDockerUtils(config *ant_config.DockerConfig, cli *client.Client) *Utils {
	return &Utils{config: config, cli: cli}
}

// Execute - executes a command
func (u *Utils) Execute(
	ctx context.Context,
	opts *domain.ExecutorOptions,
	containerID string,
	cmd string,
	executeCommandWithoutShell bool,
	helper bool) (info ExecuteInfo, err error) {
	var cmds []string
	if executeCommandWithoutShell {
		cmds = strings.Split(cmd, " ")
	} else {
		cmds = []string{"/bin/sh", "-c", cmd}
	}
	execConfig := types.ExecConfig{
		AttachStdin:  false,
		AttachStderr: true,
		AttachStdout: true,
		Tty:          false,
		Cmd:          cmds,
	}
	if helper {
		execConfig.Env = opts.HelperEnvironment.AsArray()
	} else {
		execConfig.Env = opts.Environment.AsArray()
	}
	if opts.WorkingDirectory != "" {
		execConfig.WorkingDir = opts.WorkingDirectory
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":                  "DockerAdapter",
			"Container":                  containerID,
			"Command":                    cmds,
			"Options":                    opts,
			"CWD":                        execConfig.WorkingDir,
			"ExecuteCommandWithoutShell": executeCommandWithoutShell,
		}).Debug("executing...")
	}
	containerInfo, err := u.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return info, fmt.Errorf("failed to inspect container for execution %s due to %w", cmd, err)
	}

	// Creating execution
	resp, err := u.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return info, fmt.Errorf("failed to create execution %s due to %w", cmd, err)
	}

	execStartCheck := types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	}

	//
	var hijack types.HijackedResponse
	if u.attachOption {
		if hijack, err = u.cli.ContainerAttach(ctx, containerID, attachOptions()); err != nil {
			return info, fmt.Errorf("failed to attach to container %s due to %w", cmd, err)
		}
		if err := u.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
			return info, fmt.Errorf("failed to execute (ContainerStart) %s due to %w", cmd, err)
		}
	} else {
		if hijack, err = u.cli.ContainerExecAttach(ctx, resp.ID, execStartCheck); err != nil {
			return info, fmt.Errorf("failed to attach to container %s due to %w", cmd, err)
		}
		if err := u.cli.ContainerExecStart(ctx, resp.ID, execStartCheck); err != nil {
			return info, fmt.Errorf("failed to start execution %s due to %w", cmd, err)
		}
	}
	//defer hijack.Close()
	//var stdout, stderr bytes.Buffer
	//if _, err := stdcopy.StdCopy(&stdout, &stderr, out.Reader); err != nil {
	//return info, err
	//}
	info.Hijack = hijack
	info.ID = resp.ID
	if containerInfo.Node != nil {
		info.HostName = containerInfo.Node.Addr
		info.IPAddress = containerInfo.Node.IPAddress
	}
	return
}

// Build method builds container
func (u *Utils) Build(
	ctx context.Context,
	opts *domain.ExecutorOptions,
	name string,
	image string,
	entrypoint []string,
	helper bool) (string, error) {
	started := time.Now()
	// using fresh context so that it doesn't time out
	ctx = context.Background()
	if u.config.PullPolicy.Always() || u.config.PullPolicy.IfNotPresent() {
		_, err := u.Pull(ctx, image)
		if err != nil {
			return "", fmt.Errorf("failed to pull image %s due to %w", image, err)
		}
	}

	containerConfig := &container.Config{
		Image:        image,
		Labels:       opts.PodLabels,
		Tty:          true,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    false,
		StdinOnce:    false,
	}
	if helper {
		containerConfig.Env = opts.HelperEnvironment.AsArray()
	} else {
		containerConfig.Env = opts.Environment.AsArray()
	}
	if opts.WorkingDirectory != "" {
		containerConfig.WorkingDir = opts.WorkingDirectory
	}

	containerConfig.Entrypoint = entrypoint

	hostConfig := &container.HostConfig{
		Privileged: opts.Privileged,
		RestartPolicy: container.RestartPolicy{
			Name:              "no",
			MaximumRetryCount: 0,
		},
	}
	if opts.NetworkMode != "" {
		hostConfig.NetworkMode = container.NetworkMode(opts.NetworkMode) // default, container
	}

	if opts.MainContainer.HasDockerFromVolumes() ||
		opts.MainContainer.HasDockerBindVolumes() {
		if err := u.createDockerVolumes(ctx, opts); err != nil {
			return "", err
		}
	}
	if opts.MainContainer.HasDockerFromVolumes() {
		containerConfig.Volumes = opts.MainContainer.GetDockerVolumes()
		hostConfig.VolumesFrom = opts.MainContainer.VolumesFrom
	} else if opts.MainContainer.HasDockerBindVolumes() { // this is used for artifacts sharing
		containerConfig.Volumes = opts.MainContainer.GetDockerVolumes()
		hostConfig.Mounts = opts.MainContainer.GetDockerMounts()
	}

	resp, err := u.cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		name)
	if err != nil {
		return "", fmt.Errorf("failed to create container %s due to %w, elapsed %s",
			name, err, time.Since(started))
	}

	_, err = u.cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		_ = u.cli.ContainerRemove(
			ctx,
			resp.ID,
			container.RemoveOptions{})
		return "", fmt.Errorf("failed to inspect container %s due to %w", name, err)
	}

	//
	err = u.cli.ContainerStart(
		ctx,
		resp.ID,
		container.StartOptions{})
	if err != nil {
		_ = u.cli.ContainerStop(
			ctx,
			resp.ID,
			container.StopOptions{})
		_ = u.cli.ContainerRemove(
			ctx,
			resp.ID,
			container.RemoveOptions{})
		return "", fmt.Errorf("failed to start container %s due to %w", name, err)
	}

	logrus.WithFields(logrus.Fields{
		"Component":   "DockerAdapter",
		"Container":   name,
		"Image":       image,
		"ContainerID": resp.ID,
		"Mounts":      hostConfig.Mounts,
		"Options":     opts.String(),
	}).Info("creating container...")

	return resp.ID, nil
}

func (u *Utils) createDockerVolumes(ctx context.Context, opts *domain.ExecutorOptions) error {
	volNames := opts.MainContainer.GetDockerVolumeNames()
	for vol := range volNames {
		vBody := volume.CreateOptions{
			Name:   vol,
			Labels: map[string]string{"type": "shared"},
		}
		_, err := u.cli.VolumeCreate(ctx, vBody)
		if err != nil {
			return fmt.Errorf("failed to create docker volume %s: %w",
				vBody.Name, err)
		}
	}
	return nil
}

func (u *Utils) removeDockerVolumes(ctx context.Context, opts *domain.ExecutorOptions) error {
	volNames := opts.MainContainer.GetDockerVolumeNames()
	for vol := range volNames {
		for i := 0; i < 10; i++ {
			if err := u.cli.VolumeRemove(ctx, vol, true); err == nil {
				break
			} else {
				if strings.Contains(err.Error(), "volume is in use") {
					time.Sleep(1 * time.Second)
				} else {
					return fmt.Errorf("failed to remove docker volume %s: %w",
						vol, err)
				}
			}
		}
	}
	return nil
}

// Pull method fetches images from docker registry
func (u *Utils) Pull(ctx context.Context, image string) (io.ReadCloser, error) {
	authConfig := registry.AuthConfig{
		Username:      u.config.Username,
		Password:      u.config.Password,
		ServerAddress: u.config.Server,
	}
	logrus.WithFields(logrus.Fields{
		"Component": "DockerAdapter",
		"Image":     image,
		"Server":    u.config.Server,
	}).Info("pulling docker image...")
	//
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(authConfig); err != nil {
		return nil, err
	}
	//
	options := types.ImagePullOptions{}
	if strings.Contains(image, u.config.Server) {
		options.RegistryAuth = base64.URLEncoding.EncodeToString(buf.Bytes())
	}
	return u.cli.ImagePull(ctx, image, options)
}

// Stop stops container
func (u *Utils) Stop(
	ctx context.Context,
	id string,
	opts *domain.ExecutorOptions,
	timeout time.Duration) error {
	logrus.WithFields(logrus.Fields{
		"Component": "DockerAdapter",
		"ID":        id,
	}).Info("✋ stopping docker container...")
	timeoutSecs := int(timeout.Seconds())
	err := u.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeoutSecs})
	if err != nil {
		return fmt.Errorf("failed to stop docker container %s due to %w, timeout=%s",
			id, err, timeout)
	}

	logrus.WithFields(logrus.Fields{
		"Component": "DockerAdapter",
		"ID":        id,
	}).Info("removing docker container...")

	rmOptions := container.RemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	rmErr := u.cli.ContainerRemove(ctx, id, rmOptions)

	logrus.WithFields(logrus.Fields{
		"Component": "DockerAdapter",
		"ID":        id,
	}).Info("✋ removing docker volumes...")
	if err = u.removeDockerVolumes(ctx, opts); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "DockerAdapter",
			"ID":        id,
			"Error":     err,
		}).Error("failed to remove volume...")
		if rmErr == nil {
			rmErr = err
		}
	}

	return rmErr
}

// List containers
func (u *Utils) List(ctx context.Context) ([]executor.Info, error) {
	result, err := u.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers due to %w", err)
	}

	arr := make([]executor.Info, 0)

	// See https://godoc.org/github.com/docker/docker/api/types#Container
	for _, entry := range result {
		opts := domain.NewExecutorOptions("", "")
		opts.PodLabels = entry.Labels
		exec := executor.BaseExecutor{
			ExecutorOptions: opts,
			ID:              entry.ID,
			Name:            entry.Names[0],
			StartedAt:       time.Unix(entry.Created, 0),
			State:           executor.State(strings.ToUpper(entry.State)),
			Labels:          entry.Labels,
		}
		arr = append(arr, &exec)
	}
	return arr, nil
}

// IsExecuteRunning returns true if container is running
func (u *Utils) IsExecuteRunning(ctx context.Context, id string) (bool, int, error) {
	resp, err := u.cli.ContainerExecInspect(ctx, id)
	if err != nil {
		return false, 0, err
	}
	return resp.Running, resp.ExitCode, nil
}

// GetLogs returns logs
func (u *Utils) GetLogs(ctx context.Context, name string, waitForNotRunning bool) (io.ReadCloser, error) {
	logsOut, err := u.cli.ContainerLogs(ctx, name, container.LogsOptions{ShowStdout: true})
	if err != nil {
		return nil, err
	}
	if waitForNotRunning {
		statusCh, errCh := u.cli.ContainerWait(ctx, name, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				return logsOut, err
			}
		case <-statusCh:
		}
	}
	return logsOut, nil
}

// GetRuntimeInfo returns runtime info
func (u *Utils) GetRuntimeInfo(ctx context.Context, container string) string {
	info, err := u.cli.Info(ctx)
	if err != nil {
		return err.Error()
	}

	var sb strings.Builder
	if reader, err := u.GetLogs(ctx, container, false); err == nil {
		if data, err := io.ReadAll(reader); err == nil {
			sb.Write(data)
		}
	}
	if data, err := json.Marshal(info); err == nil {
		sb.Write(data)
	}
	return sb.String()
}

func attachOptions() container.AttachOptions {
	return container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}
}
