package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if org doesn't exist
func Test_ShouldGetOrganizationWithNonExistingId(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	// WHEN finding non-existing organization
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing org should fail
func Test_ShouldDeleteOrganizationByNonExistingId(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	// WHEN deleting non-existing organization
	err = repo.Delete(qc, "non-existing-error")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Saving org without org-id should fail
func Test_ShouldSaveOrganizationWithoutOrg(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()
	// WHEN creating organization without unit
	ec := common.NewOrganization("", "bundle")
	_, err = repo.Create(qc, ec)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "org")
}

// Saving org without bundle should fail
func Test_ShouldSaveOrganizationWithoutBundle(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()
	// WHEN creating organization without bundle
	ec := common.NewOrganization("org", "")
	_, err = repo.Create(qc, ec)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "bundle")
}

// Saving valid org
func Test_ShouldSaveValidOrganization(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	u := common.NewOrganization("test-org", "bundle")

	// WHEN saving valid org
	saved, err := repo.Create(qc, u)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving org by id should not fail
	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	// Comparing saved object
	require.NoError(t, saved.Equals(loaded))

	// WHEN Updating org
	_, err = repo.Update(qc, saved)
	// THEN it should fail due to bad org-id in context
	require.Error(t, err)

	// WHEN using saved.id in context
	_, err = repo.Update(common.NewQueryContext("", saved.ID, saved.Salt), saved)
	// THEN it should not fail
	require.NoError(t, err)
}

// Deleting a persistent org should succeed
func Test_ShouldDeletingPersistentOrganization(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	u := common.NewOrganization("test-org", "bundle")

	repo.Clear()

	// WHEN saving valid org
	saved, err := repo.Create(qc, u)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN Deleting org by id
	err = repo.Delete(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving should fail
	_, err = repo.Get(qc, saved.ID)
	require.Error(t, err)
}

// Test SaveFile and query
func Test_ShouldSaveAndQueryOrganizations(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()

	orgs := make([]*common.Organization, 10)
	for i := 0; i < 10; i++ {
		u := common.NewOrganization(fmt.Sprintf("org_%d", i), fmt.Sprintf("bundle_%d", i))
		orgs[i], err = repo.Create(qc, u)
		require.NoError(t, err)
	}
	params := make(map[string]interface{})
	_, total, err := repo.Query(qc, params, 0, 1000, []string{"id"})
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
	params["org_unit"] = "org_0"
	_, total, err = repo.Query(qc, params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

// Saving invitation
func Test_ShouldAddInvitation(t *testing.T) {
	// GIVEN repositories
	orgRepo, err := NewTestOrganizationRepository()
	require.NoError(t, err)

	userRepo, err := NewTestUserRepository()
	require.NoError(t, err)

	orgRepo.Clear()
	userRepo.Clear()

	// AND an existing organization
	org, err := orgRepo.Create(qc, common.NewOrganization("org1", "bundle1"))
	require.NoError(t, err)
	user := common.NewUser(org.ID, "user1", "name", false)
	user.Email = "test@formicary.io"
	user, err = userRepo.Create(user)
	require.NoError(t, err)

	// WHEN adding an invitation
	inv := types.NewUserInvitation("email1@formicary.io", user.ID, org.ID)
	err = orgRepo.AddInvitation(inv)
	require.NoError(t, err)
	loaded, err := orgRepo.GetInvitation(inv.ID)
	require.NoError(t, err)

	// THEN Should find invitation
	loaded, err = orgRepo.FindInvitation(inv.Email, inv.InvitationCode)
	require.NoError(t, err)
	require.Equal(t, inv.InvitedByUserID, loaded.InvitedByUserID)
	require.True(t, loaded.ExpiresAt.Unix() > time.Now().Unix())

	// AND should accept invitation
	_, err = orgRepo.AcceptInvitation(inv.Email, inv.InvitationCode)
	require.NoError(t, err)

	// BUT cannot accept again
	_, err = orgRepo.AcceptInvitation(inv.Email, inv.InvitationCode)
	require.Error(t, err)
}
