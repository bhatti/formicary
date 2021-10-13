package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateTaskResponse(t *testing.T) {
	req, res := newTestTaskResponse()
	require.Equal(t, req.JobRequestID, res.JobRequestID)
	require.NotEqual(t, "", res.String())
	require.Error(t, res.Validate())
	res.AntID = "103"
	res.Host = "localhost"
	require.NoError(t, res.Validate())
}

func Test_ShouldAddContextForTaskResponse(t *testing.T) {
	_, res := newTestTaskResponse()
	res.AddContext("name", "value")
	res.AddJobContext("name", "value")
	res.AddArtifact(&Artifact{})
}

func Test_ShouldAdditionalErrorForTaskResponse(t *testing.T) {
	_, res := newTestTaskResponse()
	res.AdditionalError("", false)
	require.Equal(t, COMPLETED, res.Status)
	res.AdditionalError("message", false)
	require.Equal(t, COMPLETED, res.Status)
	res.AdditionalError("message", true)
	require.Equal(t, FAILED, res.Status)
}

func newTestTaskResponse() (*TaskRequest, *TaskResponse) {
	req := &TaskRequest{
		UserID:          "user",
		OrganizationID:  "org",
		JobDefinitionID: "job-id",
		JobRequestID:    102,
		TaskExecutionID: "101",
		JobType:         "job-type",
		TaskType:        "task",
		Script:          []string{"a"},
		ExecutorOpts:    &ExecutorOptions{},
	}
	res := NewTaskResponse(req)
	res.Status = COMPLETED
	return req, res
}

