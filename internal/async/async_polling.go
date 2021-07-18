package async

import (
	"context"
	"time"
)

// type of function that is used to top repeated function
type pollingCompletionHandler func(
	ctx context.Context,
	payload interface{}) (bool, interface{}, error)

// pollingTask - processes data asynchronously
type pollingTask struct {
	completionHandler pollingCompletionHandler
	abortHandler      AbortHandler
	request           interface{}
	resultQ           chan PollingResponse
	pollInterval      time.Duration
	running           bool
}

// PollingResponse encapsulates results of async repeated task
type PollingResponse struct {
	Response
	Completed bool
}

// ExecutePolling executes a function repeatedly polls a background task until it is completed
func ExecutePolling(
	ctx context.Context,
	completionHandler pollingCompletionHandler,
	abortHandler AbortHandler,
	request interface{},
	pollInterval time.Duration) Awaiter {
	task := &pollingTask{
		request:           request,
		abortHandler:      abortHandler,
		completionHandler: completionHandler,
		resultQ:           make(chan PollingResponse, 1),
		pollInterval:      pollInterval,
		running:           true,
	}
	go task.run(ctx) // run handler asynchronously
	return task
}

// IsRunning checks if task is still running
func (t *pollingTask) IsRunning() bool {
	return t.running
}

// Await waits for completion of the task
func (t *pollingTask) Await(ctx context.Context) (result interface{}, err error) {
	result = nil
	select {
	case <-ctx.Done():
		err = ctx.Err()
		_, _ = t.abortHandler(ctx, t.request) // abortHandler operation
	case res := <-t.resultQ:
		result = res.Result
		err = res.Err
	}
	return
}

////////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////
func (t *pollingTask) invokeHandlerAndCheckCompletion(ctx context.Context) bool {
	var completed bool
	var result interface{}
	var err error
	if ctx.Err() != nil {
		completed = true
		err = ctx.Err()
	} else {
		completed, result, err = t.completionHandler(ctx, t.request)
	}

	if t.running && (completed || err != nil) {
		t.resultQ <- PollingResponse{
			Response:  Response{Result: result, Err: err},
			Completed: completed,
		}
		t.running = false
		close(t.resultQ) // notify wait task
		return completed
	}
	return false
}

func (t *pollingTask) run(ctx context.Context) {
	go func() {
		for {
			if t.invokeHandlerAndCheckCompletion(ctx) {
				break
			}
			select {
			case <-ctx.Done():
				break
			case <-time.After(t.pollInterval):
				continue
			}
		}
	}()
}
