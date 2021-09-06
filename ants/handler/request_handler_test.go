package handler

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	"testing"

	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/registry"
	"plexobject.com/formicary/internal/artifacts"
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
	queueClient := queue.NewStubClient(&antCfg.CommonConfig)

	requestRegistry := tasklet.NewRequestRegistry(&antCfg.CommonConfig, metricsRegistry)
	artifactService, err := artifacts.NewStub(&antCfg.S3)
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

func newTestAntConfig() *config.AntConfig {
	antCfg := &config.AntConfig{
		CommonConfig: types.CommonConfig{
			Pulsar: types.PulsarConfig{
				URL: "pulsar",
			},
			Redis: types.RedisConfig{
				Host: "localhost",
			},
			S3: types.S3Config{
				Bucket:          "buc",
				Endpoint:        "end",
				AccessKeyID:     "id",
				SecretAccessKey: "sec",
			},
		},
		Methods: []types.TaskMethod{types.HTTPPostJSON},
	}
	return antCfg
}
