package queue

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/types"
	"sync"
	"time"
)

// Callback processes received messages with ack/nack handlers
type Callback func(ctx context.Context, event *MessageEvent, ack AckHandler, nack AckHandler) error

// Filter - filters method for messages
type Filter func(ctx context.Context, event *MessageEvent) bool

// AckHandler - handles Ack
type AckHandler func()

// SubscribeOptions configures message subscription
type SubscribeOptions struct {
	Topic    string         // Topic to subscribe to
	Shared   bool           // Whether subscription is shared
	Callback Callback       // Message handler
	Filter   Filter         // Optional message filter
	Group    string         // Group name
	Props    MessageHeaders // Additional properties
}

// TopicConfig represents Kafka topic configuration
type TopicConfig struct {
	NumPartitions     int
	ReplicationFactor int
	RetentionTime     time.Duration
	Configs           map[string]string
}

// SendReceiveRequest represents a request-response operation
type SendReceiveRequest struct {
	OutTopic string         // Topic to publish to
	InTopic  string         // Topic to receive response from
	Payload  []byte         // Message payload
	Group    string         // Group name
	Props    MessageHeaders // Message properties
	Timeout  time.Duration  // Operation timeout
}

// SendReceiveResponse represents the response from SendReceive
type SendReceiveResponse struct {
	Event *MessageEvent // Received message
	Ack   AckHandler    // Acknowledge handler
	Nack  AckHandler    // Negative acknowledge handler
}

// Client interface for queuing messages
type Client interface {
	// Subscribe starts non-blocking consuming messages with callback
	Subscribe(ctx context.Context, opts SubscribeOptions) (id string, err error)
	// UnSubscribe stops message consumption
	UnSubscribe(ctx context.Context, topic string, id string) (err error)

	// Send publishes a message without reusing producer
	Send(ctx context.Context, topic string, payload []byte, props MessageHeaders) (messageID []byte, err error)
	// SendReceive publishes a message and waits for response
	SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error)

	// Publish publishes a message with reusable producer
	Publish(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error)

	// Close - Closes all producers and consumers
	Close()

	// GetMetrics returns queue statistics
	GetMetrics(ctx context.Context, topic string) (*QueueMetrics, error)

	CreateTopicIfNotExists(ctx context.Context, topic string, cfg *TopicConfig) error
}

func CreateClient(ctx context.Context, config *types.CommonConfig) (Client, error) {
	switch config.Queue.Provider {
	case types.KafkaMessagingProvider:
		return newKafkaClient(ctx, config.Queue, config.ID)
	case types.PulsarMessagingProvider:
		return newPulsarClient(ctx, config.Queue, config.ID)
	case types.RedisMessagingProvider:
		return newRedisClient(ctx, config, config.ID)
	case types.ChannelMessagingProvider:
		return newChannelClient(ctx, config.Queue, config.ID)
	default:
		if config.Queue.Provider == "" {
			return newChannelClient(ctx, config.Queue, config.ID)
		}
		return nil, fmt.Errorf("unsupported provider: %v", config.Queue.Provider)
	}
}

// ClientManager manages queue clients
type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]Client
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]Client),
	}
}

// GetClient returns a client for given config, creating if necessary
func (m *ClientManager) GetClient(ctx context.Context, config *types.CommonConfig) (Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%v-%v", config.Queue.Provider, config.Queue.Endpoints)
	if client, exists := m.clients[key]; exists {
		return client, nil
	}

	client, err := CreateClient(ctx, config)
	if err != nil {
		return nil, err
	}

	m.clients[key] = client
	return client, nil
}
