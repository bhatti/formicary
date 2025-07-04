package tasklet

import (
	"context"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"testing"
	"time"
)

func Test_ShouldTerminateArtifactExpirationTasklet(t *testing.T) {
	// GIVEN expirationTasklet
	expirationTasklet := newTestArtifactExpirationTasklet(&common.User{}, t)

	// WHEN terminating container
	_, err := expirationTasklet.TerminateContainer(context.Background(), nil)

	// THEN it should not fail
	require.Error(t, err)
}

func Test_ShouldPreExecuteArtifactExpirationTasklet(t *testing.T) {
	// GIVEN expirationTasklet
	expirationTasklet := newTestArtifactExpirationTasklet(&common.User{}, t)

	// WHEN pre-executing
	// THEN it should return true
	require.True(t, expirationTasklet.PreExecute(context.Background(), nil))
}

func Test_ShouldListArtifactExpirationTasklet(t *testing.T) {
	// GIVEN expirationTasklet
	expirationTasklet := newTestArtifactExpirationTasklet(&common.User{}, t)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := expirationTasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldExecuteArtifactExpirationTasklet(t *testing.T) {
	// GIVEN expirationTasklet
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "555"
	expirationTasklet := newTestArtifactExpirationTasklet(user, t)
	req := &common.TaskRequest{
		JobType:         "my-job",
		TaskType:        "my-task",
		JobRequestID:    "101",
		JobExecutionID:  "201",
		TaskExecutionID: "301",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		Script:          []string{"cmd"},
		ExecutorOpts:    common.NewExecutorOptions("name", common.Kubernetes),
	}

	// WHEN executing
	res, err := expirationTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, "", res.ErrorMessage)
	require.Equal(t, common.COMPLETED, res.Status)
	require.Equal(t, "1200h0m0s", res.TaskContext["DefaultArtifactExpiration"])
	require.Equal(t, 10000, res.TaskContext["DefaultArtifactLimit"])
	require.Equal(t, int64(5000), res.JobContext["TotalSize"])
	require.Equal(t, 50, res.JobContext["TotalExpired"])
}

func newTestArtifactExpirationTasklet(user *common.User, t *testing.T) *ArtifactExpirationTasklet {
	cfg := config.TestServerConfig()
	cfg.DefaultArtifactExpiration = time.Hour * 24 * 50
	cfg.DefaultArtifactLimit = 10000
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	require.NoError(t, err)
	requestRegistry := tasklet.NewRequestRegistry(
		&cfg.Common,
		metrics.New(),
	)
	artifactService, err := artifacts.NewStub(cfg.Common.S3)
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)

	mgr, err := manager.NewArtifactManager(
		cfg,
		logRepository,
		artifactRepository,
		artifactService)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		art := common.NewArtifact("bucket", ulid.Make().String(), "group", "kind", "101", "sha", 100)
		art.ID = ulid.Make().String()
		art.UserID = user.ID
		art.OrganizationID = user.OrganizationID
		art.ExpiresAt = time.Now().Add(time.Hour * -24 * time.Duration(i))
		_, _ = artifactRepository.Save(art)
	}

	expirationTasklet := NewArtifactExpirationTasklet(
		cfg,
		requestRegistry,
		mgr,
		queueClient,
		"requestTopic",
	)
	return expirationTasklet
}
