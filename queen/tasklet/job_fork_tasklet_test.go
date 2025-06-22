package tasklet

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"testing"
)

func Test_ShouldTerminateForkTasklet(t *testing.T) {
	// GIVEN forkTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)

	// WHEN terminating container
	_, err := forkTasklet.TerminateContainer(context.Background(), nil)

	// THEN it should not fail
	require.Error(t, err)
}

func Test_ShouldPreExecuteForkTasklet(t *testing.T) {
	// GIVEN forkTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)

	// WHEN pre-executing
	// THEN it should return true
	require.True(t, forkTasklet.PreExecute(context.Background(), nil))
}

func Test_ShouldListForkTasklet(t *testing.T) {
	// GIVEN tasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	tasklet := newTestForkTasklet(jobManager)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := tasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldExecuteForkTasklet(t *testing.T) {
	// GIVEN tasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	tasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "555"
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
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"

	// WHEN executing without job
	res, err := tasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing with valid job
	job := repository.NewTestJobDefinition(user, "my-job")
	job, err = jobManager.SaveJobDefinition(common.NewQueryContext(user, ""), job)
	require.NoError(t, err)
	require.Equal(t, user.ID, job.UserID)
	require.Equal(t, user.OrganizationID, job.OrganizationID)
	res, err = tasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, "", res.ErrorMessage)
	require.Equal(t, common.COMPLETED, res.Status)
}

func newTestForkTasklet(jobManager *manager.JobManager) *JobForkTasklet {
	cfg := config.TestServerConfig()
	queueClient := queue.NewStubClient(&cfg.Common)
	requestRegistry := tasklet.NewRequestRegistry(
		&cfg.Common,
		metrics.New(),
	)

	tasklet := NewJobForkTasklet(
		cfg,
		requestRegistry,
		jobManager,
		queueClient,
		"requestTopic",
	)
	return tasklet
}
