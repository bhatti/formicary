package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if system-config doesn't exist
func Test_ShouldGetSystemConfigWithNonExistingId(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestSystemConfigRepository()
	require.NoError(t, err)

	// WHEN finding non-existing system config
	_, err = repo.Get("missing_id")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing system-config should fail
func Test_ShouldDeleteByTypeSystemConfigWithNonExistingType(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestSystemConfigRepository()
	require.NoError(t, err)

	// WHEN deleting non-existing system config
	err = repo.Delete("non-existing-error")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Saving system-config
func Test_ShouldSaveSystemConfig(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestSystemConfigRepository()
	require.NoError(t, err)

	// WHEN saving valid system config
	c := types.NewSystemConfig("default", "k1", "n1", "v1")
	saved, err := repo.Save(c)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving system-config by id should not fail
	loaded, err := repo.Get(saved.ID)
	require.NoError(t, err)

	// AND comparing saved object should be equal
	require.NoError(t, c.Equals(loaded))

	c.Value = "NEW_VAL"
	// AND updating system-config should not fail
	saved, err = repo.Save(c)
	require.NoError(t, err)
	require.NoError(t, c.Equals(saved))

	// AND retrieving system-config by id should not fail
	loaded, err = repo.Get(saved.ID)
	require.NoError(t, err)
	require.NoError(t, c.Equals(loaded))
}

// Deleting a persistent system-config should succeed
func Test_ShouldDeletingPersistentSystemConfig(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestSystemConfigRepository()
	require.NoError(t, err)

	// AND existing system-config
	c := types.NewSystemConfig("default", "k1", "n1", "v1")
	// Saving valid system-config
	saved, err := repo.Save(c)
	require.NoError(t, err)

	// WHEN Deleting system-config by id
	err = repo.Delete(saved.ID)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving should fail
	_, err = repo.Get(saved.ID)
	require.Error(t, err)
}

// Test GetAll configs
func Test_ShouldGetAllSystemConfigs(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestSystemConfigRepository()
	require.NoError(t, err)
	repo.clear()
	kinds := []string{"apple", "android", "LINUX"}

	// AND a set of existing system configs
	for _, kind := range kinds {
		for i := 0; i < 10; i++ {
			c := types.NewSystemConfig(
				"default",
				kind,
				fmt.Sprintf("name-%v", i),
				fmt.Sprintf("value_%v", i))
			_, err := repo.Save(c)
			require.NoError(t, err)
			loaded, err := repo.GetByKindName(c.Kind, c.Name)
			require.NoError(t, err)
			require.NoError(t, c.Equals(loaded))
		}
	}

	for _, kind := range kinds {
		// WHEN querying by kind
		recs, total, err := repo.Query(map[string]interface{}{"kind": kind}, 0, 100, make([]string, 0))
		require.NoError(t, err)

		// THEN it should return valid records
		require.Equal(t, int64(10), total)
		require.Equal(t, 10, len(recs))
	}

	// WHEN querying by name
	recs, total, err := repo.Query(map[string]interface{}{"name": "name-0"}, 0, 100, make([]string, 0))
	require.NoError(t, err)
	require.Equal(t, int(total), len(kinds))
	require.Equal(t, len(recs), len(kinds))
}
