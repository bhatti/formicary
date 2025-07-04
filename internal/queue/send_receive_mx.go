package queue

import (
	"context"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
	"runtime/debug"
	"sync"
	"time"
)

// SendReceiveMultiplexer manages send/receive queues so that a single queue/topic can be used
// to receive messages that can be then handed out to proper recipients.
type SendReceiveMultiplexer struct {
	created               time.Time
	topic                 string
	commitTimeout         time.Duration
	callbacksCorrelations map[string]callBackCoRelation
	lock                  sync.RWMutex
}

type callBackCoRelation struct {
	ctx             context.Context
	callback        Callback
	filter          Filter
	correlationID   string
	consumerChannel chan *MessageEvent
}

// NewSendReceiveMultiplexer constructor
func NewSendReceiveMultiplexer(
	_ context.Context,
	topic string,
	commitTimeout time.Duration,
) *SendReceiveMultiplexer {
	return &SendReceiveMultiplexer{
		created:               time.Now(),
		topic:                 topic,
		commitTimeout:         commitTimeout,
		callbacksCorrelations: make(map[string]callBackCoRelation),
	}
}

// Notify invokes callback methods
func (mx *SendReceiveMultiplexer) Notify(ctx context.Context,
	event *MessageEvent, ack AckHandler, nack AckHandler) int {
	// creating a local copy of subscribers so that we don't share instance variables during notification
	subscribers := mx.cloneSubscribers()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "SendReceiveMultiplexer",
			"Topic":         mx.topic,
			"ID":            string(event.ID),
			"CorrelationID": event.CoRelationID(),
			"Subscribers":   len(subscribers),
			"CommitTimeout": mx.commitTimeout,
			"Elapsed":       time.Since(mx.created),
		}).Debug("notifying subscribers")
	}
	// notifying subscribers
	sent := 0
	started := time.Now()
	ids := make([]string, 0)
	for id, next := range subscribers {
		ids = append(ids, id)
		if next.correlationID == "" || next.correlationID == event.CoRelationID() {
			sent++
			if next.filter == nil || next.filter(ctx, event) {
				if err := next.callback(ctx, event, ack, nack); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "SendReceiveMultiplexer",
						"Event":     event,
						"Topic":     mx.topic,
					}).Warnf("failed to notify event")
				}
				if next.consumerChannel != nil {
					// this is mainly used by send/receive and could be blocked if buffer is full in other use cases
					next.consumerChannel <- event
				}
			}
		} else {
			age := started.Unix() - event.PublishTime.Unix()
			if age > int64(mx.commitTimeout.Seconds()) {
				ack() // commit old messages to eliminate redelivery
			}
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component":      "SendReceiveMultiplexer",
					"Event":          string(event.Payload),
					"AgeSecs":        age,
					"CBCoRelationID": next.correlationID,
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
	filter Filter,
	consumerChannel chan *MessageEvent) (int, error) {
	if cb == nil {
		debug.PrintStack()
		return 0, types.NewValidationError("callback not specified")
	}
	if correlationID == "" {
		// TODO we won't have it for in-topic
	}
	mx.lock.Lock()
	defer mx.lock.Unlock()
	mx.callbacksCorrelations[id] = callBackCoRelation{
		ctx:             ctx,
		correlationID:   correlationID,
		callback:        cb,
		filter:          filter,
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
	return len(mx.callbacksCorrelations), nil
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
