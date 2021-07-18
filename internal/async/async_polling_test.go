package async

import (
	"context" //"strings"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldAsyncPollingWithTimeout(t *testing.T) {
	// GIVEN a polling handler that is executed 10 times
	poll := 1 * time.Millisecond
	timeout := 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	val := 0
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		val++
		return val >= 10, val, nil
	}

	// WHEN handler is executed with timeout
	future := ExecutePolling(ctx, handler, NoAbort, 0, poll)
	_, err := future.Await(ctx)
	// THEN it should fail with timeout error
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")
}

func Test_ShouldAsyncPollingWithFailure(t *testing.T) {
	// GIVEN a polling handler that returns error
	poll := 1 * time.Millisecond
	timeout := 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	val := 0
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		if val > 1 {
			return false, nil, errors.New("fake poll error")
		}
		val++
		return val >= 10, val, nil
	}

	// WHEN handler is executed with timeout
	future := ExecutePolling(ctx, handler, NoAbort, 0, poll)
	_, err := future.Await(ctx)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "fake poll")
}

func Test_ShouldAsyncPolling(t *testing.T) {
	// GIVEN a polling handler that completes within timeout
	poll := 1 * time.Millisecond
	timeout := 15 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	val := 0
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		val++
		return val >= 5, val, nil
	}

	// WHEN handler is executed with timeout
	future := ExecutePolling(ctx, handler, NoAbort, 0, poll)
	res, err := future.Await(ctx)
	// THEN it should succeed
	require.NoError(t, err)
	require.Equal(t, 5, res)
}
