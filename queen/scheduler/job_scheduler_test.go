package scheduler

import (
	"context"
	"time"

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

// Test_ShouldWakeSchedulerOnTriggerChannel verifies that a send on triggerCh causes the
// scheduling loop to consume the signal before the next regular tick fires.
// Strategy: we set a 30s tick so any drain must come from the triggerCh case.
// We send a signal, then spin-wait (with deadline) for the channel to be empty,
// confirming the scheduler goroutine received it. We only start polling AFTER the
// send to avoid a false-positive from seeing an empty channel before the send.
func Test_ShouldWakeSchedulerOnTriggerChannel(t *testing.T) {
	serverCfg := config.TestServerConfig()
	serverCfg.Jobs.JobSchedulerCheckPendingJobsInterval = 30 * time.Second

	triggerCh := make(chan struct{}, 1)
	errorRepo, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverCfg.Common)
	require.NoError(t, err)
	healthMonitor, err := health.New(&serverCfg.Common, queueClient)
	require.NoError(t, err)
	js := New(
		serverCfg,
		queueClient,
		manager.AssertTestJobManager(serverCfg, t),
		manager.AssertTestArtifactManager(serverCfg, t),
		manager.AssertTestUserManager(serverCfg, t),
		manager.TestResourceManager(serverCfg),
		errorRepo,
		healthMonitor,
		metrics.New(),
		nil,
		nil,
		triggerCh,
	)

	require.NoError(t, js.Start(context.Background()))
	defer func() { _ = js.Stop(context.Background()) }()

	// WHEN: a trigger is sent
	triggerCh <- struct{}{}

	// THEN: the scheduler goroutine must drain it well before the 30-second tick.
	// We poll AFTER the send, so len==0 can only mean the goroutine consumed it.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(triggerCh) == 0 {
			return // pass
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scheduler did not drain triggerCh within 2 seconds")
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
		nil, // approvalSvc - not needed for unit tests
		nil, // retentionManager - not needed for unit tests
		nil, // no external triggerCh in unit tests
	)
	return scheduler
}
