package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	"plexobject.com/formicary/queen/types"
)

// Test SaveFile and query
func Test_ShouldSaveAndQueryAuditRecords(t *testing.T) {
	// GIVEN an audit repository
	repo, err := NewTestAuditRecordRepository()
	require.NoError(t, err)
	repo.clear()
	kinds := []types.AuditKind{
		types.JobRequestCreated,
		types.JobDefinitionUpdated,
		types.JobResourceUpdated,
		types.OrganizationUpdated}

	// AND an existing audit records
	for i := 0; i < 10; i++ {
		for j := 0; j < len(kinds); j++ {
			ec := types.NewAuditRecord(kinds[j], fmt.Sprintf("message %v-%v", i, j))
			_, err = repo.Save(ec)
			require.NoError(t, err)
		}
	}
	params := make(map[string]interface{})

	// WHEN querying audit records
	_, total, err := repo.Query(params, 0, 1000, []string{"id"})

	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(40), total)

	// WHEN querying by kind
	params["kind"] = "JOB_REQUEST_CREATED"
	_, total, err = repo.Query(params, 0, 1000, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
}
