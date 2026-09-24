package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/ants/registry"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

func Test_ShouldStartAndStopRequestHandler(t *testing.T) {
	// GIVEN an ant container registry is instantiated
	webClient := web.NewStubHTTPClient()
	metricsRegistry := metrics.New()
	antCfg := newTestAntConfig()
	require.NoError(t, antCfg.Validate())
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &antCfg.Common)
	require.NoError(t, err)

	requestRegistry := tasklet.NewRequestRegistry(&antCfg.Common, metricsRegistry)
	artifactService, err := artifacts.NewStub(antCfg.Common.S3)
	require.NoError(t, err)
	antContainersRegistry := registry.NewAntContainersRegistry(antCfg, queueClient, metricsRegistry)
	err = antContainersRegistry.Start(context.Background())
	require.NoError(t, err)

	// AND a new handler is created
	handler := NewRequestHandler(
		antCfg,
		queueClient,
		webClient,
		requestRegistry,
		antContainersRegistry,
		metricsRegistry,
		NewRequestExecutor(antCfg, queueClient, webClient, artifactService),
		"requestTopic")

	// WHEN a handler is started
	err = handler.Start(context.Background())
	// THEN it should not fail
	require.NoError(t, err)

	// AND WHEN a handler is stopped
	err = handler.Stop(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
}

// Test_ShouldTerminateContainerEvenWhenNotInRegistry verifies the fix for the silent-failure bug:
// before the fix, TerminateContainer returned ErrorContainerNotFound immediately when the container
// was absent from the local registry (e.g. after a prior job cancel cleared it) — without ever
// attempting the actual pod deletion. After the fix it proceeds to StopContainer regardless.
func Test_ShouldTerminateContainerEvenWhenNotInRegistry(t *testing.T) {
	webClient := web.NewStubHTTPClient()
	metricsRegistry := metrics.New()
	antCfg := newTestAntConfig()
	require.NoError(t, antCfg.Validate())
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &antCfg.Common)
	require.NoError(t, err)

	requestRegistry := tasklet.NewRequestRegistry(&antCfg.Common, metricsRegistry)
	artifactService, err := artifacts.NewStub(antCfg.Common.S3)
	require.NoError(t, err)
	antContainersRegistry := registry.NewAntContainersRegistry(antCfg, queueClient, metricsRegistry)
	require.NoError(t, antContainersRegistry.Start(context.Background()))

	handler := NewRequestHandler(
		antCfg,
		queueClient,
		webClient,
		requestRegistry,
		antContainersRegistry,
		metricsRegistry,
		NewRequestExecutor(antCfg, queueClient, webClient, artifactService),
		"requestTopic")
	require.NoError(t, handler.Start(context.Background()))
	defer func() { _ = handler.Stop(context.Background()) }()

	taskReq := &types.TaskRequest{
		JobRequestID:    "test-job-1",
		TaskExecutionID: "test-exec-1",
		TaskType:        "build",
		ExecutorOpts:    types.NewExecutorOptions("my-container", types.HTTPPostJSON),
	}

	// Container is NOT in registry (simulates pod surviving after job cancel cleared the registry).
	// StopContainer will fail for the HTTP stub (no executor with this id) — that is expected.
	taskResp, stopErr := handler.TerminateContainer(context.Background(), taskReq)
	require.NotNil(t, taskResp)

	// Key assertion: the new code reaches StopContainer (error = ErrorContainerStoppedFailed),
	// not the old bail-out (ErrorContainerNotFound). The HTTP stub has no executor for this ID
	// so StopContainer fails — but that proves it was attempted.
	assert.Equal(t, types.FAILED, taskResp.Status)
	assert.Equal(t, types.ErrorContainerStoppedFailed, taskResp.ErrorCode,
		"must attempt StopContainer even when container is absent from registry; "+
			"ErrorContainerNotFound would mean the old early-return is still in place")
	assert.Error(t, stopErr, "StopContainer failure must be returned as a Go error")
}

func newTestAntConfig() *ant_config.AntConfig {
	antCfg := &ant_config.AntConfig{
		Common: types.CommonConfig{
			Redis: &types.RedisConfig{
				Host: "localhost",
			},
			S3: &types.S3Config{
				Bucket:          "buc",
				Endpoint:        "end",
				AccessKeyID:     "id",
				SecretAccessKey: "sec",
			},
		},
		Methods: []types.TaskMethod{types.HTTPPostJSON},
	}
	_ = antCfg.Validate()
	return antCfg
}
