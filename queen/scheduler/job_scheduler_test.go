package scheduler

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"testing"
)

func Test_ShouldStartAndStopJobScheduler(t *testing.T) {
	serverCfg := config.TestServerConfig()
	// GIVEN job scheduler
	scheduler := newTestJobScheduler(t, serverCfg)

	// WHEN scheduler is started
	err := scheduler.Start(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
	// WHEN scheduler is stopped
	err = scheduler.Stop(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
}

func newTestJobScheduler(t *testing.T, serverCfg *config.ServerConfig) *JobScheduler {
	errorRepo, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverCfg.Common)
	require.NoError(t, err)
	healthMonitor, err := health.New(&serverCfg.Common, queueClient)
	require.NoError(t, err)
	scheduler := New(
		serverCfg,
		queueClient,
		manager.AssertTestJobManager(serverCfg, t),
		manager.AssertTestArtifactManager(serverCfg, t),
		manager.AssertTestUserManager(serverCfg, t),
		manager.TestResourceManager(serverCfg),
		errorRepo,
		healthMonitor,
		metrics.New(),
	)
	return scheduler
}
