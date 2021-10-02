package tasklet

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"testing"
)

func Test_ShouldTerminateMessagingTasklet(t *testing.T) {
	// GIVEN tasklet
	tasklet := newTestMessagingTasklet(t)

	// WHEN terminating container
	_, err := tasklet.TerminateContainer(context.Background(), nil)

	// THEN it should not fail
	require.Error(t, err)
}

func Test_ShouldPreExecuteMessagingTasklet(t *testing.T) {
	// GIVEN tasklet
	tasklet := newTestMessagingTasklet(t)

	// WHEN pre-executing
	// THEN it should return true
	require.True(t, tasklet.PreExecute(context.Background(), nil))
}

func Test_ShouldListMessagingTasklet(t *testing.T) {
	// GIVEN tasklet
	tasklet := newTestMessagingTasklet(t)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := tasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldExecuteMessagingTasklet(t *testing.T) {
	// GIVEN tasklet
	tasklet := newTestMessagingTasklet(t)
	req := &common.TaskRequest{
		JobType:         "job",
		TaskType:        "task",
		JobRequestID:    101,
		JobExecutionID:  "201",
		TaskExecutionID: "301",
		Action:          common.EXECUTE,
		Script:          []string{"cmd"},
		ExecutorOpts:    common.NewExecutorOptions("name", common.Kubernetes),
	}

	// WHEN executing without request queue
	res, err := tasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing without response queue
	req.ExecutorOpts.MessagingRequestQueue = "input"
	res, err = tasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing without response queue
	req.ExecutorOpts.MessagingReplyQueue = "reply"
	res, err = tasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, common.COMPLETED, res.Status)
}

func newTestMessagingTasklet(t *testing.T) *MessagingTasklet {
	cfg := config.TestServerConfig()
	queueClient := queue.NewStubClient(&cfg.CommonConfig)
	jobManager := manager.AssertTestJobManager(nil, t)
	requestRegistry := tasklet.NewRequestRegistry(
		&cfg.CommonConfig,
		metrics.New(),
	)

	tasklet := NewMessagingTasklet(
		cfg,
		requestRegistry,
		jobManager,
		queueClient,
		"requestTopic",
	)
	return tasklet
}
