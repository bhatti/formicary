package queue

import (
	"encoding/json"
	"fmt"
	"time"
)

// WSFrameType enumerates WebSocket message frame types
type WSFrameType string

const (
	// WSFrameSubscribe sent by ant to subscribe to a topic
	WSFrameSubscribe WSFrameType = "subscribe"
	// WSFrameUnsubscribe sent by ant to unsubscribe from a topic
	WSFrameUnsubscribe WSFrameType = "unsubscribe"
	// WSFramePublish sent by either side to deliver a message on a topic
	WSFramePublish WSFrameType = "publish"
	// WSFrameAck sent to acknowledge a publish frame
	WSFrameAck WSFrameType = "ack"
	// WSFrameNack sent to negatively acknowledge a publish frame (request redelivery)
	WSFrameNack WSFrameType = "nack"
	// WSFramePing sent for keepalive
	WSFramePing WSFrameType = "ping"
	// WSFramePong sent in reply to ping
	WSFramePong WSFrameType = "pong"
	// WSFrameError sent to report an error in response to another frame
	WSFrameError WSFrameType = "error"
	// WSFrameSubscribeAck sent by queen to confirm subscription
	WSFrameSubscribeAck WSFrameType = "subscribe_ack"
)

// WSFrame is the envelope for all messages exchanged over a WebSocket queue connection.
// Both queen-side and ant-side use this same structure.
type WSFrame struct {
	// Type identifies the frame purpose
	Type WSFrameType `json:"type"`

	// ID is a unique frame identifier used to correlate acks/nacks with the original frame
	ID string `json:"id"`

	// Topic is the queue topic for publish/subscribe frames
	Topic string `json:"topic,omitempty"`

	// SubscriptionID is assigned by the server on subscribe_ack and used by the server
	// when delivering messages to identify which subscription matched
	SubscriptionID string `json:"sub_id,omitempty"`

	// Payload is the raw message body
	Payload []byte `json:"payload,omitempty"`

	// Properties are the message headers (CorrelationID, ReplyTopic, etc.)
	Properties MessageHeaders `json:"properties,omitempty"`

	// Group is the subscription consumer group
	Group string `json:"group,omitempty"`

	// Shared indicates whether this is a shared subscription
	Shared bool `json:"shared,omitempty"`

	// Error contains an error description for error frames
	Error string `json:"error,omitempty"`

	// Timestamp is when the frame was created
	Timestamp time.Time `json:"ts"`
}

// MarshalWSFrame serialises a WSFrame to JSON bytes
func MarshalWSFrame(f *WSFrame) ([]byte, error) {
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now()
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("marshal WSFrame: %w", err)
	}
	return b, nil
}

// UnmarshalWSFrame deserialises JSON bytes into a WSFrame
func UnmarshalWSFrame(data []byte) (*WSFrame, error) {
	var f WSFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("unmarshal WSFrame: %w", err)
	}
	return &f, nil
}

// newSubscribeFrame builds a subscribe frame
func newSubscribeFrame(id, topic, group string, shared bool) *WSFrame {
	return &WSFrame{
		Type:      WSFrameSubscribe,
		ID:        id,
		Topic:     topic,
		Group:     group,
		Shared:    shared,
		Timestamp: time.Now(),
	}
}

// newUnsubscribeFrame builds an unsubscribe frame
func newUnsubscribeFrame(id, topic, subID string) *WSFrame {
	return &WSFrame{
		Type:           WSFrameUnsubscribe,
		ID:             id,
		Topic:          topic,
		SubscriptionID: subID,
		Timestamp:      time.Now(),
	}
}

// newPublishFrame builds a publish/deliver frame
func newPublishFrame(id, topic string, payload []byte, props MessageHeaders) *WSFrame {
	return &WSFrame{
		Type:      WSFramePublish,
		ID:        id,
		Topic:     topic,
		Payload:   payload,
		Properties: props,
		Timestamp: time.Now(),
	}
}

// newAckFrame builds an ack frame referencing the given frameID
func newAckFrame(frameID string) *WSFrame {
	return &WSFrame{
		Type:      WSFrameAck,
		ID:        frameID,
		Timestamp: time.Now(),
	}
}

// newNackFrame builds a nack frame referencing the given frameID
func newNackFrame(frameID string) *WSFrame {
	return &WSFrame{
		Type:      WSFrameNack,
		ID:        frameID,
		Timestamp: time.Now(),
	}
}

// newErrorFrame builds an error frame referencing the given frameID
func newErrorFrame(frameID, errMsg string) *WSFrame {
	return &WSFrame{
		Type:      WSFrameError,
		ID:        frameID,
		Error:     errMsg,
		Timestamp: time.Now(),
	}
}

// newPingFrame builds a ping frame
func newPingFrame(id string) *WSFrame {
	return &WSFrame{
		Type:      WSFramePing,
		ID:        id,
		Timestamp: time.Now(),
	}
}

// newPongFrame builds a pong frame
func newPongFrame(id string) *WSFrame {
	return &WSFrame{
		Type:      WSFramePong,
		ID:        id,
		Timestamp: time.Now(),
	}
}
