package types

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

// Verify table names for job-execution and context
func Test_ShouldJobExecutionTableNames(t *testing.T) {
	jobExec := testNewJobExecution("test-exec-job")
	require.Equal(t, "formicary_job_executions", jobExec.TableName())
	jobContext, _ := jobExec.AddContext("jk1", "jv1")
	require.Equal(t, "formicary_job_execution_context", jobContext.TableName())
}

// Test Accessors for created-at
func Test_ShouldReturnStringForJobExecution(t *testing.T) {
	// Given job execution
	jobExec := testNewJobExecution("test-exec-job")
	// WHEN accessing created-at
	// THEN it should return saved value
	require.Equal(t, jobExec.StartedAt, jobExec.GetCreatedAt())
	require.NotEqual(t, "", jobExec.String())
}

func Test_ShouldMatchJobExecutionState(t *testing.T) {
	// Given job execution
	jobExec := testNewJobExecution("test-exec-job")
	jobExec.Tasks[0].Stdout = []string{"one", "two", "three"}
	jobExec.JobState = common.PENDING
	require.Equal(t, jobExec.JobState.CanRestart(), jobExec.CanRestart())
	require.Equal(t, jobExec.JobState.CanCancel(), jobExec.CanCancel())
	require.Equal(t, jobExec.JobState.Completed(), jobExec.Completed())
	require.Equal(t, jobExec.JobState.Failed(), jobExec.Failed())
	require.Equal(t, !jobExec.JobState.IsTerminal(), jobExec.NotTerminal())
	require.Equal(t, 5, len(jobExec.Stdout()))
}

// Validate GetUserJobTypeKey
func Test_ShouldBuildUserJobTypeKeyForJobExecution(t *testing.T) {
	// Given job request
	job := testNewJobExecution("test-exec-job")
	job.UserID = "456"
	// WHEN building user-key

	// THEN it should return valid user-key
	require.Equal(t, "io.formicary.test.test-exec-job", job.JobTypeAndVersion())
	job.JobVersion = "v1"
	require.Equal(t, "io.formicary.test.test-exec-job:v1", job.JobTypeAndVersion())
	require.Equal(t, "v1", job.GetJobVersion())
	require.Equal(t, "456-io.formicary.test.test-exec-job:v1", job.GetUserJobTypeKey())
	require.Equal(t, job.JobType, job.GetJobType())
	require.Equal(t, job.JobState, job.GetJobState())
	require.Equal(t, job.OrganizationID, job.GetOrganizationID())
	require.Equal(t, job.UserID, job.GetUserID())
	require.Equal(t, job.StartedAt, job.GetScheduledAt())
}

// Test calculate duration
func Test_ShouldCalculateJobExecutionDuration(t *testing.T) {
	job := testNewJobExecution("test-exec-job")
	job.AddTask(NewTaskDefinition("type", common.Kubernetes))
	require.NotEqual(t, "", job.ElapsedDuration())
	require.True(t, job.ElapsedMillis() <= 1)
	ended := time.Now().Add(time.Hour)
	for _, task := range job.Tasks {
		task.TaskState = common.COMPLETED
		task.EndedAt = &ended
	}
	job.EndedAt = &ended
	require.Contains(t, job.ElapsedDuration(), "1h0m")
	require.Equal(t, int64(3600000), job.ElapsedMillis())
	require.Equal(t, int64(14400), job.ExecutionCostSecs())
	require.Equal(t, float64(0), job.CostFactor())
	job.JobState = common.EXECUTING
	require.NotEqual(t, "", job.ElapsedDuration())
	require.Equal(t, int64(14400), job.ExecutionCostSecs())
}

// Validate happy path of Validate with proper job-execution
func Test_ShouldWithGoodJobExecution(t *testing.T) {
	jobExec := testNewJobExecution("test-exec-job")
	jobExec.AddTask(NewTaskDefinition("type2", ""))
	jobExec.AddTask(NewTaskDefinition("type2", ""))
	_, _ = jobExec.AddContext("c1", "c2")
	// WHEN validating valid job-execution
	err := jobExec.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
	require.Contains(t, jobExec.Methods(), "DOCKER")
	require.Contains(t, jobExec.Methods(), "KUBERNETES")
	require.Contains(t, jobExec.TasksString(), "TaskType=task1")
	require.Equal(t, "type2", jobExec.GetLastTask().TaskType)

	require.Nil(t, jobExec.GetContext("c1x"))
	require.NotNil(t, jobExec.GetContext("c1"))
	require.NotNil(t, jobExec.DeleteContext("c1"))
	require.Nil(t, jobExec.DeleteContext("c1"))
}

// Validate happy path of Validate with proper job-execution
func Test_ShouldGetFailedTaskError(t *testing.T) {
	jobExec := testNewJobExecution("test-exec-job")
	// WHEN getting failed task error without failed task
	_, _, err := jobExec.GetFailedTaskError()

	// THEN it should not fail
	require.NoError(t, err)
	// WHEN getting failed task error with failed task
	jobExec.Tasks[0].TaskState = common.CANCELLED
	_, _, err = jobExec.GetFailedTaskError()
	require.Error(t, err)
}

// Validate should fail if job state is empty
func Test_ShouldNotValidateJobExecutionWithoutState(t *testing.T) {
	req, err := NewJobRequestFromDefinition(NewJobDefinition("bad-name"))
	require.NoError(t, err)
	jobExec := NewJobExecution(req.ToInfo())
	jobExec.JobState = ""
	require.Nil(t, jobExec.GetLastTask())

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

// Test Accessors for priority
func Test_ShouldGetSetPriorityForJobExecution(t *testing.T) {
	// Given job execution
	job := testNewJobExecution("test-exec-job")
	// WHEN accessing priority
	// THEN it should return saved value
	require.Equal(t, -1, job.GetJobPriority())
	require.Equal(t, uint64(0), job.GetID())
}

func testNewJobExecution(name string) *JobExecution {
	job := newTestJobDefinition(name)
	req := newTestJobRequest(name)
	jobExec := NewJobExecution(req.ToInfo())
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", "jv2")
	for i, t := range job.Tasks {
		jobExec.AddTasks(t)
		jobExec.Tasks[i].Stdout = []string{"test"}
		_, _ = jobExec.Tasks[i].AddContext("tk1", "v1")
		_, _ = jobExec.Tasks[i].AddContext("tk2", "v2")
	}
	_ = jobExec.AfterLoad()
	return jobExec
}
