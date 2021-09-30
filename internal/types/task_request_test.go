package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/utils"
	"testing"
)

func Test_ShouldVerifyTaskKeyForTaskRequest(t *testing.T) {
	req := newTestTaskRequest()
	require.Equal(t, "102-task", req.Key())
	require.Equal(t, "user/job-102/task", req.KeyPath())
	require.NotEqual(t, "", req.String())
	req.AddVariable("name", "value", true)
}

func Test_ShouldVerifyMaskFieldsForTaskRequest(t *testing.T) {
	req := newTestTaskRequest()
	require.Equal(t, 0, len(req.GetMaskFields()))
	req.AddVariable("a", "value", true)
	require.Equal(t, 1, len(req.GetMaskFields()))
}

func Test_ShouldVerifyCacheArtifactID(t *testing.T) {
	req := newTestTaskRequest()
	require.Equal(t, "", req.CacheArtifactID("prefix", "key"))
	req.ExecutorOpts.Cache.Paths = []string{"a", "b"}
	req.ExecutorOpts.Cache.Key = "key"
	require.Equal(t, "prefix/org/job-type/key/cache.zip", req.CacheArtifactID("prefix", "key"))
	req.OrganizationID = ""
	require.Equal(t, "prefix/user/job-type/key/cache.zip", req.CacheArtifactID("prefix", "key"))
}

func Test_ShouldSanitizeIllegalCharactersInTaskKey(t *testing.T) {
	req := newTestTaskRequest()
	req.TaskType = "my-test:&*#+-first-123*%^"
	key := utils.MakeDNS1123Compatible(fmt.Sprintf("formicary-%d-%s", req.JobRequestID, req.TaskType))
	require.Equal(t, "formicary-102-my-test-first-123", key)
}

func Test_ShouldMarshalTaskRequest(t *testing.T) {
	req := newTestTaskRequest()
	b, err := req.Marshal("key")
	require.NoError(t, err)
	copyReq, err := UnmarshalTaskRequest("key", b)
	require.NoError(t, err)
	require.Equal(t, req.String(), copyReq.String())
}

func newTestTaskRequest() *TaskRequest {
	return &TaskRequest{
		UserID:          "user",
		OrganizationID:  "org",
		JobDefinitionID: "job-id",
		JobRequestID:    102,
		JobExecutionID:  "202",
		TaskExecutionID: "101",
		JobType:         "job-type",
		TaskType:        "task",
		Action:          EXECUTE,
		Script:          []string{"a"},
		ExecutorOpts: &ExecutorOptions{
			Method:          Kubernetes,
			MainContainer:   &ContainerDefinition{},
			HelperContainer: &ContainerDefinition{},
		},
		Variables: make(map[string]VariableValue),
	}
}
