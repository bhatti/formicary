package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify table names for error-code and config
func Test_ShouldErrorCodeTableNames(t *testing.T) {
	record := NewErrorCode("*", "regex", "", "test-record")
	require.Equal(t, "formicary_error_codes", record.TableName())
}

// Validate error-code without job-type
func Test_ShouldErrorCodeWithoutJobType(t *testing.T) {
	// GIVEN
	// WHEN an error code is created without type
	record := NewErrorCode("", "regex", "", "xxx")
	err := record.ValidateBeforeSave()
	// THEN validation should not fail
	require.NoError(t, err)
}

// Validate error-code without regex
func Test_ShouldErrorCodeWithoutRegex(t *testing.T) {
	// GIVEN
	// WHEN an error code is created without regex
	record := NewErrorCode("*", "", "", "test-record")
	err := record.ValidateBeforeSave()
	// THEN validation should fail
	require.Error(t, err)
}

// Validate error-code without code
func Test_ShouldErrorCodeWithoutCode(t *testing.T) {
	// GIVEN
	// WHEN an error code is created without error-code
	record := NewErrorCode("*", "regex", "", "")
	err := record.ValidateBeforeSave()
	// THEN validation should fail
	require.Error(t, err)
}

// Validate happy path of Validate with proper error-code
func Test_ShouldWithGoodErrorCode(t *testing.T) {
	// GIVEN
	// WHEN an error code is created with all required fields
	record := NewErrorCode("*", "regex", "", "error-code")
	err := record.ValidateBeforeSave()
	// THEN validation should succeed
	require.NoError(t, err)
	require.Equal(t, "", record.ShortID())
	record.ID = "12345678901234"
	require.Equal(t, "12345678...", record.ShortID())
	require.False(t, record.Matches("msg"))
	require.True(t, record.Matches("regex1"))
}
