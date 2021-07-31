package types

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
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

func Test_ShouldCreateAuditRecordFromResource(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromJobResource(&JobResource{}, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromJobDefinition(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromJobDefinition(&JobDefinition{}, JobDefinitionUpdated, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromJobDefinitionConfig(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromJobDefinitionConfig(&JobDefinitionConfig{}, JobDefinitionUpdated, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromJobRequest(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromJobRequest(&JobRequest{}, JobRequestCreated, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromInvitation(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromInvite(&UserInvitation{}, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromToken(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromToken(&UserToken{}, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromUser(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromUser(&common.User{}, UserUpdated, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromOrganization(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromOrganization(&common.Organization{}, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromOrganizationConfig(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromOrganizationConfig(&common.OrganizationConfig{}, &common.QueryContext{}))
}

func Test_ShouldCreateAuditRecordFromSubscription(t *testing.T) {
	require.NotNil(t, NewAuditRecordFromSubscription(&common.Subscription{}, &common.QueryContext{}))
}
