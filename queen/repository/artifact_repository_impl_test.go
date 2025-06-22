package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if artifact doesn't exist
func Test_ShouldGetArtifactWithNonExistingId(t *testing.T) {
	// GIVEN artifact repository
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN loading nonexisting artifact
	_, err = repo.Get(qc, "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Saving a persistent artifact should succeed
func Test_ShouldSavingPersistentArtifact(t *testing.T) {
	// GIVEN artifact repository
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	expire := time.Now().Add(10 * time.Second)
	art := newTestArtifact(qc.User, expire)
	// WHEN Saving valid artifact
	saved, err := repo.Save(art)
	require.NoError(t, err)

	// THEN Retrieving should not fail
	loaded, err := repo.Get(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, expire.Unix(), loaded.ExpiresAt.Unix())
	require.Equal(t, len(art.Metadata), len(loaded.Metadata))
	require.Equal(t, 3, len(loaded.Metadata))
	require.Equal(t, "v1", loaded.Metadata["n1"])
	require.Equal(t, 2, len(loaded.Tags))
	require.Equal(t, "v1", loaded.Tags["t1"])
	// AND Retrieving by sha256 should not fail
	loaded, err = repo.GetBySHA256(qc, saved.SHA256)
	require.NoError(t, err)
	require.Equal(t, expire.Unix(), loaded.ExpiresAt.Unix())
	require.Equal(t, len(art.Metadata), len(loaded.Metadata))
	require.Equal(t, 3, len(loaded.Metadata))
	require.Equal(t, "v1", loaded.Metadata["n1"])
	require.Equal(t, 2, len(loaded.Tags))
	require.Equal(t, "v1", loaded.Tags["t1"])
}

// Deleting a persistent artifact should succeed
func Test_ShouldDeletingPersistentArtifact(t *testing.T) {
	// GIVEN artifact repository
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	art := newTestArtifact(qc.User, time.Now())
	// AND Saving valid artifact
	saved, err := repo.Save(art)
	require.NoError(t, err)

	// WHEN Deleting artifact by id
	err = repo.Delete(qc, saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// THEN Retrieving should fail
	_, err = repo.Get(qc, saved.ID)
	require.Error(t, err)
}

// Test GetResourceUsage usage
func Test_ShouldArtifactAccounting(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND creating a set of artifacts
	for i := 0; i < 10; i++ {
		art := newTestArtifact(qc.User, time.Now())
		art.ContentLength = 100
		// AND Saving valid artifact
		_, err = repo.Save(art)
		require.NoError(t, err)
	}
	// WHEN querying getting usage with nil range
	usage, err := repo.GetResourceUsage(qc, nil)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 0, len(usage))
	// WHEN querying getting usage with full range
	usage, err = repo.GetResourceUsage(qc, []types.DateRange{{
		StartDate: time.Now().Add(-1 * time.Minute),
		EndDate:   time.Now().Add(1 * time.Minute),
	}})
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 1, len(usage))
	require.Equal(t, 10, usage[0].Count)
	require.Equal(t, int64(1000), usage[0].Value)
}

// Test GetResourceUsage usage by org/user
func Test_ShouldArtifactAccountingByOrgUser(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND creating a set of artifacts
	for i := 0; i < 10; i++ {
		art := newTestArtifact(qc.User, time.Now())
		art.ContentLength = 100
		// AND Saving valid artifact
		_, err = repo.Save(art)
		require.NoError(t, err)
	}
	// WHEN querying getting usage with nil range
	usage, err := repo.GetResourceUsageByOrgUser(nil, 10000)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 0, len(usage))
	// WHEN querying getting usage with full range
	usage, err = repo.GetResourceUsageByOrgUser([]types.DateRange{{
		StartDate: time.Now().Add(-1 * time.Minute),
		EndDate:   time.Now().Add(1 * time.Minute),
	}}, 10000)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 1, len(usage))
	require.Equal(t, 10, usage[0].Count)
	require.Equal(t, int64(1000), usage[0].Value)
}

// Test SaveFile and query
func Test_ShouldSaveAndQueryArtifacts(t *testing.T) {
	// GIVEN artifact repository
	repo, err := NewTestArtifactRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of artifacts
	for i := 0; i < 10; i++ {
		group := fmt.Sprintf("group_%v", i)
		kind := fmt.Sprintf("kind_%v", i)
		for j := 0; j < 5; j++ {
			art := newTestArtifact(qc.User, time.Now())
			art.Kind = kind
			art.ArtifactGroup = group
			_, err = repo.Save(art)
			require.NoError(t, err)
		}
	}
	params := make(map[string]interface{})

	// WHEN querying artifacts
	_, total, err := repo.Query(qc, params, 0, 1000, []string{"id"})
	require.NoError(t, err)

	// THEN it should match expected count
	require.Equal(t, int64(50), total)

	// WHEN querying by kind
	params["kind"] = "kind_1"
	_, total, err = repo.Query(qc, params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	// THEN it should match expected count
	require.Equal(t, int64(5), total)

	// WHEN querying as a different user
	qc2, err := NewTestQC() // using different org/user
	require.NoError(t, err)
	_, total, err = repo.Query(qc2, params, 0, 1000, make([]string, 0))
	require.NoError(t, err)
	// THEN it should not return data
	require.Equal(t, int64(0), total)

	time.Sleep(1 * time.Millisecond)
	expired, err := repo.ExpiredArtifacts(qc, time.Millisecond, 1000)
	require.NoError(t, err)
	require.Equal(t, 50, len(expired))
}
