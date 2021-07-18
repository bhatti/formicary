package async

import (
	"context" //"strings"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldAsyncPollingEventWithTimeout(t *testing.T) {
	// GIVEN a handler that takes a very long time
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

	// WHEN handler is executed
	future := ExecutePollingWithSignal(ctx, handler, NoAbort, 0, poll, poll*10)
	_, err := future.Await(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")
}

func Test_ShouldAsyncPollingEventWithFailure(t *testing.T) {
	// GIVEN a handler that fails with poll error
	poll := 1 * time.Millisecond
	timeout := 10 * time.Millisecond
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

	// WHEN handler is executed
	future := ExecutePollingWithSignal(ctx, handler, NoAbort, 0, poll, poll*10)
	_, err := future.Await(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fake poll")
}

func Test_ShouldAsyncPollingEvent(t *testing.T) {
	// GIVEN a handler is executed 100 times
	started := time.Now()
	poll := 10 * time.Millisecond
	timeout := 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	val := 0
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		val++
		return val >= 100, val, nil
	}

	// WHEN handler is executed
	future := ExecutePollingWithSignal(ctx, handler, NoAbort, 0, poll, poll*100)
	go func() {
		for i:=0; i<100; i++ {
			future.Signal(ctx)
		}
	}()
	res, err := future.Await(ctx)
	require.NoError(t, err)
	require.Equal(t, 100, res)
	t.Logf("elapsed %v", time.Since(started))
}
