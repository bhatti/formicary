package async

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldAsyncRacerWithTimeout(t *testing.T) {
	// GIVEN a handlers that takes a very long time
	timeout := 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler1 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return 1, nil
	}
	handler2 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return 2, nil
	}

	// WHEN handlers are executed with racer
	f, _ := ExecuteRacer(ctx, handler1, handler2)
	_, err := f.Await(ctx)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")
}

func Test_ShouldAsyncRacerWithFirstWinner(t *testing.T) {
	// GIVEN a handlers where first takes short time
	timeout := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler1 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		return 1, nil
	}
	handler2 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		return 2, nil
	}

	// WHEN handlers are executed with racer
	f, _ := ExecuteRacer(ctx, handler1, handler2)
	r, err := f.Await(ctx)
	require.NoError(t, err)
	// THEN first should return
	require.Equal(t, 1, r)
}

func Test_ShouldAsyncRacerWithSecondWinner(t *testing.T) {
	// GIVEN a handlers where second takes short time
	timeout := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler1 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return 5, nil
	}
	handler2 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		return 2, nil
	}

	// WHEN handlers are executed with racer
	f, _ := ExecuteRacer(ctx, handler1, handler2)
	r, err := f.Await(ctx)
	require.NoError(t, err)
	// THEN second should return
	require.Equal(t, 2, r)
}

func Test_ShouldAsyncRacerWithFirstFailure(t *testing.T) {
	// GIVEN a handlers where first fails first
	timeout := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler1 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		return 1, errors.New("first failure")
	}
	handler2 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		return 2, nil
	}

	// WHEN handlers are executed with racer
	f, _ := ExecuteRacer(ctx, handler1, handler2)
	_, err := f.Await(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "first failure")
}

func Test_ShouldAsyncRacerWithSecondFailure(t *testing.T) {
	// GIVEN a handlers where second fails first
	timeout := 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	handler1 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return 1, errors.New("first failure")
	}
	handler2 := func(ctx context.Context) (interface{}, error) {
		time.Sleep(2 * time.Millisecond)
		return 2, errors.New("second failure")
	}

	// WHEN handlers are executed with racer
	f, _ := ExecuteRacer(ctx, handler1, handler2)
	_, err := f.Await(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "second failure")
}
