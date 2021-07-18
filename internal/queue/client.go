package queue

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/types"
	"time"
)

// Callback - callback method for consumer
type Callback func(ctx context.Context, event *MessageEvent) error

// AckHandler - handles Ack
type AckHandler func()

// MessageEvent structure
type MessageEvent struct {
	// Topic of message
	Topic string

	// ProducerName returns the name of the producer that has published the message.
	ProducerName string

	// Properties Return the properties attached to the message.
	Properties map[string]string

	// Payload get the payload of the message
	Payload []byte

	// ID get the unique message ID associated with this message.
	ID []byte

	// PublishTime get the publish time of this message.
	PublishTime time.Time

	// Ack call handles ack
	Ack AckHandler

	// Nack call handles nack
	Nack AckHandler
}

// Client interface for queuing messages
type Client interface {
	// Subscribe - nonblocking subscribe that calls back handler upon message
	Subscribe(
		ctx context.Context,
		topic string,
		id string,
		props map[string]string,
		shared bool,
		cb Callback) (err error)
	// UnSubscribe - unsubscribe
	UnSubscribe(_ context.Context, topic string, id string) (err error)
	// Send - sends one message and closes producer at the end
	Send(
		ctx context.Context,
		topic string,
		props map[string]string,
		payload []byte,
		reusableTopic bool) (messageID []byte, err error)
	// SendReceive - Send and receive message
	SendReceive(
		ctx context.Context,
		outTopic string,
		props map[string]string,
		payload []byte,
		inTopic string,
	) (event *MessageEvent, err error)
	// Publish - caches producer if doesn't exist and sends a message
	Publish(
		ctx context.Context,
		topic string,
		props map[string]string,
		payload []byte,
		disableBatching bool) (messageID []byte, err error)
	// Close - Closes all producers and consumers
	Close()
}

// NewMessagingClient creates new client for messaging
func NewMessagingClient(config *types.CommonConfig) (Client, error) {
	if config.MessagingProvider == types.RedisMessagingProvider {
		return newClientRedis(&config.Redis)
	} else if config.MessagingProvider == types.PulsarMessagingProvider {
		return newPulsarClient(&config.Pulsar)
	} else {
		return nil, fmt.Errorf("unsupported messaging provider %s", config.MessagingProvider)
	}
}
