package types

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
)

// Verify table names for job-request
func Test_ShouldJobRequestTableNames(t *testing.T) {
	job := NewRequest()
	require.Equal(t, "formicary_job_requests", job.TableName())
	p, _ := job.AddParam("k", "v")
	require.Equal(t, "formicary_job_request_params", p.TableName())
}

// Verify params for job-request
func Test_ShouldAddParamsForJobRequest(t *testing.T) {
	job := NewRequest()
	_ = job.SetParamsJSON("")
	_, _ = job.AddParam("k", "v")
	require.Equal(t, `{"k":"v"}`, job.GetParamsJSON())
	require.Equal(t, "v", job.GetParam("k").Value)
	job.NameValueParams = nil
	require.Equal(t, "{}", job.GetParamsJSON())
}

// Validate happy path of Validate with proper job-request
func Test_ShouldWithGoodJobRequest(t *testing.T) {
	// Given job request
	job := newTestJobRequest("test-job")
	// WHEN validating good request

	// THEN it should not fail
	require.NoError(t, job.ValidateBeforeSave())
	require.NoError(t, job.ToInfo().Validate())
}

func Test_ShouldMatchTaskRequestState(t *testing.T) {
	job := newTestJobRequest("test-job")
	job.SetJobState(types.PENDING)
	require.Equal(t, job.JobState.CanRestart(), job.CanRestart())
	require.Equal(t, job.JobState.CanCancel(), job.CanCancel())
	require.Equal(t, job.JobState.Completed(), job.Completed())
	require.Equal(t, job.JobState.Failed(), job.Failed())
	require.Equal(t, job.JobState.Pending(), job.Pending())
	require.Equal(t, !job.JobState.IsTerminal(), job.NotTerminal())
	require.False(t, job.Running())
	require.False(t, job.Done())
	require.True(t, job.Waiting())
}

// Validate GetUserJobTypeKey
func Test_ShouldBuildUserJobTypeKey(t *testing.T) {
	// Given job request
	job := newTestJobRequest("test-job")
	job.UserID = "456"
	// WHEN building user-key

	// THEN it should return valid user-key
	require.Equal(t, "io.formicary.test.test-job", job.JobTypeAndVersion())
	job.JobVersion = "v1"
	require.Equal(t, "io.formicary.test.test-job:v1", job.JobTypeAndVersion())
	require.Equal(t, "v1", job.GetJobVersion())
	require.Equal(t, "456-io.formicary.test.test-job:v1", job.GetUserJobTypeKey())
	require.Equal(t, "456-io.formicary.test.test-job:v1", job.ToInfo().GetUserJobTypeKey())
	job.OrganizationID = "123"
	require.Equal(t, "123-io.formicary.test.test-job:v1", job.ToInfo().GetUserJobTypeKey())
}

// Test Accessors for execution-id
func Test_ShouldAccessExecutionIDForJobRequest(t *testing.T) {
	// Given job request
	job := newTestJobRequest("test-job")
	job.SetJobExecutionID("123")
	job.LastJobExecutionID = "456"
	// WHEN accessing execution-id

	// THEN it should match execution-ids
	job.ToInfo().SetJobExecutionID(job.JobExecutionID)
	require.Equal(t, job.JobExecutionID, job.ToInfo().GetJobExecutionID())
	require.Equal(t, job.JobDefinitionID, job.ToInfo().GetJobDefinitionID())
	require.Equal(t, job.LastJobExecutionID, job.ToInfo().GetLastJobExecutionID())
}

// Test Accessors for schedule-attempts
func Test_ShouldGetSetScheduleAttemptsForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN accessing schedule attempts
	// THEN it should return saved value
	require.Equal(t, 0, job.GetScheduleAttempts())
}

// Test Accessors for priority
func Test_ShouldGetSetPriorityForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN accessing priority
	// THEN it should return saved value
	require.Equal(t, 0, job.GetJobPriority())
}

// Test Setter/Accessors for state
func Test_ShouldGetSetStateForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN setting state
	job.SetJobState(types.PENDING)
	// THEN it should return saved value
	require.Equal(t, types.PENDING, job.GetJobState())
}

// Test Accessors for group
func Test_ShouldGetGroupForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job")
	// WHEN accessing group
	// THEN it should return saved value
	require.Equal(t, "", job.GetGroup())
	require.Equal(t, "", job.ToInfo().GetGroup())
}

// Test Accessors for organization-id
func Test_ShouldGetOrganizationIDForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN accessing org
	// THEN it should return saved value
	require.Equal(t, job.OrganizationID, job.GetOrganizationID())
}

// Test Accessors for user-id
func Test_ShouldGetUserIDForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN accessing user
	// THEN it should return saved value
	require.Equal(t, job.UserID, job.GetUserID())
}

// Test Accessors for cron-triggered
func Test_ShouldGetCronTriggeredForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	job.CronTriggered = true
	// WHEN accessing cron-triggered
	// THEN it should return saved value
	require.Equal(t, job.CronTriggered, job.GetCronTriggered())
}

// Test adding user/cron key
func Test_ShouldAddUpdateUserKeyFromScheduleIfCronJobForJobRequest(t *testing.T) {
	// Given job definition and request info
	b, err := os.ReadFile("../../fixtures/hello_world_scheduled.yaml")
	require.NoError(t, err)
	jobDef, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	job, err := NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	// WHEN accessing cron-triggered
	// THEN it should set it
	job.UpdateUserKeyFromScheduleIfCronJob(jobDef)
	job.UpdateUserKeyFromScheduleIfCronJob(NewJobDefinition(""))
}

// Test Accessors for created-at
func Test_ShouldGetCreatedAtForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job")
	// WHEN accessing created-at
	// THEN it should return saved value
	require.Equal(t, job.CreatedAt, job.ToInfo().GetCreatedAt())
	require.NotEqual(t, "", job.UpdatedAtString())
	require.NotEqual(t, "", job.String())
}

// Test Accessors for scheduled-at
func Test_ShouldGetScheduledAtForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job").ToInfo()
	// WHEN accessing scheduled-at
	// THEN it should return saved value
	require.Equal(t, job.ScheduledAt, job.GetScheduledAt())
}

// Test Accessors for retried
func Test_ShouldGetRetriedForJobRequest(t *testing.T) {
	// Given job request info
	job := newTestJobRequest("test-job")
	// WHEN accessing retried
	// THEN it should return saved value
	require.Equal(t, 0, job.GetRetried())
	require.Equal(t, 1, job.IncrRetried())
	require.Equal(t, 0, job.ToInfo().GetRetried())
	require.Equal(t, 1, job.ToInfo().IncrRetried())
}

// Validate JSON serialization
func Test_ShouldSerializeJSONJobRequest(t *testing.T) {
	// Given job request
	job := newTestJobRequest("test-job")
	job.ScheduledAt = time.Now().Add(1 * time.Hour)
	_, err := json.Marshal(job)
	// THEN it should not fail
	require.NoError(t, err)

	j := `
{
  "id": "101",
  "user_key": "ukey",
  "job_definition_id": "job-def-id",
  "organization_id": "123",
  "user_id": "456",
  "platform": "Ubuntu",
  "job_type": "test-job",
  "job_state": "PENDING",
  "job_group": "mygroup",
  "job_priority": 100,
  "scheduled_at": "2025-06-15T00:00:00.0-00:00",
  "params": {
    "k1": "v1",
    "k2": "v2"
  }
}
`
	err = json.Unmarshal([]byte(j), &job)
	// THEN it should not fail
	require.NoError(t, err)

}

// Validate should fail if job definition-id is empty
func Test_ShouldJobRequestValidateWithoutJobDefinitionID(t *testing.T) {
	// Given job request
	job, err := NewJobRequestFromDefinition(NewJobDefinition(""))
	require.NoError(t, err)
	// WHEN validating without job-definition-id
	err = job.ValidateBeforeSave()

	// THEN it should not fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobDefinitionID is not specified")
}

// Validate should fail if job type is empty
func Test_ShouldNotValidateJobRequestWithoutType(t *testing.T) {
	// Given job request
	job, err := NewJobRequestFromDefinition(NewJobDefinition(""))
	require.NoError(t, err)
	job.JobDefinitionID = "123"
	// WHEN validating without job-type

	// THEN it should not fail
	require.Error(t, job.ValidateBeforeSave())
	require.Contains(t, job.ValidateBeforeSave().Error(), "jobType is not specified")
	require.Contains(t, job.ToInfo().Validate().Error(), "jobType is not specified")
}

// Validate should fail if built without state
func Test_ShouldNotValidateJobRequestWithoutState(t *testing.T) {
	// Given job request
	job, err := NewJobRequestFromDefinition(NewJobDefinition("task-type"))
	require.NoError(t, err)
	job.JobDefinitionID = "some-id"
	job.JobState = ""
	// WHEN validating without state

	// THEN it should not fail
	require.Error(t, job.ValidateBeforeSave())
	require.Contains(t, job.ValidateBeforeSave().Error(), "jobState is not specified")
	require.Contains(t, job.ToInfo().Validate().Error(), "jobState is not specified")
}

func newTestJobRequest(name string) *JobRequest {
	// Given job request
	job := newTestJobDefinition(name)
	job.ID = "some-id"
	req, _ := NewJobRequestFromDefinition(job)
	req.ClearParams()
	_, _ = req.AddParam("k1", "jv1")
	_, _ = req.AddParam("k2", "jv2")
	return req
}
