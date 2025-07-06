package tasklet

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/events"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
)

// ManualTasklet structure
type ManualTasklet struct {
	serverCfg *config.ServerConfig
	*tasklet.BaseTasklet
}

// NewManualTasklet constructor
func NewManualTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	queueClient queue.Client,
) *ManualTasklet {
	id := serverCfg.Common.ID + "-manual"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     "none",
		MaxCapacity:  1000000,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.Manual},
		Allocations:  make(map[string]*common.AntAllocation),
		AutoRefresh:  true,
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &ManualTasklet{
		serverCfg: serverCfg,
	}

	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.Common,
		queueClient,
		nil,
		requestRegistry,
		"none",
		serverCfg.Common.GetRegistrationTopic(),
		&registration,
		t,
	)
	return t
}

// TerminateContainer terminates container
func (t *ManualTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *ManualTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	taskResp = common.NewTaskResponse(req)
	taskResp.Status = common.COMPLETED
	taskResp.AntID = t.ID
	taskResp.Host = "server"
	taskResp.AddContext("action", "ListContainers")
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute check if request can be executed
func (t *ManualTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute task request
func (t *ManualTasklet) Execute(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("requires manual processing")
}
