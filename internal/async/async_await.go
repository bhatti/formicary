package async

import (
	"context"
)

// Handler - type of async function
type Handler func(
	ctx context.Context,
	request interface{}) (interface{}, error)

// AbortHandler - type of abortHandler function that is called if async operation is cancelled
type AbortHandler func(
	ctx context.Context,
	request interface{}) (interface{}, error)

// NoAbort - default abort function
func NoAbort(
	context.Context,
	interface{}) (interface{}, error) {
	return nil, nil
}

// Awaiter - defines method to wait for result
type Awaiter interface {
	Await(ctx context.Context) (interface{}, error)
	IsRunning() bool
}

// task - submits task asynchronously
type task struct {
	handler      Handler
	abortHandler AbortHandler
	request      interface{}
	resultQ      chan Response
	running      bool
}

// Response encapsulates results of async task
type Response struct {
	Result interface{}
	Err    error
}

// Execute executes a long-running function in background and returns a future to wait for the response
func Execute(
	ctx context.Context,
	handler Handler,
	abortHandler AbortHandler,
	request interface{}) Awaiter {
	task := &task{
		request:      request,
		handler:      handler,
		abortHandler: abortHandler,
		resultQ:      make(chan Response, 1),
		running:      true,
	}
	go task.run(ctx) // run handler asynchronously
	return task
}

// IsRunning checks if task is still running
func (t *task) IsRunning() bool {
	return t.running
}

// Await waits for completion of the task
func (t *task) Await(ctx context.Context) (result interface{}, err error) {
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

// AwaitAll waits for completion of multiple tasks
func AwaitAll(ctx context.Context, all ...Awaiter) []Response {
	results := make([]Response, 0)
	for _, next := range all {
		res, err := next.Await(ctx)
		results = append(results, Response{Result: res, Err: err})
	}
	return results
}

////////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////
func (t *task) run(ctx context.Context) {
	go func() {
		result, err := t.handler(ctx, t.request)
		t.resultQ <- Response{Result: result, Err: err} // out channel is buffered by 1
		t.running = false
		close(t.resultQ)
	}()
}
