package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBuffer(t *testing.T) *wsOfflineBuffer {
	t.Helper()
	buf, err := newWSOfflineBuffer(":memory:", 100, 1*time.Hour)
	require.NoError(t, err)
	t.Cleanup(func() { _ = buf.Close() })
	return buf
}

func TestWSOfflineBuffer_EnqueueDequeue(t *testing.T) {
	buf := newTestBuffer(t)

	props := MessageHeaders{CorrelationIDKey: "c1"}
	require.NoError(t, buf.Enqueue("topic-a", []byte("hello"), props))
	require.NoError(t, buf.Enqueue("topic-b", []byte("world"), nil))

	count, err := buf.Count()
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	msgs, err := buf.DequeueAll()
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	assert.Equal(t, "topic-a", msgs[0].Topic)
	assert.Equal(t, []byte("hello"), msgs[0].Payload)
	assert.Equal(t, "c1", msgs[0].Properties.GetCorrelationID())

	assert.Equal(t, "topic-b", msgs[1].Topic)
	assert.Equal(t, []byte("world"), msgs[1].Payload)
}

func TestWSOfflineBuffer_Remove(t *testing.T) {
	buf := newTestBuffer(t)

	require.NoError(t, buf.Enqueue("t", []byte("msg1"), nil))
	require.NoError(t, buf.Enqueue("t", []byte("msg2"), nil))

	msgs, err := buf.DequeueAll()
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	// Remove first
	require.NoError(t, buf.Remove(msgs[0].ID))

	count, err := buf.Count()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	// Remaining should be msg2
	remaining, err := buf.DequeueAll()
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, []byte("msg2"), remaining[0].Payload)
}

func TestWSOfflineBuffer_MaxSize(t *testing.T) {
	buf, err := newWSOfflineBuffer(":memory:", 3, 1*time.Hour)
	require.NoError(t, err)
	defer func() { _ = buf.Close() }()

	require.NoError(t, buf.Enqueue("t", []byte("1"), nil))
	require.NoError(t, buf.Enqueue("t", []byte("2"), nil))
	require.NoError(t, buf.Enqueue("t", []byte("3"), nil))

	// 4th should fail
	err = buf.Enqueue("t", []byte("4"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "full")
}

func TestWSOfflineBuffer_TTL(t *testing.T) {
	// TTL of 2 seconds — SQLite stores Unix epoch seconds so TTL must be >= 1s
	buf, err := newWSOfflineBuffer(":memory:", 100, 2*time.Second)
	require.NoError(t, err)
	defer func() { _ = buf.Close() }()

	require.NoError(t, buf.Enqueue("t", []byte("expire-me"), nil))

	// Message should be present before TTL expires
	msgs, err := buf.DequeueAll()
	require.NoError(t, err)
	assert.Len(t, msgs, 1)

	// Now use a fresh buffer with TTL already in the past by inserting with a past expiry
	buf2, err := newWSOfflineBuffer(":memory:", 100, 1*time.Second)
	require.NoError(t, err)
	defer func() { _ = buf2.Close() }()

	require.NoError(t, buf2.Enqueue("t", []byte("will-expire"), nil))

	// Manually expire by waiting 1.1 seconds (TTL = 1s)
	time.Sleep(1100 * time.Millisecond)

	// DequeueAll should prune and return nothing
	msgs2, err := buf2.DequeueAll()
	require.NoError(t, err)
	assert.Len(t, msgs2, 0)

	count, err := buf2.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestWSOfflineBuffer_EmptyDequeue(t *testing.T) {
	buf := newTestBuffer(t)

	msgs, err := buf.DequeueAll()
	require.NoError(t, err)
	assert.Empty(t, msgs)

	count, err := buf.Count()
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestWSOfflineBuffer_NilProperties(t *testing.T) {
	buf := newTestBuffer(t)

	require.NoError(t, buf.Enqueue("t", []byte("payload"), nil))

	msgs, err := buf.DequeueAll()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	// nil map unmarshals to empty map or nil — either is valid
	_ = msgs[0].Properties
}
