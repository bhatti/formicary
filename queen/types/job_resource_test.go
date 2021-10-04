package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify table names for res-resource
func Test_ShouldJobResourceTableNames(t *testing.T) {
	res := newTestJobResource("test-res")
	require.Equal(t, "formicary_job_resources", res.TableName())
}

// Validate happy path of Validate with proper res-resource
func Test_ShouldWithGoodJobResource(t *testing.T) {
	res := newTestJobResource("test-res")
	res.AfterLoad()
	// WHEN validating valid job-resource
	err := res.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
	require.Error(t, res.MatchTag([]string{"a", "b"}))
	require.Equal(t, "", res.ShortID())
	res.ID = "1234567890"
	require.Equal(t, "12345678...", res.ShortID())
}

func Test_ShouldHaveConfigWithGoodJobResource(t *testing.T) {
	res := newTestJobResource("test-res")
	// WHEN validating valid job-resource
	err := res.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "k1=jv1,k2=jv2,", res.ConfigString())
	require.NotNil(t, res.GetConfig("k1"))
	require.NotNil(t, res.GetConfigByID("c1"))
	require.Nil(t, res.GetConfig("k3"))
	require.NotNil(t, res.DeleteConfig("k1"))
	require.Nil(t, res.GetConfig("k1"))
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
	c1, _ := res.AddConfig("k1", "jv1")
	c1.ID = "c1"
	c2, _ := res.AddConfig("k2", "jv2")
	c2.ID = "c2"
	res.Uses = []*JobResourceUse{&JobResourceUse{
		UserID: "u10",
		Value: 101,
	}}
	return res
}
