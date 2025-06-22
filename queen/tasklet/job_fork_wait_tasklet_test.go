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
	"plexobject.com/formicary/queen/types"
	"testing"
	"time"
)

func Test_ShouldTerminateForkWaitTasklet(t *testing.T) {
	// GIVEN waitTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	waitTasklet, _, _ := newTestForkWaitTasklet(t, user, jobManager)

	// WHEN terminating container
	_, err := waitTasklet.TerminateContainer(context.Background(), nil)

	// THEN it should not fail
	require.Error(t, err)
}

func Test_ShouldPreExecuteForkWaitTasklet(t *testing.T) {
	// GIVEN waitTasklet
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	jobManager := manager.AssertTestJobManager(nil, t)
	waitTasklet, _, _ := newTestForkWaitTasklet(t, user, jobManager)

	// WHEN pre-executing
	// THEN it should return true
	require.True(t, waitTasklet.PreExecute(context.Background(), nil))
}

func Test_ShouldListForkWaitTasklet(t *testing.T) {
	// GIVEN waitTasklet
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	jobManager := manager.AssertTestJobManager(nil, t)
	waitTasklet, _, _ := newTestForkWaitTasklet(t, user, jobManager)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := waitTasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)

}

func Test_ShouldExecuteForkWaitTasklet(t *testing.T) {
	// GIVEN waitTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "555"
	waitTasklet, jobReq, jobExec := newTestForkWaitTasklet(t, user, jobManager)
	require.NotEmpty(t, jobReq.ID)
	require.NotEmpty(t, jobExec.ID)
	req := &common.TaskRequest{
		JobType:         "io.formicary.test.my-job",
		TaskType:        jobExec.Tasks[0].TaskType,
		JobRequestID:    jobReq.ID,
		JobExecutionID:  jobExec.ID,
		TaskExecutionID: jobExec.Tasks[0].ID,
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		Script:          []string{"cmd"},
		ExecutorOpts:    common.NewExecutorOptions("name", common.Kubernetes),
		Variables:       make(map[string]common.VariableValue),
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.AddVariable("log_interval", "1s", false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// WHEN executing without AwaitForkedTasks
	res, err := waitTasklet.Execute(ctx, req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)
	require.Contains(t, res.ErrorMessage, "await_forked_tasks")

	req.ExecutorOpts.AwaitForkedTasks = []string{"task1"}
	// WHEN executing without task1.ForkedJobID
	res, err = waitTasklet.Execute(ctx, req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)
	require.Contains(t, res.ErrorMessage, "task1.ForkedJobI")

	// WHEN executing with AwaitForkedTasks and ForkedJobID
	req.AddVariable("task1.ForkedJobID", req.JobRequestID, false)
	res, err = waitTasklet.Execute(ctx, req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, "", res.ErrorMessage)
	require.Equal(t, common.COMPLETED, res.Status)
	require.Equal(t, []string{jobReq.ID}, res.TaskContext["RequestIDs"])
	require.Equal(t, 1, res.TaskContext["TotalRequests"])
}

func newTestForkWaitTasklet(
	t *testing.T,
	user *common.User,
	jobManager *manager.JobManager,
) (waitTasklet *JobForkWaitTasklet, req *types.JobRequest, exec *types.JobExecution) {
	cfg := config.TestServerConfig()
	queueClient := queue.NewStubClient(&cfg.Common)
	requestRegistry := tasklet.NewRequestRegistry(
		&cfg.Common,
		metrics.New(),
	)
	jobExecRepo, err := repository.NewTestJobExecutionRepository()
	require.NoError(t, err)

	waitTasklet = NewJobForkWaitTasklet(
		cfg,
		requestRegistry,
		jobManager,
		queueClient,
		"requestTopic",
	)
	qc := common.NewQueryContext(user, "")
	req, exec, err = repository.NewTestJobExecution(qc, "my-job")
	require.NoError(t, err)

	exec, err = jobExecRepo.Save(exec)
	require.NoError(t, err)

	req.JobExecutionID = exec.ID
	err = jobManager.SetJobRequestReadyToExecute(req.ID, exec.ID, "")
	require.NoError(t, err)

	exec.JobState = common.COMPLETED
	err = jobManager.FinalizeJobRequestAndExecutionState(qc, qc.User, &types.JobDefinition{}, req, exec, common.READY, 0, 0)
	require.NoError(t, err)
	//_ = jobManager.UpdateJobRequestState(qc, req, common.READY, common.COMPLETED, "", "", 0, 0, false)

	return
}
