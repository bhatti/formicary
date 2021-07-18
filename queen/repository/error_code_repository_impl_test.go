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

	// WHEN finding non-existing error
	_, err = repo.Get("missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Deleting non-existing error-code should fail
func Test_ShouldDeleteByTypeErrorCodeWithNonExistingType(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)

	// WHEN deleting non-existing error
	err = repo.Delete("non-existing-error")

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

	// WHEN saving error code without regex
	ec := common.NewErrorCode("*", "", "ERR_BAD_1")
	_, err = repo.Save(ec)

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
	// WHEN saving error code without regex
	ec := common.NewErrorCode("*", "some exit", "")
	_, err = repo.Save(ec)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "errorCode")
}

// Saving valid error-code
func Test_ShouldSaveValidErrorCode(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)

	// Saving valid error-code
	ec := common.NewErrorCode("*", "exit-1", "ERR_ONE")
	saved, err := repo.Save(ec)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving error-code by id
	loaded, err := repo.Get(saved.ID)

	// THEN it should not fail
	require.NoError(t, err)

	// AND comparing saved object should match
	require.Equal(t, ec.ErrorCode, loaded.ErrorCode)

	ec.ErrorCode = "ERR_NEW"
	// WHEN updating error-code
	saved, err = repo.Save(ec)

	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving error-code by id
	loaded, err = repo.Get(saved.ID)
	require.NoError(t, err)
	// AND comparing saved object should match
	require.Equal(t, ec.ErrorCode, loaded.ErrorCode)

}

// Deleting a persistent error-code should succeed
func Test_ShouldDeletingPersistentErrorCode(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)

	// AND previously saved error code
	ec := common.NewErrorCode("*", "exit-2", "ERR_TWO")
	saved, err := repo.Save(ec)
	require.NoError(t, err)

	// WHEN deleting error-code by id
	err = repo.Delete(saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// AND retrieving should fail
	_, err = repo.Get(saved.ID)
	// THEN it should fail
	require.Error(t, err)
}

// Test GetAll error codes
func Test_ShouldGetAllErrorCodes(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()

	// AND a set of error-codes in the database
	for i := 0; i < 10; i++ {
		ec := common.NewErrorCode("*", fmt.Sprintf("exit-%v", i), fmt.Sprintf("ERR_ALL_%v", i))
		_, err = repo.Save(ec)
		require.NoError(t, err)
	}

	// WHEN finding all error codes
	all, err := repo.GetAll()
	require.NoError(t, err)
	// THEN it should return all
	require.Equal(t, 10, len(all))
}

// Test Query error codes
func Test_ShouldQueryAllErrorCodes(t *testing.T) {
	// GIVEN an error repository
	repo, err := NewTestErrorCodeRepository()
	require.NoError(t, err)
	repo.clear()

	// AND a set of error-codes in the database
	for i := 0; i < 10; i++ {
		ec := common.NewErrorCode("*", fmt.Sprintf("exit-%v", i), fmt.Sprintf("ERR_ALL_%v", i))
		_, err = repo.Save(ec)
		require.NoError(t, err)
	}

	// WHEN querying error codes
	all, _, err := repo.Query(make(map[string]interface{}), 0, 100, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(all))
}
