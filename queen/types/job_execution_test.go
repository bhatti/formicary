package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify table names for job-execution and context
func Test_ShouldJobExecutionTableNames(t *testing.T) {
	jobExec := testNewJobExecution("test-exec-job")
	require.Equal(t, "formicary_job_executions", jobExec.TableName())
	jobContext, _ := jobExec.AddContext("jk1", "jv1")
	require.Equal(t, "formicary_job_execution_context", jobContext.TableName())
}

// Validate happy path of Validate with proper job-execution
func Test_ShouldWithGoodJobExecution(t *testing.T) {
	jobExec := testNewJobExecution("test-exec-job")
	// WHEN validating valid job-execution
	err := jobExec.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
}

// Validate should fail if job state is empty
func Test_ShouldNotValidateJobExecutionWithoutState(t *testing.T) {
	req, err := NewJobRequestFromDefinition(NewJobDefinition("bad-name"))
	require.NoError(t, err)
	jobExec := NewJobExecution(req.ToInfo())
	jobExec.JobState = ""

	// WHEN validating without job-state
	err = jobExec.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobState is not specified")
}

// Validate should succeed if built without tasks
func Test_ShouldValidateJobExecutionWithoutTasks(t *testing.T) {
	req, err := NewJobRequestFromDefinition(NewJobDefinition("valid-name"))
	if err != nil {
		t.Fatalf("unexpected error %vv", err)
	}
	jobExec := NewJobExecution(req.ToInfo())
	// WHEN validating without tasks
	err = jobExec.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
}

func testNewJobExecution(name string) *JobExecution {
	job := newTestJobDefinition(name)
	req := newTestJobRequest(name)
	jobExec := NewJobExecution(req.ToInfo())
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", "jv2")
	for _, t := range job.Tasks {
		task := jobExec.AddTask(t)
		_, _ = task.AddContext("tk1", "v1")
		_, _ = task.AddContext("tk2", "v2")
	}
	return jobExec
}
