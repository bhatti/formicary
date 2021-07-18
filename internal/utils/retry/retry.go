package retry

import (
	"time"

	"github.com/jpillora/backoff"
)

const (
	defaultRetryBackoffMin = 1 * time.Second
	defaultRetryBackoffMax = 5 * time.Second
)

// Handler - type of runner function
type Handler func() error

// Checker - checks if should retry
type Checker func(tries int, err error) bool

// Retry structure
type Retry struct {
	handler      Handler
	retryChecker Checker
	backoff      *backoff.Backoff
}

// New constructor
func New(handler Handler, retryChecker Checker) *Retry {
	return &Retry{
		handler:      handler,
		retryChecker: retryChecker,
		backoff:      &backoff.Backoff{Min: defaultRetryBackoffMin, Max: defaultRetryBackoffMax},
	}
}

// Run retries the handler
func (r *Retry) Run() error {
	var err error
	var tries int
	for {
		tries++
		err = r.handler()
		if err == nil || !r.retryChecker(tries, err) {
			break
		}

		time.Sleep(r.backoff.Duration())
	}

	return err
}
