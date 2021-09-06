package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

func Test_ShouldNotFindUnknownEmailVerification(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestEmailVerificationRepository()
	require.NoError(t, err)

	// AND user
	u := common.NewUser("", "username", "name", "email@formicary.io", false)
	u.ID = qc.UserID

	// WHEN finding unknown verification
	_, err = repo.Get(qc, "blah")

	// THEN it should fail
	require.Error(t, err)
}

func Test_ShouldFindEmailVerification(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestEmailVerificationRepository()
	require.NoError(t, err)

	// AND user
	u := common.NewUser("", "username", "name", "email@formicary.io", false)
	u.ID = qc.UserID

	ev := types.NewEmailVerification("email@formicary.io", u)
	saved, err := repo.Create(ev)

	require.NoError(t, err)

	// WHEN finding record
	_, err = repo.Get(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldDeleteEmailVerification(t *testing.T) {
	// GIVEN a user repository
	repo, err := NewTestEmailVerificationRepository()
	require.NoError(t, err)

	// AND user
	u := common.NewUser("", "username", "name", "email@formicary.io", false)
	u.ID = qc.UserID

	ev := types.NewEmailVerification("email@formicary.io", u)
	_, err = repo.Create(ev)
	require.NoError(t, err)

	// WHEN deleting record
	err = repo.Delete(qc, ev.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// AND WHEN deleting unknown record
	err = repo.Delete(qc, "blah")
	// THEN it should fail
	require.Error(t, err)
}

// Should create and verify email token
func Test_ShouldVerifyEmailVerification(t *testing.T) {
	// GIVEN a user repository
	verificationRepository, err := NewTestEmailVerificationRepository()
	require.NoError(t, err)
	userRepository, err := NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()

	// AND user
	u := common.NewUser("", "username", "name", "email@formicary.io", false)
	u.ID = qc.UserID

	ev := types.NewEmailVerification("email@formicary.io", u)
	_, err = verificationRepository.Create(ev)

	// THEN it should not fail
	require.NoError(t, err)

	// AND should find it by ID
	_, total, err := verificationRepository.Query(qc, make(map[string]interface{}), 0, 10, make([]string, 0))
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// AND should verify it
	saved, err := verificationRepository.Verify(qc, u.ID, ev.EmailCode)
	require.NoError(t, err)
	require.Equal(t, ev.EmailCode, saved.EmailCode)

	// AND should work with verify again
	saved, err = verificationRepository.Verify(qc, u.ID, ev.EmailCode)
	require.NoError(t, err)

	emails := verificationRepository.GetVerifiedEmails(qc, u.ID)
	require.Equal(t, 1, len(emails))
}
