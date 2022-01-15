package controller

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"strings"
	"testing"
	"time"
)

func Test_InitializeSwaggerStructsForContainerExecution(t *testing.T) {
	_ = containerExecutionsQueryParams{}
	_ = containerExecutionsQueryResponseBody{}
	_ = containerIDParamsBody{}
}

func Test_ShouldQueryContainerExecutions(t *testing.T) {
	// GIVEN container execution controller
	cfg := config.TestServerConfig()
	queueClient := buildTestQueueClient(cfg)
	mgr := newTestResourceManager(cfg, queueClient, t)
	sendContainerExecution(cfg, queueClient, t)

	webServer := web.NewStubWebServer()
	ctrl := NewContainerExecutionController(mgr, webServer)

	// WHEN querying execution controllers
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err := ctrl.queryContainerExecutions(ctx)

	// THEN it should return valid results
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*events.ContainerLifecycleEvent)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldDeleteContainerExecution(t *testing.T) {
	// GIVEN container execution controller
	cfg := config.TestServerConfig()
	queueClient := buildTestQueueClient(cfg)
	mgr := newTestResourceManager(cfg, queueClient, t)
	sendAntRegistration(cfg, queueClient, t)
	sendContainerExecution(cfg, queueClient, t)

	webServer := web.NewStubWebServer()
	ctrl := NewContainerExecutionController(mgr, webServer)

	// WHEN deleting container execution
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = "container-id"
	ctx.Params["antID"] = "ant-id"
	ctx.Params["method"] = "test"
	err := ctrl.deleteContainerExecution(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func sendContainerExecution(serverCfg *config.ServerConfig, queueClient queue.Client, t *testing.T) {
	event := events.ContainerLifecycleEvent{
		AntID:          "ant-id",
		Method:         common.TaskMethod("test"),
		ContainerName:  "container-name",
		ContainerID:    "container-id",
		ContainerState: common.EXECUTING,
		Labels:         make(map[string]string),
		StartedAt:      time.Now(),
	}
	bEvent, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	_, _ = queueClient.Send(
		context.Background(),
		serverCfg.GetContainerLifecycleTopic(),
		bEvent,
		queue.NewMessageHeaders(
			queue.ReusableTopicKey, "false",
			queue.Source, "test",
		),
	)
}
