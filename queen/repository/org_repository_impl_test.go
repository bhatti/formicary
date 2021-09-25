package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
)

// Get operation should fail if org doesn't exist
func Test_ShouldGetOrganizationWithNonExistingId(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
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
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN deleting non-existing organization
	err = repo.Delete(qc, "non-existing-error")
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Saving org without user should not fail
func Test_ShouldSaveOrganizationWithoutUser(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN creating organization without unit
	ec := common.NewOrganization("", "org", "bundle")
	_, err = repo.Create(qc, ec)
	// THEN it should not fail
	require.NoError(t, err)
}

// Saving org without org-id should fail
func Test_ShouldSaveOrganizationWithoutOrg(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN creating organization without unit
	ec := common.NewOrganization("user", "", "bundle")
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
	qc, err := NewTestQC()
	require.NoError(t, err)
	// WHEN creating organization without bundle
	ec := common.NewOrganization("user", "org", "")
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
	qc, err := NewTestQC()
	require.NoError(t, err)
	u := common.NewOrganization("user", "test-org", "bundle")

	// WHEN saving valid org
	saved, err := repo.Create(qc, u)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving org by id should not fail
	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	// Comparing saved object
	require.NoError(t, saved.Equals(loaded))
	require.Nil(t, loaded.Subscription)

	// WHEN Updating org
	_, err = repo.Update(qc, saved)
	// THEN it should fail due to bad org-id in context
	require.Error(t, err)

	// WHEN using saved.id in context
	_, err = repo.Update(common.NewQueryContextFromIDs("", saved.ID), saved)
	// THEN it should not fail
	require.NoError(t, err)
}

// Saving valid org with subscription
func Test_ShouldSaveValidOrganizationWithSubscription(t *testing.T) {
	// GIVEN repositories
	organizationRepository, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	subscriptionRepository, err := NewTestSubscriptionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN creating subscription
	subscription, err := subscriptionRepository.Create(qc, common.NewFreemiumSubscription(qc.User))
	require.NoError(t, err)
	// THEN Retrieving should fail
	_, err = subscriptionRepository.Get(qc, subscription.ID)
	require.NoError(t, err)

	// AND retrieving org by id should not fail
	loaded, err := organizationRepository.Get(qc, qc.GetOrganizationID())
	require.NoError(t, err)
	// Comparing saved object
	require.NoError(t, qc.User.Organization.Equals(loaded))
	require.NotNil(t, loaded.Subscription)
	require.Equal(t, subscription.ID, loaded.Subscription.ID)

	// WHEN Updating org
	_, err = organizationRepository.Update(common.NewQueryContextFromIDs("", "Bad"), qc.User.Organization)
	// THEN it should fail due to bad org-id in context
	require.Error(t, err)

	// WHEN using saved.id in context
	_, err = organizationRepository.Update(qc, qc.User.Organization)
	// THEN it should not fail
	require.NoError(t, err)
}

// Deleting a persistent org should succeed
func Test_ShouldDeletingPersistentOrganization(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	repo.Clear()

	qc, err := NewTestQC()
	require.NoError(t, err)

	u := common.NewOrganization("user", "test-org", "bundle")
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
	organizationRepository, err := NewTestOrganizationRepository()
	require.NoError(t, err)
	organizationRepository.Clear()

	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND existing orgs
	orgs := make([]*common.Organization, 10)
	for i := 0; i < 10; i++ {
		organization := common.NewOrganization(qc.GetUserID(), fmt.Sprintf("org_%d", i), fmt.Sprintf("bundle_%d", i))
		orgs[i], err = organizationRepository.Create(qc, organization)
		require.NoError(t, err)
	}

	// WHEN searching orgs without criteria
	params := make(map[string]interface{})
	_, total, err := organizationRepository.Query(qc, params, 0, 1000, []string{"id"})
	require.NoError(t, err)
	// THEN all should return
	require.Equal(t, int64(11), total) // 10 + 1 from QC

	// WHEN searching orgs with criteria
	params["org_unit"] = orgs[0].OrgUnit
	_, total, err = organizationRepository.Query(qc, params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	// THEN matching should return
	require.Equal(t, int64(1), total)
}

