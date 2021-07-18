package async

import (
	"context"
	"plexobject.com/formicary/internal/math"
	"time"
)

// pollingEventTask - processes data asynchronously with event
type pollingEventTask struct {
	pollingTask
	polls           time.Duration
	maxPollInterval time.Duration
	signalQ         chan struct{}
}

// SignalAwaiter - defines method to wait for result with signal for notification
type SignalAwaiter interface {
	Signal(ctx context.Context)
	Await(ctx context.Context) (interface{}, error)
	IsRunning() bool
}

// ExecutePollingWithSignal executes a function repeatedly polls a background task until it is completed and polling
// can be increased exponentially and triggered via event
func ExecutePollingWithSignal(
	ctx context.Context,
	completionHandler pollingCompletionHandler,
	abortHandler AbortHandler,
	request interface{},
	pollInterval time.Duration,
	maxPollInterval time.Duration) SignalAwaiter {
	task := &pollingEventTask{
		pollingTask: pollingTask{
			request:           request,
			abortHandler:      abortHandler,
			completionHandler: completionHandler,
			resultQ:           make(chan PollingResponse, 1),
			pollInterval:      pollInterval,
			running:           true,
		},
		maxPollInterval: maxPollInterval,
		signalQ:         make(chan struct{}, 1),
	}
	go task.run(ctx) // run handler asynchronously
	return task
}

// Signal notifies goroutine to poll
func (t *pollingEventTask) Signal(_ context.Context) {
	t.signalQ <- struct{}{}
}

////////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////
func (t *pollingEventTask) run(ctx context.Context) {
	go func() {
		for {
			t.polls++
			if t.invokeHandlerAndCheckCompletion(ctx) {
				break
			}
			select {
			case <-t.signalQ:
				continue
			case <-ctx.Done():
				break
			case <-time.After(math.MinDuration(t.maxPollInterval, t.polls*t.pollInterval)):
				continue
			}
		}
	}()
}
