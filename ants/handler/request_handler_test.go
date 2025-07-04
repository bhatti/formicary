package handler

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/metrics"
	"testing"

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
