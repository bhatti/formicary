package tasklet

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	"testing"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

func Test_ShouldStartAndStopRegistration(t *testing.T) {
	cfg := newTestCommonConfig()
	err := cfg.Validate(make([]string, 0))
	require.NoError(t, err)

	// GIVEN request registry
	metricsRegistry := metrics.New()
	queueClient := queue.NewStubClient(cfg)
	requestRegistry := NewRequestRegistry(cfg, metricsRegistry)

	// WHEN a tasklet is created and registered
	registration := types.AntRegistration{
		AntID:        "ant-id",
		AntTopic:     "requestTopic",
		MaxCapacity:  100,
		Tags:         make([]string, 0),
		Methods:      []types.TaskMethod{types.ForkJob},
		Allocations:  make(map[string]*types.AntAllocation),
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}

	tasklet := NewBaseTasklet(
		"suffix",
		cfg,
		queueClient,
		nil,
		requestRegistry,
		"requestTopic",
		"registrationTopic",
		&registration,
		&MockExecutorImpl{})

	// THEN tasklet start and stop should not fail
	err = tasklet.Start(context.Background())
	require.NoError(t, err)
	err = tasklet.Stop(context.Background())
	require.NoError(t, err)
}

func newTestCommonConfig() *types.CommonConfig {
	cfg := &types.CommonConfig{}
	cfg.S3.AccessKeyID = "admin"
	cfg.S3.SecretAccessKey = "password"
	cfg.S3.Bucket = "test-bucket"
	cfg.Pulsar.URL = "test"
	cfg.Redis.Host = "test"
	return cfg
}

type MockExecutorImpl struct {
}

func (m *MockExecutorImpl) PreExecute(
	_ context.Context,
	_ *types.TaskRequest) bool {
	return true
}

func (m *MockExecutorImpl) Execute(
	_ context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	taskResp = types.NewTaskResponse(taskReq)
	taskResp.Status = types.COMPLETED
	return
}

func (m *MockExecutorImpl) TerminateContainer(
	_ context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	taskResp = types.NewTaskResponse(taskReq)
	taskResp.Status = types.COMPLETED
	return
}

func (m *MockExecutorImpl) ListContainers(
	_ context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	taskResp = types.NewTaskResponse(taskReq)
	taskResp.Status = types.COMPLETED
	return
}
