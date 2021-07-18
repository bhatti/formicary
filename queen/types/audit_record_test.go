package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify table names for audit-record and config
func Test_ShouldAuditRecordTableNames(t *testing.T) {
	record := NewAuditRecord(JobDefinitionUpdated, "test-record")
	require.Equal(t, "formicary_audit_records", record.TableName())
}

// Validate audit-record without kind
func Test_ShouldNotValidateAuditRecordWithoutKind(t *testing.T) {
	record := NewAuditRecord("", "test-record")
	// WHEN validating without kind
	err := record.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "kind")
}

// Validate audit-record without message
func Test_ShouldNotValidateAuditRecordWithoutMessage(t *testing.T) {
	record := NewAuditRecord("JOB", "")
	// WHEN validating without message
	err := record.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "message")
}

// Validate happy path of Validate with proper audit-record
func Test_ShouldWithGoodAuditRecord(t *testing.T) {
	record := NewAuditRecord(JobDefinitionUpdated, "test-record")
	err := record.ValidateBeforeSave()
	// THEN it should not fail
	require.NoError(t, err)
}
