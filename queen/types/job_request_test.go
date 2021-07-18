package types

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Verify table names for job-request
func Test_ShouldJobRequestTableNames(t *testing.T) {
	job := newTestJobRequest("test-job")
	require.Equal(t, "formicary_job_requests", job.TableName())
}

// Validate happy path of Validate with proper job-request
func Test_ShouldWithGoodJobRequest(t *testing.T) {
	// Given job request
	job := newTestJobRequest("test-job")
	// WHEN validating good request
	err := job.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
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
  "id": 101,
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
func Test_ShouldJobRequestValidateWithoutType(t *testing.T) {
	// Given job request
	job, err := NewJobRequestFromDefinition(NewJobDefinition(""))
	job.JobDefinitionID = "123"
	require.NoError(t, err)
	// WHEN validating without job-type
	err = job.ValidateBeforeSave()

	// THEN it should not fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobType is not specified")
}

// Validate should fail if built without state
func Test_ShouldJobRequestValidateWithoutTasks(t *testing.T) {
	// Given job request
	job, err := NewJobRequestFromDefinition(NewJobDefinition("task-type"))
	require.NoError(t, err)
	job.JobDefinitionID = "some-id"
	job.JobState = ""
	// WHEN validating without state
	err = job.ValidateBeforeSave()

	// THEN it should not fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobState is not specified")
}

func newTestJobRequest(name string) *JobRequest {
	// Given job request
	job := newTestJobDefinition(name)
	job.ID = "some-id"
	req, _ := NewJobRequestFromDefinition(job)
	_, _ = req.AddParam("k1", "jv1")
	_, _ = req.AddParam("k2", "jv2")
	return req
}
