package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	common "plexobject.com/formicary/internal/types"
)

// Get operation should fail if error-code doesn't exist
func Test_ShouldGetErrorCodeWithNonExistingId(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN finding non-existing error
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing error-code should fail
func Test_ShouldDeleteByTypeErrorCodeWithNonExistingType(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN deleting non-existing error
	err = repo.Delete(qc, "non-existing-error")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Saving error-code without regex/exit-code should fail
func Test_ShouldSaveErrorCodeWithoutRegex(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN saving error code without regex
	ec := common.NewErrorCode("*", "", "", "ERR_BAD_1")
	_, err = repo.Save(qc, ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "regex")
}

// Saving error-code without code should fail
func Test_ShouldSaveErrorCodeWithoutCode(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN saving error code without regex
	ec := common.NewErrorCode("*", "some exit", "", "")
	_, err = repo.Save(qc, ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "errorCode")
}

// Saving valid error-code
func Test_ShouldSaveValidErrorCode(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	repo.clear()

	// Saving valid error-code
	ec := common.NewErrorCode("*", "exit-1", "", "ERR_ONE")
	ec.OrganizationID = qc.GetOrganizationID()
	ec.UserID = qc.GetUserID()
	saved, err := repo.Save(qc, ec)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving error-code by id
	loaded, err := repo.Get(qc, saved.ID)

	// THEN it should not fail
	require.NoError(t, err)

	// AND comparing saved object should match
	require.Equal(t, ec.ErrorCode, loaded.ErrorCode)

	ec.ErrorCode = "ERR_NEW"
	// WHEN updating error-code
	saved, err = repo.Save(qc, ec)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving error-code by id
	loaded, err = repo.Get(qc, saved.ID)
	require.NoError(t, err)
	// AND comparing saved object should match
	require.Equal(t, ec.ErrorCode, loaded.ErrorCode)

	qc2, err := NewTestQC()
	require.NoError(t, err)

	ec.ErrorCode = "ERR_NEW_AGAIN"
	// WHEN updating error-code with different user
	saved, err = repo.Save(qc2, ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "record not found")

}

// Deleting a persistent error-code should succeed
func Test_ShouldDeletingPersistentErrorCode(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND previously saved error code
	ec := common.NewErrorCode("*", "exit-2", "", "ERR_TWO")
	ec.OrganizationID = qc.GetOrganizationID()
	ec.UserID = qc.GetUserID()
	saved, err := repo.Save(qc, ec)
	require.NoError(t, err)

	// WHEN deleting error-code by id
	err = repo.Delete(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving should fail
	_, err = repo.Get(qc, saved.ID)
	// THEN it should fail
	require.Error(t, err)
}

// Test GetAll error codes
func Test_ShouldGetAllErrorCodes(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of error-codes in the database
	for i := 0; i < 40; i++ {
		ec := common.NewErrorCode(
			"*",
			fmt.Sprintf("exit-%v", i),
			fmt.Sprintf("cmd-%v", i),
			fmt.Sprintf("ERR_ALL_%v", i))
		if i < 10 {
			_, err = repo.Save(qc, ec)
		} else if i < 20 {
			_, err = repo.Save(common.NewQueryContextFromIDs("", "org"), ec)
		} else if i < 30 {
			_, err = repo.Save(common.NewQueryContextFromIDs("user", ""), ec)
		} else {
			_, err = repo.Save(common.NewQueryContextFromIDs("", ""), ec)
		}
		require.NoError(t, err)
	}

	// WHEN finding all error codes
	all, err := repo.GetAll(qc)
	require.NoError(t, err)
	// THEN it should return all
	require.Equal(t, 20, len(all))
}

// Test Query error codes
func Test_ShouldQueryAllErrorCodes(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of error-codes in the database
	for i := 0; i < 10; i++ {
		ec := common.NewErrorCode(
			"*",
			fmt.Sprintf("exit-%v", i),
			fmt.Sprintf("cmd-%v", i),
			fmt.Sprintf("ERR_ALL_%v", i))
		ec.OrganizationID = qc.GetOrganizationID()
		ec.UserID = qc.GetUserID()
		_, err = repo.Save(qc, ec)
		require.NoError(t, err)
	}

	// WHEN querying error codes
	all, _, err := repo.Query(
		qc,
		make(map[string]interface{}),
		0,
		100,
		make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(all))
}

func Test_ShouldMatchErrorCodes(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of error-codes in the database
	for i := 0; i < 10; i++ {
		ec := common.NewErrorCode(
			fmt.Sprintf("myjob-%v", i),
			fmt.Sprintf("regex exit-%v", i),
			fmt.Sprintf("curl url-%v", i),
			fmt.Sprintf("ERR_CODE_%v", i))
		if i%2 == 0 {
			ec.PlatformScope = "Linux"
		}
		ec.OrganizationID = qc.GetOrganizationID()
		ec.UserID = qc.GetUserID()
		_, err = repo.Save(qc, ec)
		require.NoError(t, err)
	}

	// WHEN matching error codes
	ec, err := repo.Match(
		qc,
		"regex exit-0 is here",
		"Linux",
		"mycurl url-0",
		"myjob-0",
		"")
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, "ERR_CODE_0", ec.ErrorCode)
}
