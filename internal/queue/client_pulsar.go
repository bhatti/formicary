package queue

// See https://pulsar.apache.org/docs/en/reference-configuration/

import (
	"context"
	"fmt"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
	"strings"
	"sync"
	"time"
)

// ClientPulsar implements the queue.Client interface using Apache Pulsar
type ClientPulsar struct {
	*MetricsCollector
	config               *types.QueueConfig
	client               pulsar.Client
	consumerProducerLock sync.Mutex
	producers            map[string]pulsar.Producer
	consumers            map[string]*pulsarSubscription
}

type pulsarSubscription struct {
	topic    string
	consumer pulsar.Consumer
	callback Callback
	filter   Filter
	ctx      context.Context
	cancel   context.CancelFunc
}

func newPulsarClient(ctx context.Context, config *types.QueueConfig, _ string) (*ClientPulsar, error) {
	opts := pulsar.ClientOptions{
		URL: config.Endpoints[0],
		//Logger:            logrus.StandardLogger(),
	}
	if config.ConnectionTimeout != nil {
		opts.OperationTimeout = *config.ConnectionTimeout
		opts.ConnectionTimeout = *config.ConnectionTimeout
	}

	// Add authentication if configured
	if config.Token != "" {
		opts.Authentication = pulsar.NewAuthenticationToken(config.Token)
	}

	client, err := pulsar.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create pulsar client: %w", err)
	}

	return &ClientPulsar{
		config:           config,
		client:           client,
		producers:        make(map[string]pulsar.Producer),
		consumers:        make(map[string]*pulsarSubscription),
		MetricsCollector: newMetricsCollector(ctx),
	}, nil
}

func (c *ClientPulsar) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
	if ctx.Err() != nil {
		return "", fmt.Errorf("context cancelled: %w", ctx.Err())
	}
	if c.closed {
		return "", fmt.Errorf("client is closed")
	}
	if err := validateSubscribeOptions(&opts); err != nil {
		return "", err
	}

	subID := ulid.Make().String()

	// Create subscription
	subscription, err := c.createSubscription(ctx, opts.Topic, subID, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create subscription: %w", err)
	}

	c.consumerProducerLock.Lock()
	c.consumers[subID] = subscription
	c.consumerProducerLock.Unlock()
	c.setTopic(opts.Topic, true)

	// Register message processing
	go c.processMessages(ctx, opts.Topic, subscription)

	return subID, nil
}

func (c *ClientPulsar) UnSubscribe(_ context.Context, _ string, id string) error {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	subscription, exists := c.consumers[id]
	if !exists {
		return fmt.Errorf("subscription not found: %s", id)
	}

	subscription.cancel()
	if err := subscription.consumer.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}
	subscription.consumer.Close()
	delete(c.consumers, id)

	return nil
}

// Send implements queue.Client interface
func (c *ClientPulsar) Send(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context is cancelled or expired: %w", ctx.Err())
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Validate input
	if err := validateSendRequest(topic, payload, props, c.config); err != nil {
		return nil, err
	}

	// Create non-reusable producer
	opts := pulsar.ProducerOptions{
		Topic:              topic,
		DisableBatching:    true,
		MaxPendingMessages: 1,
	}
	if c.config.OperationTimeout != nil {
		opts.SendTimeout = *c.config.OperationTimeout
	}
	producer, err := c.client.CreateProducer(opts)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	defer producer.Close()

	// Send message with retry
	msgID, err := c.sendMessage(ctx, producer, payload, props)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Update metrics
	c.updateMetrics(topic, 1, 0, 0, 0, -1)

	return msgID, nil
}

func (c *ClientPulsar) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Validate request
	if err := validateSendReceiveRequest(req, c.config); err != nil {
		return nil, err
	}

	// Setup correlation
	props := prepareProps(req)
	consumerCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Create consumer for response
	consumer, err := c.client.Subscribe(pulsar.ConsumerOptions{
		Topic:            req.InTopic,
		SubscriptionName: ulid.Make().String(),
		Type:             pulsar.Exclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create response consumer: %w", err)
	}
	defer consumer.Close()

	// Send request
	_, err = c.Publish(ctx, req.OutTopic, req.Payload, props)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	msg, err := consumer.Receive(consumerCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	event := pulsarMessageToEvent(msg)
	ackOnce := sync.Once{}
	nackOnce := sync.Once{}

	ack := func() {
		ackOnce.Do(func() {
			_ = consumer.Ack(msg)
		})
	}
	nack := func() {
		nackOnce.Do(func() {
			consumer.Nack(msg)
		})
	}
	return &SendReceiveResponse{
		Event: event,
		Ack:   ack,
		Nack:  nack,
	}, nil
}

func (c *ClientPulsar) Publish(ctx context.Context, topic string,
	payload []byte, props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Validate input
	if err := validateSendRequest(topic, payload, props, c.config); err != nil {
		return nil, err
	}

	producer, err := c.getOrCreateProducer(topic, props)
	if err != nil {
		return nil, err
	}

	return c.sendMessage(ctx, producer, payload, props)
}

func (c *ClientPulsar) Close() {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if c.closed {
		return
	}

	for _, producer := range c.producers {
		producer.Close()
	}

	for _, subscription := range c.consumers {
		subscription.cancel()
		subscription.consumer.Close()
	}

	c.client.Close()
	c.closed = true
}

func (c *ClientPulsar) CreateTopicIfNotExists(_ context.Context, topic string, _ *TopicConfig) error {
	return validateTopic(topic)
}

// Helper functions...
func (c *ClientPulsar) processMessages(ctx context.Context, topic string, sub *pulsarSubscription) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithError(fmt.Errorf("%v", r)).Error("recovered from panic in message processing")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := sub.consumer.Receive(ctx)
			if err != nil {
				if ctx.Err() == nil && !strings.Contains(err.Error(), "ConsumerClosed") {
					c.updateMetrics(topic, 0, 0, 1, 0, -1) // Record retry
					logrus.WithError(err).Error("failed to receive message")
				} else {
					return
				}
				continue
			}

			event := pulsarMessageToEvent(msg)
			if sub.filter == nil || sub.filter(ctx, event) {
				ackOnce := sync.Once{}
				nackOnce := sync.Once{}

				ack := func() {
					ackOnce.Do(func() {
						_ = sub.consumer.Ack(msg)
						c.updateMetrics(topic, 0, 1, 0, 0, -1) // Record successful consume
					})
				}
				nack := func() {
					nackOnce.Do(func() {
						sub.consumer.Nack(msg)
						c.updateMetrics(topic, 0, 0, 1, 0, -1) // Record retry
					})
				}
				if err := sub.callback(ctx, event, ack, nack); err != nil {
					c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
					logrus.WithError(err).Error("failed to process message")
				}
			}
		}
	}
}

func pulsarMessageToEvent(msg pulsar.Message) *MessageEvent {
	event := &MessageEvent{
		Topic:       msg.Topic(),
		Payload:     msg.Payload(),
		ID:          msg.ID().Serialize(),
		PublishTime: msg.PublishTime(),
		Properties:  msg.Properties(),
	}

	// Add standard properties
	if msg.Key() != "" {
		event.Properties.SetMessageKey(msg.Key())
	}
	event.Properties.SetProducer(msg.ProducerName())
	if msg.EventTime().Unix() > 0 {
		event.Properties["EventTime"] = msg.EventTime().Format(time.RFC3339)
	}

	return event
}

// Additional helper methods...
// Producer management
func (c *ClientPulsar) getOrCreateProducer(topic string, props MessageHeaders) (pulsar.Producer, error) {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if producer := c.producers[topic]; producer != nil {
		return producer, nil
	}

	producer, err := c.createProducer(topic, props)
	if err != nil {
		return nil, err
	}

	c.producers[topic] = producer
	c.setTopic(topic, true)
	return producer, nil
}

func (c *ClientPulsar) createProducer(topic string, props MessageHeaders) (pulsar.Producer, error) {
	opts := pulsar.ProducerOptions{
		Topic:                   topic,
		DisableBatching:         props.IsDisableBatching(),
		CompressionType:         pulsar.LZ4,
		MaxPendingMessages:      int(c.config.MaxConnections),
		BatchingMaxPublishDelay: 10 * time.Millisecond,
		BatchingMaxMessages:     1000,
		Properties: map[string]string{
			"producer_name": ulid.Make().String(),
		},
	}
	if c.config.OperationTimeout != nil {
		opts.SendTimeout = *c.config.OperationTimeout
	}

	// Handle routing mode if specified
	if key := props.GetMessageKey(); key != "" {
		opts.MessageRouter = func(msg *pulsar.ProducerMessage, tm pulsar.TopicMetadata) int {
			// Use consistent hashing for message routing
			hash := hashString(key)
			return int(hash % tm.NumPartitions())
		}
	}

	producer, err := c.client.CreateProducer(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
		}).Debug("created producer")
	}

	return producer, nil
}

// Subscription management
func (c *ClientPulsar) createSubscription(ctx context.Context,
	topic, id string, opts SubscribeOptions) (*pulsarSubscription, error) {
	subCtx, cancel := context.WithCancel(context.Background())

	// Configure subscription type based on shared flag
	subType := pulsar.Shared
	if !opts.Shared {
		subType = pulsar.Exclusive
	}

	// Determine subscription name
	subName := opts.Group // fmt.Sprintf("sub-%s", id)

	consumerOpts := pulsar.ConsumerOptions{
		Topic:                       topic,
		SubscriptionName:            subName,
		Type:                        subType,
		SubscriptionInitialPosition: pulsar.SubscriptionPositionLatest,
		MessageChannel:              make(chan pulsar.ConsumerMessage, c.config.MaxConnections),
		ReceiverQueueSize:           int(c.config.MaxConnections),
		NackRedeliveryDelay:         1 * time.Second,
		RetryEnable:                 true,
		Name:                        fmt.Sprintf("consumer-%s", id),
		// Configure dead letter policy
		DLQ: &pulsar.DLQPolicy{
			MaxDeliveries:    4, // Match with retry times
			RetryLetterTopic: fmt.Sprintf("%s-%s-RETRY", topic, subName),
			DeadLetterTopic:  fmt.Sprintf("%s-%s-DLQ", topic, subName),
		},
		// Configure backoff policy
		NackBackoffPolicy: c,
		Properties: map[string]string{
			"consumer_name": fmt.Sprintf("consumer-%s", id),
		},
	}

	consumer, err := c.client.Subscribe(consumerOpts)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	subscription := &pulsarSubscription{
		topic:    topic,
		consumer: consumer,
		callback: opts.Callback,
		filter:   opts.Filter,
		ctx:      subCtx,
		cancel:   cancel,
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
			"ID":        id,
			"Shared":    opts.Shared,
		}).Debug("created subscription")
	}

	return subscription, nil
}

func (c *ClientPulsar) Next(retryCount uint32) time.Duration {
	// Configure retry delays
	defaultRetryTimes := []time.Duration{
		1 * time.Second, // First retry after 1s
		2 * time.Second, // Second retry after 2s
		3 * time.Second, // Third retry after 3s
		5 * time.Second, // Final retry after 5s
	}

	if int(retryCount) < len(defaultRetryTimes) {
		return defaultRetryTimes[retryCount]
	}
	return defaultRetryTimes[len(defaultRetryTimes)-1]
}

// Message handling
func (c *ClientPulsar) sendMessage(ctx context.Context, producer pulsar.Producer,
	payload []byte, props MessageHeaders) ([]byte, error) {
	msg := &pulsar.ProducerMessage{
		Payload:    payload,
		Properties: props,
		EventTime:  time.Now(),
	}

	// Set key if specified
	if key := props.GetMessageKey(); key != "" {
		msg.Key = key
	}

	// Set delay if specified
	//if delay := props.GetDelay(); delay != "" {
	//	if d, err := time.ParseDuration(delay); err == nil {
	//		msg.DeliverAfter = d
	//	}
	//}

	// Send with retry
	var id pulsar.MessageID
	var err error
	for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
		id, err = producer.Send(ctx, msg)
		if err == nil {
			break
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientPulsar",
				"Topic":     producer.Topic(),
				"Attempt":   attempt + 1,
				"Error":     err,
			}).Debug("retrying message send")
		}

		time.Sleep(getRetryDelay(attempt))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send message after %d attempts: %w", maxReconnectAttempts, err)
	}

	// Convert message ID to bytes
	msgID := id.Serialize()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     producer.Topic(),
			"MessageID": fmt.Sprintf("%x", msgID),
		}).Debug("message sent successfully")
	}

	return msgID, nil
}

// Utility methods
func hashString(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// Cleanup helpers
func (c *ClientPulsar) closeProducer(topic string) {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if producer := c.producers[topic]; producer != nil {
		producer.Close()
		delete(c.producers, topic)
	}
}

func (s *pulsarSubscription) close() error {
	s.cancel()
	err := s.consumer.Unsubscribe()
	s.consumer.Close()
	return err
}

// Additional helper method for error handling
func (c *ClientPulsar) handleError(topic string, err error, isProducerError bool) {
	if isProducerError {
		c.updateMetrics(topic, 0, 0, 0, 1, -1)
	} else {
		c.updateMetrics(topic, 0, 0, 1, 0, -1)
	}

	logrus.WithFields(logrus.Fields{
		"Component":  "ClientPulsar",
		"Topic":      topic,
		"IsProducer": isProducerError,
		"Error":      err,
	}).Error("pulsar operation failed")
}
