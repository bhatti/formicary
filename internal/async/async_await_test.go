package async

import (
	"context" //"strings"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func fib(n uint) uint64 {
	if n <= 1 {
		return uint64(n)
	}
	return fib(n-1) + fib(n-2)
}

func Test_ShouldAsyncWithSleep(t *testing.T) {
	// GIVEN a handler that takes a very long time
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return fib(10000), nil
	}
	// WHEN handler is executed
	future := Execute(ctx, handler, NoAbort, 10)

	// AND is awaited for completio
	_, err := future.Await(ctx)
	elapsed := time.Since(started)
	t.Logf("TestWithSleep took %s", elapsed)
	// THEN it should fail with deadline exceeded
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")
}

func Test_ShouldAsyncWithCancel(t *testing.T) {
	// GIVEN a handler that sleeps for a short time
	started := time.Now()
	timeout := 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		return 0, nil
	}
	// WHEN handler is executed
	future := Execute(ctx, handler, NoAbort, 0)
	// AND is cancelled
	cancel()

	// THEN await should fail
	_, err := future.Await(ctx)
	elapsed := time.Since(started)
	t.Logf("TestAsyncWithCancel took %s -- %v", elapsed, err)
	require.Error(t, err)
}

func Test_ShouldAsyncWithTimeout(t *testing.T) {
	// GIVEN a handler that sleeps for a short time
	started := time.Now()
	timeout := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		val := payload.(int)
		if val%2 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
		return val * val, nil
	}
	futures := make([]Awaiter, 0)

	// WHEN a set of executors are created
	for i := 1; i <= 4; i++ {
		future := Execute(ctx, handler, NoAbort, i)
		futures = append(futures, future)
	}
	sum := 0
	var savedError error

	// AND is then awaited for all executors
	for i := 0; i < len(futures); i++ {
		res, err := futures[i].Await(ctx)
		if res != nil {
			sum += res.(int)
		} else if err != nil {
			savedError = err
		}
	}
	elapsed := time.Since(started)
	t.Logf("TestWithTimeout took %s", elapsed)
	// THEN it should fail with timeout error
	require.Error(t, savedError)
	require.Contains(t, savedError.Error(), "deadline")
}

func Test_ShouldAsyncWithFailure(t *testing.T) {
	// GIVEN a handler that fails for even value
	started := time.Now()
	timeout := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		val := payload.(int)
		if val%2 == 0 {
			return nil, errors.New("fake even error")
		}
		return val * val, nil
	}

	// WHEN futures are created for the execution
	futures := make([]Awaiter, 0)
	for i := 1; i <= 4; i++ {
		future := Execute(ctx, handler, NoAbort, i)
		futures = append(futures, future)
	}
	sum := 0
	var savedError error
	for i := 0; i < len(futures); i++ {
		res, err := futures[i].Await(ctx)
		if res != nil {
			sum += res.(int)
		} else if err != nil {
			savedError = err
		}
	}
	elapsed := time.Since(started)
	t.Logf("TestWithFailure took %s", elapsed)
	expected := 1 + 9

	// THEN it should fail with fake even
	require.Equal(t, expected, sum)
	require.Error(t, savedError)
	require.Contains(t, savedError.Error(), "fake even")
}

func Test_ShouldAsync(t *testing.T) {
	// GIVEN a handler that multiplies value
	started := time.Now()
	timeout := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		val := payload.(int)
		return val * val, nil
	}
	// WHEN futures are created for the execution
	futures := make([]Awaiter, 0)
	for i := 1; i <= 4; i++ {
		future := Execute(ctx, handler, NoAbort, i)
		futures = append(futures, future)
	}
	sum := 0

	// AND awaited
	results := AwaitAll(ctx, futures...)
	for _, res := range results {
		sum += res.Result.(int)
	}

	// THEN it sould return correct value
	elapsed := time.Since(started)
	t.Logf("Test took %s", elapsed)
	expected := 1 + 4 + 9 + 16
	require.Equal(t, expected, sum)
}
