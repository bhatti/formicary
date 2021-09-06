package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if user doesn't exist
func Test_ShouldGetUserWithNonExistingId(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	// WHEN finding non-existing user
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing user should fail
func Test_ShouldDeleteUserByNonExistingId(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	// WHEN deleting non-existing user
	err = repo.Delete(qc, "non-existing-error")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Saving user without org-id should succeed
func Test_ShouldSaveUserWithoutOrg(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// WHEN creating a user without org-id
	ec := common.NewUser("", "username", "name", "test@formicary.io", false)
	_, err = repo.Create(ec)

	// THEN it should not fail
	require.NoError(t, err)
}

// Saving user without username should fail
func Test_ShouldNotSaveUserWithoutUsername(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// WHEN creating a user without username
	ec := common.NewUser("org", "", "name", "test@formicary.io", false)
	_, err = repo.Create(ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "username")
}

// Saving user without name should fail
func Test_ShouldNotSaveUserWithoutName(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// WHEN creating a user without name
	ec := common.NewUser("org", "user", "", "test@formicary.io", false)
	_, err = repo.Create(ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "name")
}

// Saving valid user with subscription
func Test_ShouldSaveValidUserWithSubscription(t *testing.T) {
	// GIVEN a user repository
	userRepository, err := NewTestUserRepository()
	require.NoError(t, err)
	subscriptionRepository, err := NewTestSubscriptionRepository()
	require.NoError(t, err)

	// WHEN creating a valid user
	u := common.NewUser("test-org", "username", "name", "test@formicary.io", false)
	// Saving valid user
	saved, err := userRepository.Create(u)

	// THEN it should not fail
	require.NoError(t, err)

	subscription, err := subscriptionRepository.Create(common.NewFreemiumSubscription(saved.ID, ""))
	require.NoError(t, err)

	// AND retrieving user by id should not fail
	loaded, err := userRepository.Get(common.NewQueryContext(saved.ID, saved.OrganizationID, saved.Salt), saved.ID)
	require.NoError(t, err)

	require.NotNil(t, loaded.Subscription)
	require.Equal(t, subscription.ID, loaded.Subscription.ID)
}

// Saving valid user
func Test_ShouldSaveValidUser(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// WHEN creating a valid user
	u := common.NewUser(qc.OrganizationID, "username", "name", "test@formicary.io", false)
	// Saving valid user
	saved, err := repo.Create(u)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving user by id should not fail
	loaded, err := repo.Get(common.NewQueryContext(saved.ID, saved.OrganizationID, saved.Salt), saved.ID)
	require.NoError(t, err)

	// AND comparing saved object should match
	require.NoError(t, saved.Equals(loaded))

	// WHEN updating user by another
	_, err = repo.Update(common.NewQueryContext("bad", "", ""), saved)

	// THEN it should fail
	require.Error(t, err)

	// WHEN updating by same user
	_, err = repo.Update(common.NewQueryContext(saved.ID, saved.OrganizationID, saved.Salt), saved)

	// THEN it should not fail
	require.NoError(t, err)
}

// Deleting a persistent user should succeed
func Test_ShouldDeletingPersistentUser(t *testing.T) {
	repo, err := NewTestUserRepository()
	if err != nil {
		t.Fatalf("unexpected error %v while creating user repository", err)
	}

	u := common.NewUser("test-org", "user", "name", "test@formicary.io", false)

	// Saving valid user
	saved, err := repo.Create(u)
	if err != nil {
		t.Fatalf("unexpected error %v while saving error code", err)
	}

	// Delete user by id
	err = repo.Delete(qc, saved.ID)
	if err != nil {
		t.Fatalf("unexpected error while deleting user %v", err)
	}

	// Retrieving should fail
	loaded, err := repo.Get(qc, saved.ID)
	if err == nil || loaded != nil {
		t.Fatalf("Should not find user after deletion %v - %v", err, loaded)
	}
}

// Test SaveFile and query
func Test_ShouldSaveAndQueryUsers(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND a set of users in the database
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			u := common.NewUser(
				fmt.Sprintf("org_%d", i),
				fmt.Sprintf("username_%d_%d", i, j),
				"name",
				fmt.Sprintf("username_%d_%d@formicary.io", i, j),
				true)
			if _, err := repo.Create(u); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		}
	}
	params := make(map[string]interface{})

	// WHEN querying users
	_, total, err := repo.Query(common.NewQueryContext("", "", ""), params, 0, 1000, []string{"id"})

	// THEN it should not fail and match number of expected records
	require.NoError(t, err)
	require.Equal(t, int64(50), total)

	// WHEN querying users by org-id
	params["organization_id"] = "org_0"
	_, total, err = repo.Query(common.NewQueryContext("", "", ""), params, 0, 1000, make([]string, 0))
	// THEN it should not fail and match number of expected records
	require.NoError(t, err)
	require.Equal(t, int64(5), total)

	// WHEN querying users by org-id
	_, total, err = repo.Query(
		common.NewQueryContext("", "org_0", ""),
		params,
		0,
		1000,
		make([]string, 0))
	// THEN it should not fail and match number of expected records
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
}

// Saving session
func Test_ShouldAddSession(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	// AND an existing user
	u := common.NewUser("test-org", "username", "name", "test@formicary.io", false)
	// Saving valid user
	saved, err := repo.Create(u)
	require.NoError(t, err)

	// WHEN creating a user session
	session := types.NewUserSession(u, "session-id")
	err = repo.AddSession(session)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN updating the user session
	session.Username = saved.Username
	session.UserID = saved.ID
	err = repo.UpdateSession(session)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN finding session by id
	_, err = repo.GetSession(session.SessionID)
	// THEN it should not fail
	require.NoError(t, err)
}

// Saving an API token
func Test_ShouldAddToken(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestUserRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND an existing user
	u := common.NewUser("test-org", "username-tok", "name", "test@formicary.io", false)
	// Saving valid user
	saved, err := repo.Create(u)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		// WHEN creating API token
		tokenName := fmt.Sprintf("tok-name_%d", i)
		tok := types.NewUserToken(saved.ID, saved.OrganizationID, tokenName)
		err = repo.AddToken(tok)
		if err == nil {
			t.Fatalf("expecting error")
		}
		tok.APIToken = "abc"
		tok.ExpiresAt = time.Now().Add(1 * time.Minute)
		err = repo.AddToken(tok)

		// THEN it should not fail
		require.NoError(t, err)

		// finding token should not fail
		require.True(t, repo.HasToken(saved.ID, tokenName, tok.SHA256))
		if i == 9 {
			tok.TokenName = "abcdef"
			// Adding another API token should fail because by default you can  only have 10 active API tokens
			err = repo.AddToken(tok)
			require.Error(t, err)
		}
	}

	// WHEN finding api tokens
	toks, err := repo.GetTokens(common.NewQueryContext("", "", ""), saved.ID)

	// THEN it should not fail and match expected number of records
	require.NoError(t, err)
	require.Equal(t, 10, len(toks))

	// WHEN revoking api token
	err = repo.RevokeToken(common.NewQueryContext(saved.ID, "test-org", ""), saved.ID, toks[0].ID)
	require.NoError(t, err)

	// THEN should not find revoked api token
	require.False(t, repo.HasToken(saved.ID, "tok-name", "abc"))

	// WHEN finding all tokens
	toks, err = repo.GetTokens(common.NewQueryContext("", "", ""), saved.ID)
	// THEN it should not fail and match expected number of records
	require.NoError(t, err)
	require.Equal(t, 9, len(toks))
	require.NoError(t, err)
}
