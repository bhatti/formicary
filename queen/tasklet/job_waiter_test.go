// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklet

import (
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// BuildTaskResponse without output_variables should copy all child context variables.
func Test_BuildResponse_WithoutOutputMap_CopiesAll(t *testing.T) {
	jobManager := manager.AssertTestJobManager(nil, t)
	req, exec, ctxVars := setupWaiterTest(t, jobManager, nil)

	taskReq := buildWaiterTaskRequest(req, exec)
	waiter := buildCompletedWaiter(t, jobManager, taskReq, req)

	resp, err := waiter.BuildTaskResponse(taskReq)
	require.NoError(t, err)

	for k, expected := range ctxVars {
		v, ok := resp.TaskContext[k]
		require.True(t, ok, "expected context key %s to exist", k)
		require.Equal(t, expected, v)
	}
}

// BuildTaskResponse with output_variables should only return mapped variables, renamed.
func Test_BuildResponse_WithOutputMap_MapsNames(t *testing.T) {
	sw := &common.SubWorkflowConfig{
		OutputVariables: []common.SubWorkflowVariable{
			{Name: "child_row_count", Value: "etl_row_count"},
		},
	}
	jobManager := manager.AssertTestJobManager(nil, t)
	req, exec, _ := setupWaiterTest(t, jobManager, sw)

	taskReq := buildWaiterTaskRequest(req, exec)
	taskReq.ExecutorOpts.SubWorkflow = sw
	waiter := buildCompletedWaiter(t, jobManager, taskReq, req)

	resp, err := waiter.BuildTaskResponse(taskReq)
	require.NoError(t, err)

	// Mapped key should exist under the parent name
	_, ok := resp.TaskContext["etl_row_count"]
	require.True(t, ok, "expected parent context key etl_row_count")
	// Original child key should NOT exist
	_, ok = resp.TaskContext["child_row_count"]
	require.False(t, ok, "original child key should not be in parent context")
}

// BuildTaskResponse with output_variables should exclude unmapped child context variables.
func Test_BuildResponse_WithOutputMap_IgnoresUnmapped(t *testing.T) {
	sw := &common.SubWorkflowConfig{
		OutputVariables: []common.SubWorkflowVariable{
			{Name: "child_row_count", Value: "etl_row_count"},
		},
	}
	jobManager := manager.AssertTestJobManager(nil, t)
	req, exec, _ := setupWaiterTest(t, jobManager, sw)

	taskReq := buildWaiterTaskRequest(req, exec)
	taskReq.ExecutorOpts.SubWorkflow = sw
	waiter := buildCompletedWaiter(t, jobManager, taskReq, req)

	resp, err := waiter.BuildTaskResponse(taskReq)
	require.NoError(t, err)

	// "other_var" is in child context but NOT in output_variables — must be excluded
	_, ok := resp.TaskContext["other_var"]
	require.False(t, ok, "unmapped variable other_var must not be promoted to parent context")
}

// BuildTaskResponse with empty output_variables should copy all child context variables.
func Test_BuildResponse_WithEmptyOutputMap_CopiesAll(t *testing.T) {
	sw := &common.SubWorkflowConfig{}
	jobManager := manager.AssertTestJobManager(nil, t)
	req, exec, ctxVars := setupWaiterTest(t, jobManager, sw)

	taskReq := buildWaiterTaskRequest(req, exec)
	taskReq.ExecutorOpts.SubWorkflow = sw
	waiter := buildCompletedWaiter(t, jobManager, taskReq, req)

	resp, err := waiter.BuildTaskResponse(taskReq)
	require.NoError(t, err)

	for k, expected := range ctxVars {
		v, ok := resp.TaskContext[k]
		require.True(t, ok, "expected context key %s to exist", k)
		require.Equal(t, expected, v)
	}
}

/////////////////////////////////////////// HELPERS ////////////////////////////////////////////

func setupWaiterTest(
	t *testing.T,
	jobManager *manager.JobManager,
	sw *common.SubWorkflowConfig,
) (*types.JobRequest, *types.JobExecution, map[string]interface{}) {
	t.Helper()

	jobExecRepo, err := repository.NewTestJobExecutionRepository()
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	req, exec, err := repository.NewTestJobExecution(qc, "waiter-test-job")
	require.NoError(t, err)

	// Add context variables to the execution
	_, _ = exec.AddContext("child_row_count", "42")
	_, _ = exec.AddContext("other_var", "ignored")

	exec, err = jobExecRepo.Save(exec)
	require.NoError(t, err)

	req.JobExecutionID = exec.ID
	err = jobManager.SetJobRequestReadyToExecute(req.ID, exec.ID, "")
	require.NoError(t, err)

	exec.JobState = common.COMPLETED
	err = jobManager.FinalizeJobRequestAndExecutionState(
		qc, qc.User, &types.JobDefinition{}, req, exec, common.READY, 0, 0)
	require.NoError(t, err)

	ctxVars := map[string]interface{}{
		"child_row_count": "42",
		"other_var":       "ignored",
	}
	return req, exec, ctxVars
}

func buildWaiterTaskRequest(req *types.JobRequest, exec *types.JobExecution) *common.TaskRequest {
	taskReq := &common.TaskRequest{
		JobType:         req.JobType,
		TaskType:        exec.Tasks[0].TaskType,
		JobRequestID:    req.ID,
		JobExecutionID:  exec.ID,
		TaskExecutionID: exec.Tasks[0].ID,
		UserID:          req.UserID,
		OrganizationID:  req.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("name", common.AwaitForkedJob),
		Variables:       map[string]common.VariableValue{},
	}
	taskReq.ExecutorOpts.AwaitForkedTasks = []string{"fork-task"}
	taskReq.Variables["fork-task"+forkedJobIDSuffix] = common.NewVariableValue(req.ID, false)
	return taskReq
}

func buildCompletedWaiter(
	t *testing.T,
	jobManager *manager.JobManager,
	taskReq *common.TaskRequest,
	req *types.JobRequest,
) *JobWaiter {
	t.Helper()
	ctx := t.Context()
	waiter, err := NewJobWaiter(ctx, "test-ant", jobManager, taskReq)
	require.NoError(t, err)
	// Simulate the child being completed so Poll returns true immediately
	waiter.Lock()
	waiter.requests[req.ID] = req
	waiter.Unlock()
	return waiter
}
