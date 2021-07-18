package types

import (
	"github.com/stretchr/testify/require"
	"testing"

	common "plexobject.com/formicary/internal/types"
)

// Verify table names for task-execution and context
func Test_ShouldTaskExecutionTableNames(t *testing.T) {
	task := NewTaskExecution(NewTaskDefinition("task-tupe", common.Shell))
	require.Equal(t, "formicary_task_executions", task.TableName())
	taskContext, _ := task.AddContext("k1", "v1")
	require.Equal(t, "formicary_task_execution_context", taskContext.TableName())
}

// Validate task-execution with proper initialization
func Test_ShouldTaskExecutionHappyPath(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("task-type", common.Shell))
	taskExec.JobExecutionID = "xx"
	// WHEN validating task-execution a valid task execution
	err := taskExec.ValidateBeforeSave()
	// THEN it should not fail
	require.NoError(t, err)
}

// Test validate without task-type
func Test_ShouldTaskExecutionWithoutTaskType(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	taskExec.JobExecutionID = "xx"
	// WHEN validating task-execution without type
	err := taskExec.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "taskType is not specified")
}

