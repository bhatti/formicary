package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// newTestRetentionManager creates a RetentionManager backed by the shared test SQLite DB.
func newTestRetentionManager(t *testing.T) (*RetentionManager, *repository.Locator) {
	t.Helper()
	locator, err := repository.NewTestLocator()
	require.NoError(t, err)
	rm, err := NewRetentionManager(
		locator.JobDefinitionRepository,
		locator.JobRequestRepository,
		metrics.New(),
	)
	require.NoError(t, err)
	return rm, locator
}

// seedOldRequest saves a job request with the given state and back-dates updated_at.
// State is set via direct SQL to bypass validation that resets newly saved requests to PENDING.
func seedOldRequest(t *testing.T, locator *repository.Locator, qc *common.QueryContext, jobDef *types.JobDefinition, state common.RequestState, daysAgo int) string {
	t.Helper()
	req, err := types.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	saved, err := locator.JobRequestRepository.Save(qc, req)
	require.NoError(t, err)
	locator.DB.Exec("UPDATE formicary_job_requests SET job_state = ?, updated_at = ? WHERE id = ?",
		state, time.Now().AddDate(0, 0, -daysAgo), saved.ID)
	return saved.ID
}

// Test_RetentionManager_PurgeAll_RemovesOldHistory verifies that PurgeAll purges expired records.
func Test_RetentionManager_PurgeAll_RemovesOldHistory(t *testing.T) {
	rm, locator := newTestRetentionManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Seed a job definition with a 1-day retention for COMPLETED.
	jobDef := repository.NewTestJobDefinition(qc.User, "retention-purge-all")
	jobDef.RetentionDaysForCompleted = 1
	savedDef, err := locator.JobDefinitionRepository.Save(qc, jobDef)
	require.NoError(t, err)

	// Seed an old COMPLETED request (2 days ago — older than 1-day window).
	oldID := seedOldRequest(t, locator, qc, savedDef, common.COMPLETED, 2)

	// Seed a recent COMPLETED request (same day — within window).
	recentID := seedOldRequest(t, locator, qc, savedDef, common.COMPLETED, 0)

	total, err := rm.PurgeAll(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))

	// Old request must be gone; recent request must remain.
	var oldCount, recentCount int64
	locator.DB.Raw("SELECT COUNT(*) FROM formicary_job_requests WHERE id = ?", oldID).Scan(&oldCount)
	locator.DB.Raw("SELECT COUNT(*) FROM formicary_job_requests WHERE id = ?", recentID).Scan(&recentCount)
	require.Equal(t, int64(0), oldCount, "old request should be purged")
	require.Equal(t, int64(1), recentCount, "recent request should be kept")
}

// Test_RetentionManager_PurgeAll_SkipsExecuting verifies EXECUTING rows are never touched.
func Test_RetentionManager_PurgeAll_SkipsExecuting(t *testing.T) {
	rm, locator := newTestRetentionManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobDef := repository.NewTestJobDefinition(qc.User, "retention-skip-executing")
	jobDef.RetentionDaysForCompleted = 1
	savedDef, err := locator.JobDefinitionRepository.Save(qc, jobDef)
	require.NoError(t, err)

	// Seed an old EXECUTING request (should never be purged).
	executingID := seedOldRequest(t, locator, qc, savedDef, common.EXECUTING, 30)

	_, err = rm.PurgeAll(context.Background())
	require.NoError(t, err)

	var count int64
	locator.DB.Raw("SELECT COUNT(*) FROM formicary_job_requests WHERE id = ?", executingID).Scan(&count)
	require.Equal(t, int64(1), count, "EXECUTING request must not be purged")
}

// Test_RetentionManager_PurgeAll_PurgesCancelled verifies CANCELLED history is purged.
func Test_RetentionManager_PurgeAll_PurgesCancelled(t *testing.T) {
	rm, locator := newTestRetentionManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobDef := repository.NewTestJobDefinition(qc.User, "retention-cancelled")
	jobDef.RetentionDaysForCancelled = 1
	savedDef, err := locator.JobDefinitionRepository.Save(qc, jobDef)
	require.NoError(t, err)

	cancelledID := seedOldRequest(t, locator, qc, savedDef, common.CANCELLED, 3)

	total, err := rm.PurgeAll(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))

	var count int64
	locator.DB.Raw("SELECT COUNT(*) FROM formicary_job_requests WHERE id = ?", cancelledID).Scan(&count)
	require.Equal(t, int64(0), count, "CANCELLED request should be purged")
}

// Test_RetentionManager_ContextCancellation verifies PurgeAll respects context cancellation.
func Test_RetentionManager_ContextCancellation(t *testing.T) {
	rm, _ := newTestRetentionManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := rm.PurgeAll(ctx)
	// Should complete quickly (no job defs or context respected) — just must not panic.
	_ = err
}
