package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"

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
func Test_ShouldValidateTaskExecutionHappyPath(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("task-type", common.Shell))
	taskExec.JobExecutionID = "xx"
	// WHEN validating task-execution a valid task execution
	err := taskExec.ValidateBeforeSave()
	// THEN it should not fail
	require.NoError(t, err)
}

// Test validate without task-type
func Test_ShouldNotValidateTaskExecutionWithoutTaskType(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	taskExec.JobExecutionID = "xx"
	// WHEN validating task-execution without type
	err := taskExec.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "taskType is not specified")
}

// Test string method
func Test_ShouldMatchTaskExecutionString(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	require.Contains(t, taskExec.String(), "TaskType")
}

// Test calculate duration
func Test_ShouldCalculateTaskExecutionDuration(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	require.NotEqual(t, "", taskExec.ElapsedDuration())
	ended := time.Now().Add(time.Hour)
	taskExec.EndedAt = &ended
	require.NotEqual(t, "", taskExec.ElapsedDuration())
}

// Test calculate cost
func Test_ShouldCalculateTaskExecutionCost(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	require.Equal(t, int64(0), taskExec.ExecutionCostSecs())
	taskExec.StartedAt = time.Now().Add(-1 * time.Hour)
	ended := time.Now().Add(time.Hour)
	taskExec.EndedAt = &ended
	require.Equal(t, int64(7200), taskExec.ExecutionCostSecs())
}

func Test_ShouldMatchTaskExecutionState(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	taskExec.SetStatus(common.PENDING)
	require.Equal(t, taskExec.TaskState.CanRestart(), taskExec.CanRestart())
	require.Equal(t, taskExec.TaskState.CanCancel(), taskExec.CanCancel())
	require.Equal(t, taskExec.TaskState.Completed(), taskExec.Completed())
	require.Equal(t, taskExec.TaskState.Failed(), taskExec.Failed())
	require.Equal(t, !taskExec.TaskState.IsTerminal(), taskExec.NotTerminal())
}

func Test_ShouldStringifyTaskExecutionContextState(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	taskExec.DeleteContext("k1")
	_, _ = taskExec.AddContext("k1", "v1")
	_, _ = taskExec.AddContext("k1", "v1")
	_, _ = taskExec.AddContext("k2", "v2")
	_, _ = taskExec.AddContext("k2", nil)
	_ = taskExec.DeleteContext("k2")
	require.Equal(t, "v1", taskExec.GetContext("k1").Value)
	require.Contains(t, taskExec.ContextString(), "k1")
	require.NoError(t, taskExec.AfterLoad())
}

func Test_ShouldAddArtifactToTaskExecution(t *testing.T) {
	taskExec := NewTaskExecution(NewTaskDefinition("", common.Shell))
	taskExec.AddArtifact(&common.Artifact{})
}

func Test_ShouldEqualTaskExecution(t *testing.T) {
	taskExec1 := NewTaskExecution(NewTaskDefinition("type1", common.Shell))
	_, _ = taskExec1.AddContext("k1", "v1")
	taskExec2 := NewTaskExecution(NewTaskDefinition("type1", common.Shell))
	_, _ = taskExec2.AddContext("k1", "v1")
	_, _ = taskExec2.AddContext("k2", "v2")
	taskExec3 := NewTaskExecution(NewTaskDefinition("type2", common.Shell))
	_, _ = taskExec3.AddContext("k1", "v1")
	taskExec4 := NewTaskExecution(NewTaskDefinition("", common.Shell))
	require.Error(t, taskExec1.Equals(nil))
	require.Error(t, taskExec1.Equals(taskExec4)) // taskExec4 doesn't have type
	require.Error(t, taskExec4.Equals(taskExec1)) // taskExec4 doesn't have type
	require.Error(t, taskExec1.Equals(taskExec3)) // task-type doesn't match
	require.Error(t, taskExec1.Equals(taskExec2)) // context size doesn't match
	_, _ = taskExec1.AddContext("k2", "blah")
	require.Error(t, taskExec1.Equals(taskExec2)) // context contents doesn't match
	require.NoError(t, taskExec1.Equals(taskExec1))
	require.Error(t, taskExec1.Equals(taskExec2)) // context contents doesn't match
	_, _ = taskExec1.AddContext("k2", "v2")
	require.NoError(t, taskExec1.Equals(taskExec2))
}

