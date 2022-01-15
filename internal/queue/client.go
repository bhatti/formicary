package queue

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/types"
)

// constants
const (
	// DisableBatchingKey to disable batch send
	DisableBatchingKey = "DisableBatching"
	// ReusableTopicKey to cache producer
	ReusableTopicKey = "ReusableTopic"
	// Source to of message
	Source = "Source"
	// CorrelationIDKey for send/receive
	CorrelationIDKey = "CorrelationID"
	replyTopicKey    = "ReplyTopic"
	messageKey       = "Key"
	producerKey      = "Producer"
	groupKey         = "Group"
	lastOffsetKey    = "lastOffset"
	firstOffsetKey   = "lastOffset"
)

// Callback - callback method for consumer
type Callback func(ctx context.Context, event *MessageEvent) error

// Filter - filters method for messages
type Filter func(ctx context.Context, event *MessageEvent) bool

// AckHandler - handles Ack
type AckHandler func()

// Client interface for queuing messages
type Client interface {
	// Subscribe - nonblocking subscribe that calls back handler upon message
	Subscribe(
		ctx context.Context,
		topic string,
		shared bool,
		cb Callback,
		filter Filter,
		props MessageHeaders,
	) (id string, err error)
	// UnSubscribe - unsubscribe
	UnSubscribe(
		ctx context.Context,
		topic string,
		id string,
	) (err error)
	// Send - sends one message and closes producer at the end
	Send(
		ctx context.Context,
		topic string,
		payload []byte,
		props MessageHeaders,
	) (messageID []byte, err error)
	// SendReceive - Send and receive message
	SendReceive(
		ctx context.Context,
		outTopic string,
		payload []byte,
		inTopic string,
		props MessageHeaders,
	) (event *MessageEvent, err error)
	// Publish - caches producer if it doesn't exist and sends a message
	Publish(
		ctx context.Context,
		topic string,
		payload []byte,
		props MessageHeaders,
	) (messageID []byte, err error)
	// Close - Closes all producers and consumers
	Close()
}

// NewMessagingClient creates new client for messaging
func NewMessagingClient(config *types.CommonConfig) (Client, error) {
	if config.MessagingProvider == types.RedisMessagingProvider {
		return newClientRedis(&config.Redis)
	} else if config.MessagingProvider == types.PulsarMessagingProvider {
		return newPulsarClient(&config.Pulsar)
	} else if config.MessagingProvider == types.KafkaMessagingProvider {
		return newKafkaClient(config)
	} else {
		return nil, fmt.Errorf("unsupported messaging provider %s", config.MessagingProvider)
	}
}
