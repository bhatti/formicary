package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
)

// Get operation should fail if subscription doesn't exist
func Test_ShouldGetSubscriptionWithNonExistingId(t *testing.T) {
	// GIVEN subscription repository
	repo, err := NewTestSubscriptionRepository()
	require.NoError(t, err)

	// WHEN loading non existing subscription
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Saving a persistent subscription should succeed
func Test_ShouldSavingPersistentSubscription(t *testing.T) {
	// GIVEN subscription repository
	repo, err := NewTestSubscriptionRepository()
	require.NoError(t, err)
	subscription := common.NewFreemiumSubscription("test-user", "test-org")
	// WHEN Saving valid subscription
	saved, err := repo.Create(subscription)
	require.NoError(t, err)

	// THEN Retrieving should not fail
	_, err = repo.Get(qc, saved.ID)
	require.NoError(t, err)
}

// Updating a persistent subscription should succeed
func Test_ShouldUpdatePersistentSubscription(t *testing.T) {
	// GIVEN subscription repository
	repo, err := NewTestSubscriptionRepository()
	require.NoError(t, err)

	subscription := common.NewFreemiumSubscription("test-user", "test-org")
	// AND Saving valid subscription
	saved, err := repo.Create(subscription)
	require.NoError(t, err)

	saved.DiskQuota += 10
	saved.CPUQuota += 10
	// WHEN Updating subscription by id
	saved, err = repo.Update(qc, saved)
	// THEN it should not fail
	require.NoError(t, err)

	// THEN Retrieving should fail
	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, saved.DiskQuota, loaded.DiskQuota)
	require.Equal(t, saved.CPUQuota, loaded.CPUQuota)
}

// Deleting a persistent subscription should succeed
func Test_ShouldDeletingPersistentSubscription(t *testing.T) {
	// GIVEN subscription repository
	repo, err := NewTestSubscriptionRepository()
	require.NoError(t, err)

	subscription := common.NewFreemiumSubscription("test-user", "test-org")
	// AND Saving valid subscription
	saved, err := repo.Create(subscription)
	require.NoError(t, err)

	// WHEN Deleting subscription by id
	err = repo.Delete(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// THEN Retrieving should fail
	_, err = repo.Get(qc, saved.ID)
	require.Error(t, err)
}

// Test SaveFile and query
func Test_ShouldSaveAndQuerySubscriptions(t *testing.T) {
	// GIVEN subscription repository
	repo, err := NewTestSubscriptionRepository()
	require.NoError(t, err)
	repo.Clear()

	// AND a set of subscriptions
	for i := 0; i < 10; i++ {
		subscription := common.NewFreemiumSubscription(qc.UserID, fmt.Sprintf("test-org-%d", i))
		_, err = repo.Create(subscription)
		require.NoError(t, err)
	}
	params := make(map[string]interface{})

	// WHEN querying by org
	params["organization_id"] = "test-org-1"
	_, total, err := repo.Query(common.NewQueryContext("", "test-org-1", ""), params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	// THEN it should match expected count
	require.Equal(t, int64(1), total)

	// WHEN querying as a different user
	_, total, err = repo.Query(common.NewQueryContext("x", "y", ""), params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	// THEN it should not return data
	require.Equal(t, int64(0), total)
}
