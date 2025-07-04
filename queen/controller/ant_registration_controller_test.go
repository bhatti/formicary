package controller

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/resource"
	"strings"
	"testing"
	"time"
)

func Test_InitializeSwaggerStructsForAntRegistration(t *testing.T) {
	_ = antQueryParams{}
	_ = antRegistrationsQueryResponseBody{}
	_ = antIDParams{}
	_ = antRegistrationResponseBody{}
}

func Test_ShouldQueryAntRegistration(t *testing.T) {
	// GIVEN ant registration controller

	cfg := config.TestServerConfig()
	queueClient := buildTestQueueClient(cfg)

	mgr := newTestResourceManager(cfg, queueClient, t)
	sendAntRegistration(cfg, queueClient, t)

	webServer := web.NewStubWebServer()
	ctrl := NewAntRegistrationController(mgr, webServer)

	// WHEN querying ant registration
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err := ctrl.queryAntRegistrations(ctx)

	// THEN it should return valid results
	require.NoError(t, err)
	recs := ctx.Result.([]*common.AntRegistration)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldGetAntRegistration(t *testing.T) {
	// GIVEN ant registration controller

	cfg := config.TestServerConfig()
	queueClient := buildTestQueueClient(cfg)
	mgr := newTestResourceManager(cfg, queueClient, t)
	sendAntRegistration(cfg, queueClient, t)

	webServer := web.NewStubWebServer()
	ctrl := NewAntRegistrationController(mgr, webServer)

	// WHEN fetching ant registration
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = "ant-id"
	err := ctrl.getAntRegistration(ctx)
	// THEN it should return valid results
	require.NoError(t, err)
}

func newTestResourceManager(serverCfg *config.ServerConfig, queueClient queue.Client, t *testing.T) resource.Manager {
	mgr := resource.New(serverCfg, queueClient)
	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	return mgr
}

func sendAntRegistration(serverCfg *config.ServerConfig, queueClient queue.Client, t *testing.T) {
	reg := common.AntRegistration{
		AntID:        "ant-id",
		AntTopic:     "ant-topic",
		Methods:      []common.TaskMethod{"test"},
		Tags:         []string{"tag"},
		CurrentLoad:  1,
		MaxCapacity:  10,
		AntStartedAt: time.Now(),
		ReceivedAt:   time.Now(),
		CreatedAt:    time.Now(),
	}
	b, err := json.Marshal(reg)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	_, _ = queueClient.Send(
		context.Background(),
		serverCfg.Common.GetRegistrationTopic(),
		b,
		make(map[string]string),
	)
}

func buildTestQueueClient(cfg *config.ServerConfig) queue.Client {
	queueClient, _ := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	if channelClient, ok := queueClient.(*queue.ClientChannel); ok {
		channelClient.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			err := json.Unmarshal(inReq.Payload, &req)
			if err != nil {
				return nil, err
			}
			res := common.NewTaskResponse(&req)
			res.AntID = "query-ant-test"
			res.Host = "query-ant-test"
			res.Status = common.COMPLETED
			return json.Marshal(res)
		})
	}
	return queueClient
}
