package tasklet

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
)

// ArtifactExpirationTasklet structure
type ArtifactExpirationTasklet struct {
	serverCfg *config.ServerConfig
	*tasklet.BaseTasklet
	artifactManager *manager.ArtifactManager
}

// NewArtifactExpirationTasklet constructor
func NewArtifactExpirationTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	artifactManager *manager.ArtifactManager,
	queueClient queue.Client,
	requestTopic string,
) *ArtifactExpirationTasklet {
	id := serverCfg.ID + "-artifact-expiration-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.ExpireArtifactsTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.ExpireArtifacts},
		Allocations:  make(map[uint64]*common.AntAllocation),
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &ArtifactExpirationTasklet{
		serverCfg:       serverCfg,
		artifactManager: artifactManager,
	}

	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.CommonConfig,
		queueClient,
		requestRegistry,
		requestTopic,
		serverCfg.GetRegistrationTopic(),
		registration,
		t,
	)
	return t
}

// TerminateContainer terminates container
func (t *ArtifactExpirationTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *ArtifactExpirationTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	taskResp = common.NewTaskResponse(req)
	taskResp.Status = common.COMPLETED
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute check if request can be executed
func (t *ArtifactExpirationTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute task request
func (t *ArtifactExpirationTasklet) Execute(
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	var queryContext *common.QueryContext
	if taskReq.AdminUser {
		queryContext = common.NewQueryContext(nil, "").WithAdmin()
	} else {
		queryContext = common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID)
	}

	expired, size, err := t.artifactManager.ExpireArtifacts(
		ctx,
		queryContext,
		t.serverCfg.DefaultArtifactExpiration,
		t.serverCfg.DefaultArtifactLimit)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":                 "ArtifactExpirationTasklet",
			"Request":                   taskReq,
			"DefaultArtifactExpiration": t.serverCfg.DefaultArtifactExpiration,
			"DefaultArtifactLimit":      t.serverCfg.DefaultArtifactLimit,
			"Error":                     err,
		}).Warnf("failed to expire artifacts")
		return buildTaskResponseWithError(taskReq, err)
	}
	logrus.WithFields(logrus.Fields{
		"Component":                 "ArtifactExpirationTasklet",
		"Request":                   taskReq,
		"DefaultArtifactExpiration": t.serverCfg.DefaultArtifactExpiration,
		"DefaultArtifactLimit":      t.serverCfg.DefaultArtifactLimit,
		"TotalExpired":              expired,
		"TotalSize":                 size,
		"Admin":                     queryContext.IsAdmin(),
	}).Info("expired artifacts")

	taskResp = common.NewTaskResponse(taskReq)
	taskResp.Status = common.COMPLETED
	taskResp.AddContext("DefaultArtifactExpiration", t.serverCfg.DefaultArtifactExpiration.String())
	taskResp.AddContext("DefaultArtifactLimit", t.serverCfg.DefaultArtifactLimit)
	taskResp.AddJobContext("TotalExpired", expired)
	taskResp.AddJobContext("TotalSize", size)
	taskResp.AddJobContext("Admin", queryContext.IsAdmin())
	return
}
