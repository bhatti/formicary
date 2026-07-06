// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

// Get should fail for a non-existent ID.
func Test_ShouldNot_GetConfig_WithNonExistingId(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	_, err = repo.Get(qc, "missing_id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Delete should fail for a non-existent ID.
func Test_ShouldNot_DeleteConfig_WithNonExistingId(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	err = repo.Delete(qc, "non-existing-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Save + Get for an org config.
func Test_Should_SaveOrgConfig(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	c, err := common.NewOrgConfig(qc.User.OrganizationID, "k1", "default", true)
	require.NoError(t, err)

	saved, err := repo.Save(qc, c)
	require.NoError(t, err)

	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, c.Name, loaded.Name)

	c.Value = "NEW_VAL"
	saved, err = repo.Save(qc, c)
	require.NoError(t, err)

	loaded, err = repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "NEW_VAL", loaded.Value)
}

// Save + Get for a user config.
func Test_Should_SaveUserConfig(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	c, err := common.NewUserConfig(qc.User.ID, "user_key", "user_val", false)
	require.NoError(t, err)

	saved, err := repo.Save(qc, c)
	require.NoError(t, err)

	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "user_key", loaded.Name)
	require.Equal(t, common.ConfigurableTypeUser, loaded.ConfigurableType)
}

// Org config persisted through the org repository should be accessible.
func Test_ShouldSavePersistentOrganizationWithConfig(t *testing.T) {
	orgRepo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	configRepo, err := NewTestConfigRepository()
	require.NoError(t, err)
	orgRepo.Clear()

	qc, err := NewTestQC()
	require.NoError(t, err)

	c1, err := common.NewOrgConfig(qc.GetOrganizationID(), "k1", "secret", true)
	require.NoError(t, err)
	_, err = configRepo.Save(qc, c1)
	require.NoError(t, err)

	c2, err := common.NewOrgConfig(qc.GetOrganizationID(), "k2", "next-secret", true)
	require.NoError(t, err)
	_, err = configRepo.Save(qc, c2)
	require.NoError(t, err)

	loaded, err := orgRepo.Get(qc, qc.GetOrganizationID())
	require.NoError(t, err)
	require.Equal(t, 2, len(loaded.Configs))
	require.Equal(t, "secret", loaded.GetConfig("k1").Value)
	require.Equal(t, "next-secret", loaded.GetConfig("k2").Value)

	recs, total, err := configRepo.QueryOrgConfigs(qc, c1.ConfigurableID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, 2, len(recs))
}

// Delete a persisted config.
func Test_ShouldDeletingPersistentConfig(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	c, err := common.NewOrgConfig(qc.User.OrganizationID, "k1", "default", true)
	require.NoError(t, err)

	saved, err := repo.Save(qc, c)
	require.NoError(t, err)

	err = repo.Delete(qc, saved.ID)
	require.NoError(t, err)

	_, err = repo.Get(qc, saved.ID)
	require.Error(t, err)
}

// Query returns all saved org configs.
func Test_ShouldGetAllOrgConfigs(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		c, err := common.NewOrgConfig(
			qc.User.OrganizationID,
			fmt.Sprintf("name-%v", i),
			fmt.Sprintf("value_%v", i),
			true)
		require.NoError(t, err)
		_, err = repo.Save(qc, c)
		require.NoError(t, err)
		_, err = repo.getByOwnerAndName(common.ConfigurableTypeOrg, c.ConfigurableID, c.Name)
		require.NoError(t, err)
	}

	recs, total, err := repo.QueryOrgConfigs(qc, qc.User.OrganizationID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
	require.Equal(t, 10, len(recs))
}

// QueryUserConfigs returns all saved user configs.
func Test_ShouldGetAllUserConfigs(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		c, err := common.NewUserConfig(
			qc.User.ID,
			fmt.Sprintf("user-key-%v", i),
			fmt.Sprintf("user-val-%v", i),
			false)
		require.NoError(t, err)
		_, err = repo.Save(qc, c)
		require.NoError(t, err)
	}

	recs, total, err := repo.QueryUserConfigs(qc, qc.User.ID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Equal(t, 5, len(recs))
}

// User config should take precedence over org config with the same name.
func Test_ShouldUserConfigOverrideOrgConfig(t *testing.T) {
	repo, err := NewTestConfigRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	orgCfg, err := common.NewOrgConfig(qc.User.OrganizationID, "shared_key", "org_value", false)
	require.NoError(t, err)
	_, err = repo.Save(qc, orgCfg)
	require.NoError(t, err)

	userCfg, err := common.NewUserConfig(qc.User.ID, "shared_key", "user_value", false)
	require.NoError(t, err)
	_, err = repo.Save(qc, userCfg)
	require.NoError(t, err)

	// Simulate the merge that buildDynamicConfigs performs.
	merged := make(map[string]string)
	orgConfigs, _, err := repo.QueryOrgConfigs(qc, qc.User.OrganizationID, 0, 100)
	require.NoError(t, err)
	for _, c := range orgConfigs {
		merged[c.Name] = c.Value
	}
	userConfigs, _, err := repo.QueryUserConfigs(qc, qc.User.ID, 0, 100)
	require.NoError(t, err)
	for _, c := range userConfigs {
		merged[c.Name] = c.Value // user wins on collision
	}

	require.Equal(t, "user_value", merged["shared_key"])
}
