package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if job-resource doesn't exist
func Test_ShouldGetJobResourceWithNonExistingId(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN finding non-existing job-resource
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Pausing non-existing job-resource should fail
func Test_ShouldPauseByTypeJobResourceWithNonExistingType(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN pausing non-existing job-resource
	err = repo.SetPaused(qc, "non-existing-job", true)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to set paused")
}

// Deleting non-existing job-resource should fail
func Test_ShouldDeleteByTypeJobResourceWithNonExistingType(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN deleting non-existing job-resource
	err = repo.Delete(qc, "non-existing-job")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Saving job-resource without job-type should fail
func Test_ShouldSaveJobResourceWithoutJobType(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)

	// WHEN saving resource without job-type
	job := types.NewJobResource("", 1)
	_, err = repo.Save(job)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource-type is not specified")
}

// Saving valid job-resource without config should succeed
func Test_ShouldSaveValidJobResourceWithoutConfig(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	resource := newTestResource("valid-resource-without-config")
	resource.UserID = qc.User.ID
	resource.OrganizationID = qc.User.OrganizationID

	// WHEN saving valid resource
	saved, err := repo.Save(resource)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN Retrieving resource by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// Comparing saved object
	require.NoError(t, loaded.Equals(resource))
}

// Saving a job with config should succeed
func Test_ShouldSaveValidJobResourceWithConfig(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)

	qc, err := NewTestQC()
	require.NoError(t, err)
	// Creating a job
	resource := newTestResource("valid-job-with-config")
	resource.UserID = qc.User.ID
	resource.OrganizationID = qc.User.OrganizationID
	_, _ = resource.AddConfig("jk1", "jv1")
	_, _ = resource.AddConfig("jk2", true)

	// WHEN saving job
	saved, err := repo.Save(resource)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving job by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	require.NoError(t, loaded.Equals(resource))
}

// Updating a job resource should succeed
func Test_ShouldUpdateValidJobResource(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	repo.clear()

	qc, err := NewTestQC()
	require.NoError(t, err)
	resource := newTestResource("test-resource-for-update")
	resource.UserID = qc.User.ID
	resource.OrganizationID = qc.User.OrganizationID

	// WHEN saving resource
	saved, err := repo.Save(resource)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving resource by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(resource))

	// should be able to delete config
	require.NotNil(t, resource.DeleteConfig("db"))

	// WHEN updating resource
	saved, err = repo.Save(resource)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving resource by id
	loaded, err = repo.Get(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(resource))
}

// Pausing a persistent job-resource should succeed
func Test_ShouldPausingPersistentJobResource(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	resource := newTestResource("test-resource-for-pause")
	resource.UserID = qc.User.ID
	resource.OrganizationID = qc.User.OrganizationID
	err = resource.ValidateBeforeSave()
	require.NoError(t, err)

	// AND existing resource
	saved, err := repo.Save(resource)
	require.NoError(t, err)

	// WHEN pausing resource by id
	err = repo.SetPaused(qc, saved.ID, true)
	// THEN it should not fail
	require.NoError(t, err)
}

// Deleting a persistent job-resource should succeed
func Test_ShouldDeletingPersistentJobResource(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	resource := newTestResource("test-resource-for-delete")
	resource.UserID = qc.User.ID
	resource.OrganizationID = qc.User.OrganizationID
	err = resource.ValidateBeforeSave()
	require.NoError(t, err)

	// AND existing resource
	saved, err := repo.Save(resource)
	require.NoError(t, err)

	// WHEN deleting resource by id
	err = repo.Delete(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving
	_, err = repo.Get(qc, saved.ID)
	// THEN it should fail
	require.Error(t, err)
}

// Querying job-resources by job-type
func Test_ShouldQueryJobResourceByJobType(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	repo.clear()

	qc, err := NewTestQC()
	require.NoError(t, err)
	resources := make(map[string]*types.JobResource)
	// AND a set of job-resources in database
	for i := 0; i < 10; i++ {
		resource := newTestResource(fmt.Sprintf("query-job-%v", i))
		resource.UserID = qc.User.ID
		resource.OrganizationID = qc.User.OrganizationID
		saved, err := repo.Save(resource)
		if err != nil {
			t.Fatalf("unexpected error %v while saving a job", err)
		}
		resources[saved.ID] = resource
	}

	// WHEN querying resources without filters
	params := make(map[string]interface{})
	_, total, err := repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying resources by job-type
	params["resource_type"] = "query-job-1"
	res, total, err := repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, len(res))
	require.NoError(t, res[0].Equals(resources[res[0].ID]))
}

// Test Query with different operators
func Test_ShouldQueryJobResourceWithDifferentOperators(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	repo.clear()

	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of job-resources in database
	for i := 0; i < 10; i++ {
		resource := newTestResource(fmt.Sprintf("resource-query-operator-%v", i))
		resource.UserID = qc.User.ID
		resource.OrganizationID = qc.User.OrganizationID
		_, err := repo.Save(resource)
		if err != nil {
			t.Fatalf("failed to save resource %v", err)
		}
	}

	// WHEN querying using LIKE
	params := make(map[string]interface{})
	params["resource_type:like"] = "resource-query-operator"
	_, total, err := repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying using IN operator
	params = make(map[string]interface{})
	params["resource_type:in"] = "resource-query-operator-0,resource-query-operator-1"
	_, total, err = repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	// WHEN querying using exact operator
	params = make(map[string]interface{})
	params["resource_type:="] = "resource-query-operator-0"
	_, total, err = repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// WHEN querying using not equal operator
	params = make(map[string]interface{})
	params["resource_type:!="] = "resource-query-operator-0"
	_, total, err = repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using GreaterThan operator
	params = make(map[string]interface{})
	params["resource_type:>"] = "resource-query-operator-0"
	_, total, err = repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using LessThan operator
	params = make(map[string]interface{})
	params["resource_type:<"] = "resource-query-operator-9"
	_, total, err = repo.Query(qc, params, 0, 100, []string{"resource_type desc"})
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)
}

func Test_ShouldAllocateDeallocateResources(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)

	repo.clear()
	resource := newTestResource("allocate-resource")
	resource.Quota = 10
	_, err = repo.Save(resource)
	if err != nil {
		t.Fatalf("failed to save resource %v", err)
	}
	uses := make([]*types.JobResourceUse, resource.Quota)

	for i := 0; i < resource.Quota; i++ {
		uses[i] = types.NewJobResourceUse(resource.ID, 101, fmt.Sprintf("201_%d", i), "301", 1, time.Now())
		// WHEN allocating a resource with enough quota
		_, err = repo.Allocate(resource, uses[i])
		// THEN it should not fail
		require.NoError(t, err)

		// AND used quota should increment
		remaining, err := repo.GetUsedQuota(resource.ID)
		require.NoError(t, err)
		require.Equal(t, i+1, remaining)
	}

	// WHEN allocating an resource when quota is exceeded
	use := types.NewJobResourceUse(resource.ID, 101, "201", "301", 1, time.Now())
	_, err = repo.Allocate(resource, use)

	// THEN it should fail
	require.Error(t, err)

	for i, use := range uses {
		// WHEN deallocating resource
		err = repo.Deallocate(use)
		// THEN it should not fail
		require.NoError(t, err)

		// AND used quota should decrement
		remaining, err := repo.GetUsedQuota(resource.ID)
		require.NoError(t, err)
		require.Equal(t, resource.Quota-i-1, remaining)
	}
}

// resource-type   platform     category     tags
// CI,           , windows,                  , build,test,windows,xp
// CI,           , windows,                  , 10,build,deploy,test,windows
// CI,           , LINUX,                    , build,deploy,test,ubuntu
// CI,           , mac,                      , build,catalina,macos,test
// CI,           , mac,                      , bigsur,build,deploy,macos,test
func Test_ShouldMatchResources(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobResourceRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	resources := make([]*types.JobResource, 5)
	tags := []string{"macos bigsur build test deploy", "macos catalina build test",
		"windows 10 build test deploy", "windows xp build test", "ubuntu build test deploy"}
	platforms := []string{"MAC", "MAC", "WINDOWS", "WINDOWS", "LINUX"}

	// Initializing resource array in the database
	for i := 0; i < len(resources); i++ {
		resources[i] = newTestResource(fmt.Sprintf("match-resource-%d", i))
		resources[i].ResourceType = "CI"
		resources[i].Quota = 10
		resources[i].Tags = utils.SplitTags(tags[i])
		resources[i].Platform = platforms[i]
		resources[i].UserID = qc.User.ID
		resources[i].OrganizationID = qc.User.OrganizationID
		resources[i], err = repo.Save(resources[i])
		require.NoError(t, err)
	}

	// WHEN matching by CI resource, MAC platform and 'build test' tags
	matching, total, err := repo.MatchByTags(
		qc,
		"CI",
		"MAC",
		utils.SplitTags("build test	"),
		2)
	// THEN it should return valid result
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(matching))

	// WHEN matching by CI resource, MAC platform and 'build test deploy' tags
	matching, total, err = repo.MatchByTags(
		qc,
		"CI",
		"MAC",
		utils.SplitTags("build test deploy"),
		2)
	// THEN it should return valid result
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 1, len(matching))

	// WHEN matching WINDOWS platform and 'build test' tags
	matching, total, err = repo.MatchByTags(
		qc,
		"CI",
		"WINDOWS",
		utils.SplitTags("build test"),
		2)
	// THEN it should return valid result
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(matching))

	// WHEN matching WINDOWS platform and 'build test deploy' tags
	matching, total, err = repo.MatchByTags(
		qc,
		"CI",
		"WINDOWS",
		utils.SplitTags("build test deploy"),
		2)
	// THEN it should return valid result
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 1, len(matching))

	// WHEN matching bigger value (12) than available (2)
	matching, total, err = repo.MatchByTags(
		qc,
		"CI",
		"WINDOWS",
		utils.SplitTags("build test deploy"),
		12)
	// THEN it should not find any matching list
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 0, len(matching))
}

// Creating a test job
func newTestResource(name string) *types.JobResource {
	resource := types.NewJobResource(name, 1)
	resource.LeaseTimeout = 1 * time.Second
	_, _ = resource.AddConfig("env", "prod")
	_, _ = resource.AddConfig("docker", map[string]interface{}{"image": "alpine", "memory": 200})
	_, _ = resource.AddConfig("db", "mysql")

	return resource
}
