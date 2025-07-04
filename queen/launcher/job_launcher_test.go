package launcher

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"testing"
)

func Test_ShouldStartAndStopJobLauncher(t *testing.T) {
	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	// GIVEN job launcher
	launcher := newTestLauncher(serverCfg, t)

	// WHEN starting launcher
	err = launcher.Start(context.Background())

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN stopping launcher
	err = launcher.Stop(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
}

func newTestLauncher(serverCfg *config.ServerConfig, t *testing.T) *JobLauncher {
	errorRepo, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverCfg.Common)
	require.NoError(t, err)
	return New(serverCfg,
		queueClient,
		manager.AssertTestJobManager(serverCfg, t),
		manager.AssertTestArtifactManager(serverCfg, t),
		manager.AssertTestUserManager(serverCfg, t),
		manager.TestResourceManager(serverCfg),
		errorRepo,
		metrics.New(),
	)
}
