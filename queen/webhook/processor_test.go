package webhook

import (
	"context"
	"encoding/json"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"testing"
)

func Test_ShouldCreateJobWebhook(t *testing.T) {
	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	ctx := context.Background()
	// GIVEN webhook processor
	queueClient := queue.NewStubClient(&serverCfg.Common)
	http := web.NewStubHTTPClient()
	http.PostMapping["https://formicary.io/webhook/jobs"] = web.NewStubHTTPResponse(200, "test-body")

	processor := New(serverCfg, queueClient, http)
	_ = processor.Start(ctx)
	defer func() {
		_ = processor.Stop(ctx)
	}()
	event := events.NewWebhookJobEvent(
		&events.JobExecutionLifecycleEvent{
			JobRequestID:   ulid.Make().String(),
			JobType:        "sample-job",
			JobExecutionID: "200",
			JobState:       common.COMPLETED,
		},
		&common.Webhook{
			URL: "https://formicary.io/webhook/jobs",
		})
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	_, err = queueClient.Publish(ctx, serverCfg.Common.GetJobWebhookTopic(), payload, make(queue.MessageHeaders))
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, int64(1), processor.jobsProcessed)
}

func Test_ShouldCreateTaskWebhook(t *testing.T) {
	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	ctx := context.Background()
	// GIVEN webhook processor
	queueClient := queue.NewStubClient(&serverCfg.Common)
	http := web.NewStubHTTPClient()
	http.PostMapping["https://formicary.io/webhook/tasks"] = web.NewStubHTTPResponse(200, "test-body")

	processor := New(serverCfg, queueClient, http)
	_ = processor.Start(ctx)
	defer func() {
		_ = processor.Stop(ctx)
	}()
	event := events.NewWebhookTaskEvent(
		&events.TaskExecutionLifecycleEvent{
			JobRequestID:    "101",
			TaskType:        "sample-job",
			JobExecutionID:  "100",
			TaskExecutionID: "200",
			TaskState:       common.COMPLETED,
		},
		&common.Webhook{
			URL: "https://formicary.io/webhook/tasks",
		})
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	_, err = queueClient.Publish(ctx, serverCfg.Common.GetTaskWebhookTopic(), payload, make(queue.MessageHeaders))
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, int64(1), processor.tasksProcessed)
}
