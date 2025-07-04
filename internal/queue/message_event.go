package queue

import (
	"fmt"
	"time"
)

// constants
const (
	// DisableBatchingKey to disable batch send
	DisableBatchingKey = "DisableBatching"
	// ReusableTopicKey to cache producer
	ReusableTopicKey = "ReusableTopic"
	// MessageTarget of message
	MessageTarget = "MessageTarget"
	// CorrelationIDKey for send/receive
	CorrelationIDKey = "CorrelationID"
	replyTopicKey    = "ReplyTopic"
	messageKey       = "Key"
	producerKey      = "Producer"
	groupKey         = "ArtifactGroup"
	lastOffsetKey    = "lastOffset"
	firstOffsetKey   = "lastOffset"
)

// MessageHeaders for message headers
type MessageHeaders map[string]string

// NewMessageHeaders constructor
func NewMessageHeaders(keyValues ...string) MessageHeaders {
	res := make(MessageHeaders)
	for i := 0; i < len(keyValues)-1; i += 2 {
		res[keyValues[i]] = keyValues[i+1]
	}
	return res
}

// GetReplyTopic getter
func (h MessageHeaders) GetReplyTopic() string {
	return h[replyTopicKey]
}

// SetReplyTopic setter
func (h MessageHeaders) SetReplyTopic(val string) {
	h[replyTopicKey] = val
}

// GetCorrelationID getter
func (h MessageHeaders) GetCorrelationID() string {
	return h[CorrelationIDKey]
}

// SetCorrelationID setter
func (h MessageHeaders) SetCorrelationID(val string) {
	h[CorrelationIDKey] = val
}

// GetMessageKey getter
func (h MessageHeaders) GetMessageKey() string {
	return h[messageKey]
}

// SetMessageKey setter
func (h MessageHeaders) SetMessageKey(val string) {
	h[messageKey] = val
}

// GetProducer getter
func (h MessageHeaders) GetProducer() string {
	return h[producerKey]
}

// SetProducer setter
func (h MessageHeaders) SetProducer(val string) {
	h[producerKey] = val
}

// GetGroup getter
func (h MessageHeaders) GetGroup(defGroup string) string {
	group := h[groupKey]
	if group != "" {
		return group
	}
	return defGroup
}

// SetGroup setter
func (h MessageHeaders) SetGroup(val string) {
	h[groupKey] = val
}

// GetFirstOffset getter
func (h MessageHeaders) GetFirstOffset() string {
	return h[firstOffsetKey]
}

// SetFirstOffset setter
func (h MessageHeaders) SetFirstOffset(val string) {
	h[firstOffsetKey] = val
}

// GetLastOffset getter
func (h MessageHeaders) GetLastOffset() string {
	return h[lastOffsetKey]
}

// SetLastOffset setter
func (h MessageHeaders) SetLastOffset(val string) {
	h[lastOffsetKey] = val
}

// IsDisableBatching getter
func (h MessageHeaders) IsDisableBatching() bool {
	return h[DisableBatchingKey] == "true"
}

// SetDisableBatching setter
func (h MessageHeaders) SetDisableBatching(val bool) {
	h[DisableBatchingKey] = fmt.Sprintf("%v", val)
}

// IsReusableTopic getter
func (h MessageHeaders) IsReusableTopic() bool {
	return h[ReusableTopicKey] == "true" || h[ReusableTopicKey] == "" // default is true
}

// SetReusableTopic setter
func (h MessageHeaders) SetReusableTopic(val bool) {
	h[ReusableTopicKey] = fmt.Sprintf("%v", val)
}

// MessageEvent structure
type MessageEvent struct {
	// Topic of message
	Topic string

	// ProducerName returns the name of the producer that has published the message.
	ProducerName string

	// Properties Return the properties attached to the message.
	Properties MessageHeaders

	// Payload get the Payload of the message
	Payload []byte

	// ID get the unique message ID associated with this message.
	ID []byte

	// PublishTime get the publishing time of this message.
	PublishTime time.Time

	// Partition if available
	Partition int

	// Offset if available
	Offset int64
}

// CoRelationID returns correlation-id
func (e *MessageEvent) CoRelationID() string {
	return e.Properties.GetCorrelationID()
}

// ReplyTopic returns reply-topic
func (e *MessageEvent) ReplyTopic() string {
	return e.Properties.GetReplyTopic()
}

// Producer returns producer name
func (e *MessageEvent) Producer() string {
	return e.Properties.GetProducer()
}
