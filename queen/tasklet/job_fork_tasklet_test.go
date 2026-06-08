package tasklet

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
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
	// GIVEN forkTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	req := &common.TaskRequest{
		ExecutorOpts: common.NewExecutorOptions("name", common.Kubernetes),
	}
	// WHEN listing containers
	_, err := forkTasklet.ListContainers(context.Background(), req)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldExecuteForkTasklet(t *testing.T) {
	// GIVEN forkTasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
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
	res, err := forkTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should fail
	require.Equal(t, common.FAILED, res.Status)

	// WHEN executing with valid job
	job := repository.NewTestJobDefinition(user, "my-job")
	job, err = jobManager.SaveJobDefinition(common.NewQueryContext(user, ""), job)
	require.NoError(t, err)
	require.Equal(t, user.ID, job.UserID)
	require.Equal(t, user.OrganizationID, job.OrganizationID)
	res, err = forkTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	// THEN it should not fail
	require.Equal(t, "", res.ErrorMessage)
	require.Equal(t, common.COMPLETED, res.Status)
}

func newTestForkTasklet(jobManager *manager.JobManager) *JobForkTasklet {
	cfg := config.TestServerConfig()
	queueClient, _ := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
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

func Test_ShouldForkWithInputMapResolvesTemplates(t *testing.T) {
	// GIVEN a fork tasklet and a saved job definition
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "555"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "my-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "fork-task",
		JobRequestID:    "parent-101",
		JobExecutionID:  "parent-201",
		TaskExecutionID: "parent-301",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.ForkJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.ExecutorOpts.SubWorkflow = &common.SubWorkflowConfig{
		InputParams: []common.SubWorkflowVariable{
			{Name: "child_param", Value: "{{ .parent_value }}"},
		},
	}
	req.Variables["parent_value"] = common.NewVariableValue("resolved-value", false)

	// WHEN executing with sub_workflow.input_variables
	res, err := forkTasklet.Execute(context.Background(), req)

	// THEN it should succeed
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, res.Status)
	require.NotEmpty(t, res.TaskContext["fork-task"+forkedJobIDSuffix])
}

func Test_ShouldForkWithInputMapErrorOnBadTemplate(t *testing.T) {
	// GIVEN a fork tasklet
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "555"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "my-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "fork-task",
		JobRequestID:    "parent-102",
		JobExecutionID:  "parent-202",
		TaskExecutionID: "parent-302",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.ForkJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.ExecutorOpts.SubWorkflow = &common.SubWorkflowConfig{
		InputParams: []common.SubWorkflowVariable{
			{Name: "child_param", Value: "{{ .unclosed_template"},
		},
	}

	// WHEN executing with invalid template expression
	res, err := forkTasklet.Execute(context.Background(), req)

	// THEN it should return FAILED response
	require.NoError(t, err)
	require.Equal(t, common.FAILED, res.Status)
	require.Contains(t, res.ErrorMessage, "input_params")
}

func Test_ShouldForkSetsAlwaysCascadeCancel(t *testing.T) {
	// GIVEN a fork tasklet with a sub_workflow definition
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "777"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "my-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "fork-task",
		JobRequestID:    "parent-103",
		JobExecutionID:  "parent-203",
		TaskExecutionID: "parent-303",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.ForkJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.ExecutorOpts.SubWorkflow = &common.SubWorkflowConfig{}

	// WHEN executing a FORK_JOB with sub_workflow
	res, err := forkTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, res.Status)

	// THEN the child request must always have cascade_cancel=true
	childID := res.TaskContext["fork-task"+forkedJobIDSuffix].(string)
	childReq, err := jobManager.GetJobRequest(qc, childID)
	require.NoError(t, err)
	require.True(t, childReq.CascadeCancel, "cascade_cancel must always be true for forked children")
}

func Test_ShouldForkWithNoInputMapForwardsNothing(t *testing.T) {
	// GIVEN a fork tasklet with sub_workflow but no input_map
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "888"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "my-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "fork-task",
		JobRequestID:    "parent-104",
		JobExecutionID:  "parent-204",
		TaskExecutionID: "parent-304",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.ForkJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.ExecutorOpts.SubWorkflow = &common.SubWorkflowConfig{} // no input_variables
	req.Variables["sensitive_key"] = common.NewVariableValue("should-not-forward", false)

	// WHEN executing without input_variables
	res, err := forkTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, res.Status)

	// THEN the child request should have no params from the parent (only ForkedJob marker)
	childID := res.TaskContext["fork-task"+forkedJobIDSuffix].(string)
	childReq, err := jobManager.GetJobRequest(qc, childID)
	require.NoError(t, err)
	require.Nil(t, childReq.GetParam("sensitive_key"), "parent variable must not be forwarded when input_variables is absent")
}

func Test_ShouldForkSecretNotAccessibleViaInputMap(t *testing.T) {
	// GIVEN a fork tasklet with input_variables that references a secret variable
	jobManager := manager.AssertTestJobManager(nil, t)
	forkTasklet := newTestForkTasklet(jobManager)
	user := common.NewUser("", "user@formicary.io", "name", "", acl.NewRoles(""))
	user.ID = "999"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "my-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "fork-task",
		JobRequestID:    "parent-105",
		JobExecutionID:  "parent-205",
		TaskExecutionID: "parent-305",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.ForkJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.ForkJobType = "io.formicary.test.my-job"
	req.ExecutorOpts.SubWorkflow = &common.SubWorkflowConfig{
		InputParams: []common.SubWorkflowVariable{
			{Name: "child_token", Value: "{{ .api_token }}"},
		},
	}
	// Mark api_token as secret — it must NOT be accessible via template
	req.Variables["api_token"] = common.NewVariableValue("super-secret", true)

	// WHEN executing with input_variables that references a secret
	res, err := forkTasklet.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, res.Status)

	// THEN child_token should not contain the secret value (template resolved against masked vars)
	childID := res.TaskContext["fork-task"+forkedJobIDSuffix].(string)
	childReq, err := jobManager.GetJobRequest(qc, childID)
	require.NoError(t, err)
	p := childReq.GetParam("child_token")
	if p != nil {
		require.NotEqual(t, "super-secret", fmt.Sprintf("%v", p.Value), "secret must not be accessible via input_variables template")
	}
}
