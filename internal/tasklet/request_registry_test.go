package tasklet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/types"
)

func newTestRegistry(t *testing.T) RequestRegistry {
	t.Helper()
	cfg := newTestCommonConfig()
	return NewRequestRegistry(cfg, metrics.New())
}

func newTestRequest(jobID, taskType string) *types.TaskRequest {
	req := &types.TaskRequest{
		JobRequestID: jobID,
		TaskType:     taskType,
		TaskExecutionID: jobID + "-" + taskType,
	}
	ctx, cancel := context.WithCancel(context.Background())
	req.Cancel = cancel
	_ = ctx // held alive by cancel
	return req
}

// ─── Add / Remove ────────────────────────────────────────────────────────────

func Test_ShouldAddAndRemoveRequest(t *testing.T) {
	r := newTestRegistry(t)
	req := newTestRequest("job-1", "build")

	require.NoError(t, r.Add(req))
	assert.Equal(t, 1, r.Count())

	require.NoError(t, r.Remove(req))
	assert.Equal(t, 0, r.Count())
}

func Test_ShouldRejectDuplicateAdd(t *testing.T) {
	r := newTestRegistry(t)
	req := newTestRequest("job-1", "build")
	require.NoError(t, r.Add(req))
	require.Error(t, r.Add(req), "duplicate add should fail")
}

// ─── Cancel (single task) ────────────────────────────────────────────────────

func Test_ShouldCancelSingleTask(t *testing.T) {
	r := newTestRegistry(t)
	req := newTestRequest("job-1", "build")
	require.NoError(t, r.Add(req))

	cancelled := make(chan struct{})
	origCancel := req.Cancel
	req.Cancel = func() {
		origCancel()
		close(cancelled)
	}

	require.NoError(t, r.Cancel(req.Key()))
	select {
	case <-cancelled:
	default:
		t.Fatal("cancel function was not called")
	}
	assert.True(t, req.Cancelled, "Cancelled flag must be set")
}

func Test_ShouldFailCancelUnknownKey(t *testing.T) {
	r := newTestRegistry(t)
	require.Error(t, r.Cancel("nonexistent-key"))
}

// ─── CancelJob (all tasks for a job) ─────────────────────────────────────────

func Test_ShouldCancelAllTasksForJob(t *testing.T) {
	r := newTestRegistry(t)

	// two tasks belonging to the same job
	req1 := newTestRequest("job-2", "build")
	req2 := newTestRequest("job-2", "test")
	require.NoError(t, r.Add(req1))
	require.NoError(t, r.Add(req2))

	var cancelled1, cancelled2 bool
	orig1, orig2 := req1.Cancel, req2.Cancel
	req1.Cancel = func() { orig1(); cancelled1 = true }
	req2.Cancel = func() { orig2(); cancelled2 = true }

	require.NoError(t, r.CancelJob("job-2"))

	assert.True(t, cancelled1, "task 1 cancel must be called")
	assert.True(t, cancelled2, "task 2 cancel must be called")
	assert.True(t, req1.Cancelled, "task 1 Cancelled flag must be set")
	assert.True(t, req2.Cancelled, "task 2 Cancelled flag must be set")
	assert.Equal(t, 0, r.Count(), "cancelled tasks must be removed from registry")
}

func Test_ShouldNotCancelOtherJobsTasks(t *testing.T) {
	r := newTestRegistry(t)

	// task for "job-A" — should be cancelled
	reqA := newTestRequest("job-A", "build")
	// task for "job-B" — must NOT be touched
	reqB := newTestRequest("job-B", "build")
	require.NoError(t, r.Add(reqA))
	require.NoError(t, r.Add(reqB))

	var cancelledB bool
	origB := reqB.Cancel
	reqB.Cancel = func() { origB(); cancelledB = true }

	require.NoError(t, r.CancelJob("job-A"))

	assert.False(t, cancelledB, "job-B must not be cancelled")
	assert.False(t, reqB.Cancelled)
	assert.True(t, reqA.Cancelled, "job-A must be cancelled")
	// CancelJob deletes cancelled tasks from the map immediately
	assert.Equal(t, 1, r.Count(), "only job-B task remains in registry")
}

func Test_ShouldSucceedWhenNoTasksFoundForJob(t *testing.T) {
	// Cancel after completion is a valid race — should not error
	r := newTestRegistry(t)
	require.NoError(t, r.CancelJob("nonexistent-job"))
}

// ─── GetAllocations ───────────────────────────────────────────────────────────

func Test_ShouldReturnAllocations(t *testing.T) {
	r := newTestRegistry(t)
	req := newTestRequest("job-3", "deploy")
	require.NoError(t, r.Add(req))

	allocs := r.GetAllocations()
	assert.Contains(t, allocs, "job-3")
}
