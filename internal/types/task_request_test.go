package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/utils"
	"testing"
)

func Test_ShouldSanitizeIllegalCharactersInTaskKey(t *testing.T) {
	req := &TaskRequest{
		JobRequestID: 102,
		TaskType:     "my-test:&*#+-first-123*%^",
	}
	key := utils.MakeDNS1123Compatible(fmt.Sprintf("formicary-%d-%s", req.JobRequestID, req.TaskType))
	require.Equal(t, "formicary-102-my-test-first-123", key)
}
