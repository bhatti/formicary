package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify table names for res-resource
func Test_ShouldJobResourceTableNames(t *testing.T) {
	res := newTestJobResource("test-res")
	require.Equal(t, "formicary_job_resources",res.TableName())
}

// Validate happy path of Validate with proper res-resource
func Test_ShouldWithGoodJobResource(t *testing.T) {
	res := newTestJobResource("test-res")
	// WHEN validating valid job-resource
	err := res.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
}

// Validate should fail if res type is empty
func Test_ShouldNotValidateJobResourceWithoutName(t *testing.T) {
	res := NewJobResource("", 10)
	// WHEN validating valid job-resource without name
	err := res.ValidateBeforeSave()

	// THEN it should not fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource-type")
}

// Validate should fail if res quota is empty
func Test_ShouldNotValidateJobResourceWithoutQuota(t *testing.T) {
	res := NewJobResource("xxx", 0)
	// WHEN validating valid job-resource without name
	err := res.ValidateBeforeSave()

	// THEN it should not fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "quota")
}

func newTestJobResource(kind string) *JobResource {
	res := NewJobResource(kind, 1)
	_, _ = res.AddConfig("k1", "jv1")
	_, _ = res.AddConfig("k2", "jv2")
	return res
}
