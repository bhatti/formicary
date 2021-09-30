package queue

import (
	"context"
	"github.com/sirupsen/logrus"
	"sync"
)

// SendReceiveMultiplexer manages send/receive queues so that a single queue/topic can be used
// to receive messages that can be then handed out to proper recipients.
type SendReceiveMultiplexer struct {
	callbacksCorrelations map[string]callBackCoRelation
	lock                  sync.RWMutex
}

type callBackCoRelation struct {
	callback      Callback
	correlationID string
}

// NewSendReceiveMultiplexer constructor
func NewSendReceiveMultiplexer(id string, correlationID string, cb Callback) *SendReceiveMultiplexer {
	return &SendReceiveMultiplexer{
		callbacksCorrelations: map[string]callBackCoRelation{
			id: {correlationID: correlationID, callback: cb},
		},
	}
}

// Notify invokes callback methods
func (mx *SendReceiveMultiplexer) Notify(ctx context.Context, event *MessageEvent) int {
	mx.lock.RLock()
	defer mx.lock.RUnlock()
	for _, cbRel := range mx.callbacksCorrelations {
		go func(cbRel callBackCoRelation) {
			if cbRel.correlationID == "" || cbRel.correlationID == event.CoRelationID() {
				if err := cbRel.callback(ctx, event); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "SendReceiveMultiplexer",
						"Event":     event,
					}).Warnf("failed to notify event")
				}
			}
		}(cbRel)
	}
	return len(mx.callbacksCorrelations)
}

// Add adds callback with id
func (mx *SendReceiveMultiplexer) Add(id string, correlationID string, cb Callback) int {
	mx.lock.Lock()
	defer mx.lock.Unlock()
	mx.callbacksCorrelations[id] = callBackCoRelation{
		correlationID: correlationID,
		callback:      cb,
	}
	return len(mx.callbacksCorrelations)
}

// Remove removes callback
func (mx *SendReceiveMultiplexer) Remove(id string) int {
	mx.lock.Lock()
	defer mx.lock.Unlock()
	delete(mx.callbacksCorrelations, id)
	return len(mx.callbacksCorrelations)
}
