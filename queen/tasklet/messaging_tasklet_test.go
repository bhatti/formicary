package tasklet

import (
	"context"
	"encoding/json"
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
	// GIVEN messagingTasklet
	messagingTasklet := newTestMessagingTasklet(t)

	// WHEN terminating container
	_, err := messagingTasklet.TerminateContainer(context.Background(), nil)

	// THEN it should not fail
	require.Error(t, err)
}

func Test_ShouldPreExecuteMessagingTasklet(t *testing.T) {
	// GIVEN messagingTasklet
	messagingTasklet := newTestMessagingTasklet(t)

	// WHEN pre-executing
	// THEN it should return true
	require.True(t, messagingTasklet.PreExecute(context.Background(), nil))
}

func Test_ShouldListMessagingTasklet(t *testing.T) {
	// GIVEN messagingTasklet
	messagingTasklet := newTestMessagingTasklet(t)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := messagingTasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldExecuteMessagingTasklet(t *testing.T) {
	// GIVEN messagingTasklet
	messagingTasklet := newTestMessagingTasklet(t)
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
	res, err := messagingTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing without response queue
	req.ExecutorOpts.MessagingRequestQueue = "input"
	res, err = messagingTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing without response queue
	req.ExecutorOpts.MessagingReplyQueue = "reply"
	res, err = messagingTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, "", res.ErrorMessage)
	require.Equal(t, common.COMPLETED, res.Status)
}

func newTestMessagingTasklet(t *testing.T) *MessagingTasklet {
	cfg := config.TestServerConfig()
	queueClient := queue.NewStubClient(&cfg.Common)
	jobManager := manager.AssertTestJobManager(nil, t)
	requestRegistry := tasklet.NewRequestRegistry(
		&cfg.Common,
		metrics.New(),
	)
	queueClient.SendReceivePayloadFunc = func(
		_ queue.MessageHeaders,
		payload []byte) ([]byte, error) {
		var req common.TaskRequest
		err := json.Unmarshal(payload, &req)
		if err != nil {
			return nil, err
		}
		res := common.NewTaskResponse(&req)
		res.AntID = "test"
		res.Host = "test"
		res.Status = common.COMPLETED
		return json.Marshal(res)
	}

	messagingTasklet := NewMessagingTasklet(
		cfg,
		requestRegistry,
		jobManager,
		queueClient,
		"requestTopic",
	)
	return messagingTasklet
}
