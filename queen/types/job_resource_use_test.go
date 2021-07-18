package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Verify table names for use-resource-use
func Test_ShouldJobResourceUseTableNames(t *testing.T) {
	use := NewJobResourceUse("resID", 12345, "taskID", "userID", 0, time.Now())
	require.Equal(t, "formicary_job_resource_uses", use.TableName())
}

// Validate happy path of Validate with proper use-resource-use
func Test_ShouldValidateWithGoodJobResourceUse(t *testing.T) {
	use := newTestJobResourceUse(1)
	// WHEN validating valid resource
	err := use.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldNotValidateJobResourceUseWithoutValue(t *testing.T) {
	use := NewJobResourceUse("resID", 12345, "taskID", "userID", 0, time.Now())
	// WHEN validating without value
	err := use.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "value")
}

func Test_ShouldNotValidateJobResourceUseWithoutResourceID(t *testing.T) {
	use := NewJobResourceUse("", 12345, "taskID", "userID", 1, time.Now())
	// WHEN validating without resource-id
	err := use.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "job-resource-id")
}

func Test_ShouldJobResourceUseValidateWithoutRequestID(t *testing.T) {
	use := NewJobResourceUse("resID", 0, "taskID", "userID", 1, time.Now())
	// WHEN validating without request-id
	err := use.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "job-request")
}

func Test_ShouldNotValidateJobResourceUseWithoutTaskID(t *testing.T) {
	use := NewJobResourceUse("resID", 12345, "", "userID", 1, time.Now())
	// WHEN saving resource without taks-id
	err := use.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "task-id")
}

func Test_ShouldJobResourceUseValidateWithoutExpiration(t *testing.T) {
	use := NewJobResourceUse("resID", 12345, "task", "userID", 1, time.Time{})
	// WHEN saving resource without expiration
	err := use.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "expiration")
}

func newTestJobResourceUse(val int) *JobResourceUse {
	return NewJobResourceUse("resID", 12345, "taskID", "userID", val, time.Now())
}
