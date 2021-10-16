package queue

import (
	"context"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

// SendReceiveMultiplexer manages send/receive queues so that a single queue/topic can be used
// to receive messages that can be then handed out to proper recipients.
type SendReceiveMultiplexer struct {
	topic                 string
	commitTimeoutSecs     int64
	callbacksCorrelations map[string]callBackCoRelation
	lock                  sync.RWMutex
}

type callBackCoRelation struct {
	ctx             context.Context
	callback        Callback
	correlationID   string
	consumerChannel chan *MessageEvent
}

// NewSendReceiveMultiplexer constructor
func NewSendReceiveMultiplexer(
	ctx context.Context,
	topic string,
	commitTimeoutSecs int64,
) *SendReceiveMultiplexer {
	return &SendReceiveMultiplexer{
		topic:                 topic,
		commitTimeoutSecs:     commitTimeoutSecs,
		callbacksCorrelations: make(map[string]callBackCoRelation),
	}
}

// Notify invokes callback methods
func (mx *SendReceiveMultiplexer) Notify(ctx context.Context, event *MessageEvent) int {
	// creating a local copy of subscribers so that we don't share instance variables during notification
	subscribers := mx.cloneSubscribers()

	// notifying subscribers
	sent := 0
	started := time.Now()
	for _, cbRel := range subscribers {
		if cbRel.correlationID == "" || cbRel.correlationID == event.CoRelationID() {
			sent++
			if err := cbRel.callback(ctx, event); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "SendReceiveMultiplexer",
					"Event":     event,
					"Topic":     mx.topic,
				}).Warnf("failed to notify event")
			}
			if cbRel.consumerChannel != nil {
				// this is mainly used by send/receive and could be blocked if buffer is full in other use cases
				cbRel.consumerChannel <- event
			}
		} else {
			age := started.Unix() - event.PublishTime.Unix()
			if age > mx.commitTimeoutSecs {
				event.Ack() // commit old messages to eliminate redelivery
			}
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component":      "SendReceiveMultiplexer",
					"Event":          event,
					"AgeSecs":        age,
					"CBCoRelationID": cbRel.correlationID,
					"CoRelationID":   event.CoRelationID(),
					"Topic":          mx.topic,
				}).Debugf("skip notify event")
			}
		}
	}
	return sent
}

// Add adds callback with id
func (mx *SendReceiveMultiplexer) Add(
	ctx context.Context,
	id string,
	correlationID string,
	cb Callback,
	consumerChannel chan *MessageEvent) int {
	mx.lock.Lock()
	defer mx.lock.Unlock()
	mx.callbacksCorrelations[id] = callBackCoRelation{
		ctx:             ctx,
		correlationID:   correlationID,
		callback:        cb,
		consumerChannel: consumerChannel,
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				mx.Remove(id)
				return
			}
		}
	}()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "SendReceiveMultiplexer",
			"ID":           id,
			"CoRelationID": correlationID,
			"Len":          len(mx.callbacksCorrelations),
			"Topic":        mx.topic,
		}).Debugf("added multiplex subscriber")
	}
	return len(mx.callbacksCorrelations)
}

// SubscriberIDs returns subscriberIDs
func (mx *SendReceiveMultiplexer) SubscriberIDs() (res []string) {
	mx.lock.RLock()
	defer mx.lock.RUnlock()
	for id := range mx.callbacksCorrelations {
		res = append(res, id)
	}
	return
}

func (mx *SendReceiveMultiplexer) cloneSubscribers() (res map[string]callBackCoRelation) {
	mx.lock.RLock()
	defer mx.lock.RUnlock()
	res = make(map[string]callBackCoRelation)
	for k, v := range mx.callbacksCorrelations {
		res[k] = v
	}
	return
}

// Remove removes callback
func (mx *SendReceiveMultiplexer) Remove(id string) int {
	mx.lock.Lock()
	defer mx.lock.Unlock()
	old := mx.callbacksCorrelations[id]
	if old.consumerChannel != nil {
		close(old.consumerChannel)
		old.consumerChannel = nil
	}
	delete(mx.callbacksCorrelations, id)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "SendReceiveMultiplexer",
			"ID":           id,
			"CoRelationID": old.correlationID,
			"Len":          len(mx.callbacksCorrelations),
			"Topic":        mx.topic,
		}).Debugf("removed multiplex subscriber")
	}
	return len(mx.callbacksCorrelations)
}
