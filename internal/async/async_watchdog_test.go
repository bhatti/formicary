package async

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldAsyncWatchdogWithTimeout(t *testing.T) {
	// GIVEN a handler that takes a very long time
	poll := 1 * time.Millisecond
	timeout := 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}

	errorHandler := func(ctx context.Context, payload interface{}) error {
		return nil
	}

	// WHEN handler is executed with watchdog
	future := ExecuteWatchdog(ctx, handler, errorHandler, NoAbort, 0, poll)
	_, err := future.Await(ctx)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")
}

func Test_ShouldAsyncWatchdogWithMainFailure(t *testing.T) {
	// GIVEN a handler that fails with error
	poll := 1 * time.Millisecond
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(5 * time.Millisecond)
		return 0, errors.New("fake main error")
	}
	errorHandler := func(ctx context.Context, payload interface{}) error {
		return nil
	}

	// WHEN handler is executed with watchdog
	future := ExecuteWatchdog(ctx, handler, errorHandler, NoAbort, 0, poll)
	_, err := future.Await(ctx)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "fake main")
}

func Test_ShouldAsyncWatchdogWithWatchdogFailure(t *testing.T) {
	// GIVEN a handler that takes a very long time
	poll := 1 * time.Millisecond
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(25 * time.Millisecond)
		return 20, nil
	}
	errorHandler := func(ctx context.Context, payload interface{}) error {
		return errors.New("watchdog error")
	}

	// WHEN handler is executed with watchdog
	future := ExecuteWatchdog(ctx, handler, errorHandler, NoAbort, 0, poll)
	_, err := future.Await(ctx)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "watchdog error")
}

func Test_ShouldAsyncWatchdog(t *testing.T) {
	// GIVEN a handler that takes a short time
	poll := 1 * time.Millisecond
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		return 10, nil
	}
	errorHandler := func(ctx context.Context, payload interface{}) error {
		return nil
	}

	// WHEN handler is executed with watchdog
	future := ExecuteWatchdog(ctx, handler, errorHandler, NoAbort, 0, poll)
	res, err := future.Await(ctx)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 10, res)
}
