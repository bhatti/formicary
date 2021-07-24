package repository

import (
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"testing"

	"github.com/stretchr/testify/require"
)

// Get operation should fail if system-config doesn't exist
func Test_ShouldNot_GetOrganizationConfig_WithNonExistingId(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)
	// WHEN fetching an org-config with unknown id
	_, err = repo.Get(qc, "missing_id")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing system-config should fail
func Test_ShouldNot_DeleteOrganizationConfig_WithNonExistingType(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)
	// WHEN deleting an org-config with unknown id
	err = repo.Delete(qc, "non-existing-error")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Saving a valid system-config
func Test_Should_SaveOrganizationConfig(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)
	c, err := common.NewOrganizationConfig("test-org", "default", "k1", true)
	require.NoError(t, err)

	// WHEN saving org config
	saved, err := repo.Save(qc, c)
	require.NoError(t, err)

	// THEN should find system-config by id
	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)

	// AND Comparing saved object should be equal
	require.Equal(t, c.Name, loaded.Name)

	c.Value = "NEW_VAL"
	// AND Updating system-config should not fail
	saved, err = repo.Save(qc, c)
	require.NoError(t, err)

	// Retrieving system-config by id
	loaded, err = repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "NEW_VAL", loaded.Value)
}

// persist organization with configs
func Test_ShouldSavePersistentOrganizationWithConfig(t *testing.T) {
	// GIVEN an org-config repository
	orgRepo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	orgConfigRepo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)

	orgRepo.Clear()
	org := common.NewOrganization("user", "test-org", "bundle")

	// AND saved org
	saved, err := orgRepo.Create(qc, org)
	require.NoError(t, err)

	// WHEN saving org config
	c1, err := common.NewOrganizationConfig(saved.ID, "k1", "secret", true)
	require.NoError(t, err)
	_, err = orgConfigRepo.Save(qc, c1)
	require.NoError(t, err)
	c2, err := common.NewOrganizationConfig(saved.ID, "k2", "next-secret", true)
	require.NoError(t, err)
	_, err = orgConfigRepo.Save(qc, c2)
	require.NoError(t, err)

	// THEN should find system-config by id
	loaded, err := orgRepo.Get(qc, saved.ID)
	require.NoError(t, err)

	// AND Comparing saved object should be equal
	require.Equal(t, org.OrgUnit, loaded.OrgUnit)
	require.Equal(t, 2, len(loaded.Configs))
	require.Equal(t, "secret", loaded.GetConfig("k1").Value)
	require.Equal(t, "next-secret", loaded.GetConfig("k2").Value)

	recs, total, err := orgConfigRepo.Query(
		common.NewQueryContext("", org.ID, ""),
		map[string]interface{}{"organization_id": org.ID}, 0, 100, make([]string, 0))
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, 2, len(recs))
	require.Equal(t, "secret", recs[0].Value)
	require.Equal(t, "next-secret", recs[1].Value)
}

// Deleting a persistent system-config should succeed
func Test_ShouldDeletingPersistentOrganizationConfig(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)

	c, err := common.NewOrganizationConfig("test-org", "default", "k1", true)
	require.NoError(t, err)

	// WHEN Saving valid system-config
	saved, err := repo.Save(qc, c)
	require.NoError(t, err)

	// AND Deleting system-config by id
	err = repo.Delete(qc, saved.ID)
	require.NoError(t, err)

	// THEN retrieving should fail
	_, err = repo.Get(qc, saved.ID)
	require.Error(t, err)
}

// Test GetAll configs
func Test_ShouldGetAllOrganizationConfigs(t *testing.T) {
	// GIVEN an org-config repository
	repo, err := NewTestOrgConfigRepository()
	require.NoError(t, err)
	repo.clear()
	// WHEN creating a set of orgs
	for i := 0; i < 10; i++ {
		c, err := common.NewOrganizationConfig(
			"test-org",
			fmt.Sprintf("name-%v", i),
			fmt.Sprintf("value_%v", i),
			true)
		require.NoError(t, err)
		_, err = repo.Save(qc, c)
		require.NoError(t, err)
		_, err = repo.get(c.OrganizationID, c.Name)
		require.NoError(t, err)
	}

	// WHEN querying orgs
	recs, total, err := repo.Query(qc, map[string]interface{}{}, 0, 100, make([]string, 0))
	require.NoError(t, err)
	// THEN it should match totals
	require.Equal(t, int64(10), total)
	require.Equal(t, 10, len(recs))

	// WHEN querying by name
	recs, total, err = repo.Query(qc, map[string]interface{}{"name": "name-0"}, 0, 100, make([]string, 0))
	require.NoError(t, err)
	// THEN it should match totals
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, len(recs))
}
