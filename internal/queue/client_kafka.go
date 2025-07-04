package queue

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"plexobject.com/formicary/internal/types"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Constants for Kafka-specific configuration
const (
	defaultMaxBytes       = 1024 * 1024 // 1MB
	defaultQueueCapacity  = 100
	defaultRetentionTime  = 2 * time.Hour
	defaultCommitInterval = time.Second
	defaultDialTimeout    = 10 * time.Second
	defaultBatchTimeout   = 10 * time.Millisecond
	defaultBatchSize      = 10
	maxReconnectAttempts  = 3
)

// ClientKafka structure implements interface for queuing messages using Apache Kafka
type ClientKafka struct {
	*MetricsCollector
	config                *types.QueueConfig
	consumerProducerLock  sync.RWMutex
	producers             map[string]*kafka.Writer
	consumersByTopicGroup map[string]*kafkaSubscription
	consumersByID         map[string]*kafkaSubscription
	readerDialer          *kafka.Dialer
}

// kafkaSubscription manages a single subscription
type kafkaSubscription struct {
	ctx           context.Context
	cancel        context.CancelFunc
	topic         string
	group         string
	mx            *SendReceiveMultiplexer
	reader        *kafka.Reader
	received      int64
	closed        bool
	lastCommitted kafka.Message // Track last committed message for manual commits
	commitLock    sync.Mutex    // Protects commit operations
	retryCount    int32         // Track retries
}

// connectionCloser for cleanup
type connectionCloser func()

// newKafkaClient creates structure for implementing queuing operations
func newKafkaClient(ctx context.Context, config *types.QueueConfig, clientID string) (*ClientKafka, error) {
	if len(config.Endpoints) == 0 {
		return nil, fmt.Errorf("no kafka brokers specified")
	}

	client := &ClientKafka{
		config:                config,
		producers:             make(map[string]*kafka.Writer),
		consumersByTopicGroup: make(map[string]*kafkaSubscription),
		consumersByID:         make(map[string]*kafkaSubscription),
		MetricsCollector:      newMetricsCollector(ctx),
		readerDialer: &kafka.Dialer{
			Timeout:   defaultDialTimeout,
			DualStack: true,
			ClientID:  clientID,
		},
	}

	// Add TLS config if specified
	if config.Tls != nil && config.Tls.Enabled {
		tlsConfig, err := config.Tls.CreateTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		client.readerDialer.TLS = tlsConfig
	}

	return client, nil
}

// Subscribe implements queue.Client interface
func (c *ClientKafka) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
	// Context validation
	if ctx.Err() != nil {
		return "", fmt.Errorf("context is cancelled or expired: %w", ctx.Err())
	}
	if c.closed {
		return "", fmt.Errorf("kafka client is closed")
	}
	if err := validateSubscribeOptions(&opts); err != nil {
		return "", err
	}

	subID := ulid.Make().String()

	// GetArtifact or create consumer with subscription
	id, _, _, err := c.getOrCreateConsumer(
		ctx,
		opts.Topic,
		opts.Shared,
		opts.Callback,
		opts.Filter,
		opts.Group,
		opts.Props,
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     opts.Topic,
			"SubID":     subID,
			"Shared":    opts.Shared,
			"Error":     err,
		}).Warnf("failed to create consumer")

		return "", fmt.Errorf("failed to create consumer: %w", err)
	}

	// Setup cleanup on context cancellation
	go func() {
		<-ctx.Done()
		if closeErr := c.closeConsumer(opts.Topic, id, false); closeErr != nil {
			logrus.WithError(closeErr).WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     opts.Topic,
				"ID":        id,
				"Error":     closeErr,
			}).Error("failed to close consumer on context cancellation")
		}
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     opts.Topic,
			"SubID":     subID,
			"Shared":    opts.Shared,
		}).Debugf("subscribed successfully")
	}

	return id, nil
}

// UnSubscribe implements the queue.Client interface
func (c *ClientKafka) UnSubscribe(ctx context.Context, topic, id string) error {
	// Context validation
	if ctx.Err() != nil {
		return fmt.Errorf("context is cancelled or expired: %w", ctx.Err())
	}

	// Client state validation
	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Topic validation
	if err := validateTopic(topic); err != nil {
		return fmt.Errorf("invalid topic: %w", err)
	}

	// ID validation
	if id == "" {
		return fmt.Errorf("subscription ID cannot be empty")
	}

	c.consumerProducerLock.RLock()
	subscription, exists := c.consumersByID[id]
	c.consumerProducerLock.RUnlock()

	if !exists {
		//debug.PrintStack()
		return fmt.Errorf("subscription not found for unsubscribe with topic: %s and ID: %s", topic, id)
	}

	// Stop the consumer
	err := c.closeConsumer(topic, id, false)
	if err != nil {
		return fmt.Errorf("failed to close consumer: %w", err)
	}

	//debug.PrintStack()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"ID":        id,
			"Group":     subscription.group,
		}).Debug("unsubscribed successfully")
	}
	return nil
}

// Send implements the queue.Client interface
func (c *ClientKafka) Send(ctx context.Context, topic string,
	payload []byte, props MessageHeaders) ([]byte, error) {
	// Validate context and state
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

	// Create non-reusable producer for single send
	producer, err := c.createProducer(topic, true)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	defer func() {
		_ = producer.Close()
	}()

	// Send with retry logic
	msgID, err := c.sendWithRetry(ctx, producer, topic, payload, props)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Update metrics
	c.updateMetrics(topic, 1, 0, 0, 0, -1)

	return msgID, nil
}

// SendReceive implements the queue.Client interface
func (c *ClientKafka) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	// Validate context and state
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context is cancelled or expired: %w", ctx.Err())
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	started := time.Now()
	// Validate request
	if err := validateSendReceiveRequest(req, c.config); err != nil {
		return nil, err
	}

	// Setup correlation
	props := prepareProps(req)
	//ctx, cancel := createTimeoutContext(ctx, req.Timeout)
	//defer cancel()

	// Create temporary subscription for response
	id, subscription, consumerChannel, err := c.getOrCreateConsumer(
		ctx,
		req.InTopic,
		false,
		func(ctx context.Context, event *MessageEvent, ack AckHandler, nack AckHandler) error {
			return nil // no-op callback
		},
		nil,
		req.Group,
		props,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create response consumer: %w", err)
	}

	defer func() {
		if cleanupErr := c.closeConsumer(req.InTopic, id, false); cleanupErr != nil {
			logrus.WithError(cleanupErr).Error("Failed to cleanup consumer")
		}
	}()
	createConsumerElapsed := time.Since(started)
	started = time.Now()
	// Send the message
	if _, err := c.Publish(ctx, req.OutTopic, req.Payload, props); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	publishElapsed := time.Since(started)
	started = time.Now()

	defer func() {
		logrus.WithFields(logrus.Fields{
			"Component":       "ClientKafka",
			"OutTopic":        req.OutTopic,
			"InTopic":         req.InTopic,
			"ID":              id,
			"Group":           subscription.group,
			"ConsumerElapsed": createConsumerElapsed,
			"PublishElapsed":  publishElapsed,
			"ReceiveElapsed":  time.Since(started),
		}).Info("completed send/receive")
	}()
	// Wait for response
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event := <-consumerChannel:
			if event == nil {
				continue
			}
			// Verify correlation ID
			if event.CoRelationID() != props.GetCorrelationID() {
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component":      "ClientKafka",
						"ExpectedCorrID": props.GetCorrelationID(),
						"ReceivedCorrID": event.CoRelationID(),
						"Topic":          req.InTopic,
					}).Debug("skipping message with wrong correlation ID")
				}
				continue // Skip messages with wrong correlation ID
			}
			return c.createSendReceiveResponse(event, subscription), nil
		}
	}
}

// Publish implements the queue.Client interface
func (c *ClientKafka) Publish(ctx context.Context, topic string,
	payload []byte, props MessageHeaders) ([]byte, error) {
	// Validate context and state
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

	// GetArtifact or create reusable producer
	producer, err := c.getProducer(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get producer: %w", err)
	}

	// Send with retry logic
	msgID, err := c.sendWithRetry(ctx, producer, topic, payload, props)
	if err != nil {
		c.closeProducer(topic) // Stop producer on error
		return nil, fmt.Errorf("failed to publish message to %s: %w", topic, err)
	}

	// Update metrics
	c.updateMetrics(topic, 1, 0, 0, 0, -1)

	return msgID, nil
}

// Close implements the queue.Client interface
func (c *ClientKafka) Close() {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if c.closed {
		return
	}

	// Stop producers
	for _, producer := range c.producers {
		_ = producer.Close()
	}

	// Stop consumers
	for _, subscription := range c.consumersByTopicGroup {
		for _, id := range subscription.mx.SubscriberIDs() {
			delete(c.consumersByID, id)
		}
		_ = subscription.close()
	}

	c.closed = true
}

// ////////////////////////// PRIVATE METHODS /////////////////////////////
func (c *ClientKafka) closeConsumer(
	topic string,
	id string,
	purgeSubscription bool) (err error) {
	defer recoverNilMessage(topic, id)
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	// find by subscriber id
	subscription := c.consumersByID[id]
	if subscription == nil {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     topic,
				"ID":        id}).
				Debugf("could not close consumer due to no subscription")
		}
		return nil
	}

	// delete subscriber id and remove it from multiplexer
	delete(c.consumersByID, id)
	total := subscription.mx.Remove(id)

	if total == 0 && purgeSubscription {
		_ = subscription.close()
		delete(c.consumersByTopicGroup, topicGroupKey(topic, subscription.group))
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":         "ClientKafka",
				"SubscriptionGroup": subscription.group,
				"Topic":             topic,
				"ID":                id}).
				Debug("closing subscriber")
		}
	}
	return
}

func (c *ClientKafka) getOrCreateConsumer(
	ctx context.Context,
	topic string,
	shared bool,
	cb Callback,
	filter Filter,
	group string,
	props MessageHeaders,
) (string, *kafkaSubscription, chan *MessageEvent, error) {
	id := ulid.Make().String()
	if dashN := strings.Index(topic, "-"); dashN > 0 {
		id = topic[0:dashN] + "-" + id // for testing only
	}

	// Determine offset
	var offset int64
	if props.GetLastOffset() != "" {
		offset = kafka.LastOffset
	} else if props.GetFirstOffset() != "" {
		offset = kafka.FirstOffset
	}

	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	// Find existing subscription
	subscription := c.consumersByTopicGroup[topicGroupKey(topic, group)]

	// Create consumer channel only for SendReceive operations
	var consumerChannel chan *MessageEvent
	if props.GetCorrelationID() != "" {
		consumerChannel = make(chan *MessageEvent, getChannelBuffer(c.config))
	}

	// Use existing subscription if available
	if subscription != nil {
		c.consumersByID[id] = subscription
		if _, err := subscription.mx.Add(ctx, id, props.GetCorrelationID(), cb, filter, consumerChannel); err != nil {
			if consumerChannel != nil {
				close(consumerChannel)
			}
			return "", nil, nil, err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":         "ClientKafka",
				"SubscriptionGroup": subscription.group,
				"Topic":             topic,
				"CorrelationID":     props.GetCorrelationID(),
				"ID":                id}).
				Debug("reusing exisitng subscriber")
		}
		return id, subscription, consumerChannel, nil
	}

	// Create new subscription
	subscription, err := c.createConsumer(ctx, topic, group, id, props.GetCorrelationID(), offset,
		shared, cb, filter, consumerChannel)
	if err != nil {
		if consumerChannel != nil {
			close(consumerChannel)
		}
		return "", nil, nil, err
	}

	// Store subscription references
	c.consumersByTopicGroup[topicGroupKey(topic, group)] = subscription
	c.consumersByID[id] = subscription

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":         "ClientKafka",
			"SubscriptionGroup": subscription.group,
			"Topic":             topic,
			"CorrelationID":     props.GetCorrelationID(),
			"ID":                id}).
			Debug("created new subscriber")
	}
	return id, subscription, consumerChannel, nil
}

// createConsumer maintains original subscription creation
func (c *ClientKafka) createConsumer(
	ctx context.Context,
	topic string,
	group string,
	id string,
	correlationID string,
	offset int64,
	shared bool,
	cb Callback,
	filter Filter,
	consumerChannel chan *MessageEvent,
) (*kafkaSubscription, error) {
	if err := c.checkTopic(topic); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ConsumerGroup": group,
			"ID":            id,
			"Topic":         topic,
			"Offset":        offset,
			"Shared":        shared,
			"Error":         err,
		}).Error("Failed to validate topic for consumer")
	}

	// Create subscription with original structure
	subscription := &kafkaSubscription{
		topic: topic,
		mx: NewSendReceiveMultiplexer(
			ctx,
			topic,
			c.config.Kafka.CommitTimeout),
		group: group,
	}

	// Add subscriber (maintaining original behavior)
	if _, err := subscription.mx.Add(ctx, id, correlationID, cb, filter, consumerChannel); err != nil {
		return nil, err
	}
	// Use background context for subscription
	subscription.ctx, subscription.cancel = context.WithCancel(context.Background())

	if subscription.reader != nil {
		_ = subscription.reader.Close()
	}

	// Create reader with original configuration
	subscription.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:         c.config.Endpoints,
		GroupID:         subscription.group,
		Topic:           topic,
		StartOffset:     offset,
		MinBytes:        1,
		MaxBytes:        1024 * 1024,
		MaxWait:         500 * time.Millisecond, // affects receive time
		QueueCapacity:   100,
		RetentionTime:   2 * time.Hour,
		Dialer:          c.readerDialer,
		ReadLagInterval: -1,
		CommitInterval:  100 * time.Millisecond, // affects receive time
		// SessionTimeout:   10 * time.Second,
		RebalanceTimeout: 2 * time.Second,
		JoinGroupBackoff: 100 * time.Millisecond, // Added backoff
		MaxAttempts:      int(c.config.RetryMax), // Added max attempts
	})

	// Register message processing
	go c.receiveMessages(topic, id, subscription, func() {
		if subscription.reader != nil {
			_ = subscription.reader.Close()
		}
		subscription.reader = kafka.NewReader(subscription.reader.Config())
	})

	return subscription, nil
}

// receiveMessages maintains original functionality with improved error handling
func (c *ClientKafka) receiveMessages(
	topic string,
	id string,
	subscription *kafkaSubscription,
	initReader func(),
) {
	started := time.Now()
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     topic,
				"ID":        id,
				"Panic":     r,
			}).Error("Recovered from panic in message processing")
		}

		err := c.closeConsumer(topic, id, false)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":     "ClientKafka",
				"ConsumerGroup": subscription.group,
				"ID":            id,
				"Topic":         topic,
				"Elapsed":       time.Since(started),
				"Error":         err,
				"CtxError":      subscription.ctx.Err(),
			}).Debug("exiting subscription loop!")
		}
	}()

	for {
		if subscription.ctx.Err() != nil {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     subscription.topic,
					"ID":        id,
				}).Debug("received done signal from context")
			}
			return
		}

		// Reading messages with manual commit (maintaining original behavior)
		msg, err := subscription.reader.FetchMessage(subscription.ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.updateMetrics(topic, 0, 0, 1, 0, -1) // Record retry
				continue
			} else if strings.Contains(err.Error(), "Rebalance In Progress") {
				initReader()
				continue
			}
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Group":     subscription.group,
				"Topic":     topic,
				"ID":        id,
				"Message":   msg,
				"Elapsed":   time.Since(started),
				"Error":     err,
				"ErrorType": reflect.TypeOf(err),
			}).Warn("failed to fetch message from kafka")

			return
		}

		// Process message with original ack/nack handling
		atomic.AddInt64(&subscription.received, 1)

		ackOnce := sync.Once{}
		nackOnce := sync.Once{}

		ack := func() {
			ackOnce.Do(func() {
				c.updateMetrics(topic, 0, 1, 0, 0, -1) // Record successful consume
				if err := subscription.reader.CommitMessages(subscription.ctx, msg); err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"Topic":     topic,
						"Partition": msg.Partition,
						"Offset":    msg.Offset,
						"Elapsed":   time.Since(started),
					}).Error("Failed to commit message")
					c.updateMetrics(topic, 0, 0, 0, 1, -1) // Record failure
				}
			})
		}

		nack := func() {
			nackOnce.Do(func() {
				// Original behavior: nack is no-op as message will be redelivered
				logrus.WithFields(logrus.Fields{
					"Topic":     topic,
					"Partition": msg.Partition,
					"Offset":    msg.Offset,
				}).Debug("Message nacked")
				c.updateMetrics(topic, 0, 0, 1, 0, -1) // Record retry
			})
		}

		event := kafkaMessageToEvent(msg)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Group":     subscription.group,
				"ID":        id,
				"Received":  subscription.received,
				"Partition": msg.Partition,
				"Offset":    msg.Offset,
				"Event":     string(event.ID),
				"Elapsed":   time.Since(started),
			}).Debug("received message")
		}

		// Asynchronous notification (maintaining original behavior)
		go func(event *MessageEvent) {
			_ = subscription.mx.Notify(subscription.ctx, event, ack, nack)
		}(event)
	}
}

// fetchMessageWithRetry attempts to fetch a message with rebalance handling - not used
func (c *ClientKafka) fetchMessageWithRetry(subscription *kafkaSubscription) (kafka.Message, error) {
	retryDelay := time.Second

	for attempt := 0; attempt < int(c.config.RetryMax); attempt++ {
		msg, err := subscription.reader.FetchMessage(subscription.ctx)
		if err == nil {
			return msg, nil
		}

		if err == io.EOF || subscription.ctx.Err() != nil {
			return kafka.Message{}, err
		}

		if strings.Contains(err.Error(), "Rebalance In Progress") {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     subscription.topic,
				"Group":     subscription.group,
				"Attempt":   attempt + 1,
			}).Debug("Rebalancing in progress, retrying")

			select {
			case <-subscription.ctx.Done():
				return kafka.Message{}, subscription.ctx.Err()
			case <-time.After(retryDelay):
				retryDelay *= 2 // Exponential backoff
				continue
			}
		}

		logrus.WithError(err).WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
			"Group":     subscription.group,
		}).Error("Failed to fetch message")

		return kafka.Message{}, err
	}

	return kafka.Message{}, fmt.Errorf("max retries exceeded while fetching message")
}

// handleMessage processes a single message
func (c *ClientKafka) handleMessage(subscription *kafkaSubscription, msg kafka.Message) error {
	// Track message for manual commit
	subscription.commitLock.Lock()
	subscription.lastCommitted = msg
	subscription.commitLock.Unlock()

	// Update received count
	atomic.AddInt64(&subscription.received, 1)

	// Create event and handlers
	event := kafkaMessageToEvent(msg)

	ackOnce := sync.Once{}
	nackOnce := sync.Once{}

	ack := func() {
		ackOnce.Do(func() {
			if err := subscription.commitMessage(msg); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     subscription.topic,
					"Partition": msg.Partition,
					"Offset":    msg.Offset,
				}).Error("Failed to commit message")
			}
		})
	}

	nack := func() {
		nackOnce.Do(func() {
			// For Kafka, not committing is equivalent to nack
			// Message will be redelivered to next consumer in Group
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     subscription.topic,
				"Partition": msg.Partition,
				"Offset":    msg.Offset,
			}).Debug("Message nacked")
		})
	}

	// Process message through multiplexer
	notifyCtx, cancel := context.WithTimeout(subscription.ctx, getMessageProcessingTimeout(c.config))
	defer cancel()

	_ = subscription.mx.Notify(notifyCtx, event, ack, nack)

	// Update metrics for successful processing
	c.updateMetrics(subscription.topic, 0, 1, 0, 0, -1)
	return nil
}

// commitMessage commits a message to Kafka
func (s *kafkaSubscription) commitMessage(msg kafka.Message) error {
	s.commitLock.Lock()
	defer s.commitLock.Unlock()

	if s.closed || s.ctx.Err() != nil {
		return fmt.Errorf("subscription is closed or cancelled")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	if err := s.reader.CommitMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to commit message: %w", err)
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Topic":     s.topic,
			"Group":     s.group,
			"Partition": msg.Partition,
			"Offset":    msg.Offset,
		}).Debug("message committed successfully")
	}

	return nil
}

// getMessageProcessingTimeout returns the configured processing timeout
func getMessageProcessingTimeout(config *types.QueueConfig) time.Duration {
	if config != nil && config.OperationTimeout != nil {
		return *config.OperationTimeout
	}
	return 30 * time.Second
}

// close handles subscription cleanup
func (s *kafkaSubscription) close() error {
	s.commitLock.Lock()
	defer s.commitLock.Unlock()

	if s.closed {
		return nil
	}

	// Cancel context and close reader
	if s.cancel != nil {
		s.cancel()
	}

	if s.reader != nil {
		if err := s.reader.Close(); err != nil {
			return fmt.Errorf("failed to close reader: %w", err)
		}
	}

	if s.cancel != nil && s.ctx.Err() == nil {
		s.cancel()
	}
	// Clean up multiplexer
	if s.mx != nil {
		for _, id := range s.mx.SubscriberIDs() {
			s.mx.Remove(id)
		}
	}

	s.closed = true
	return nil
}

func (c *ClientKafka) checkTopic(topic string) error {
	c.metricsTopicLock.Lock()
	defer c.metricsTopicLock.Unlock()

	if c.validTopics[topic] {
		return nil
	}

	retryDelay := 100 * time.Millisecond
	deadline := time.Now().Add(10 * time.Second) // Increased timeout

	var lastErr error
	var partitions []kafka.Partition

	// Try multiple brokers if first one fails
	for time.Now().Before(deadline) {
		for _, broker := range c.config.Endpoints {
			conn, err := kafka.Dial("tcp", broker)
			if err != nil {
				lastErr = err
				continue
			}

			partitions, err = conn.ReadPartitions(topic)
			_ = conn.Close()

			if err != nil {
				lastErr = err
				continue
			}

			if len(partitions) > 0 {
				c.validTopics[topic] = true
				// Add extra delay to ensure topic is fully ready across all brokers
				time.Sleep(500 * time.Millisecond)

				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component":  "ClientKafka",
						"Topic":      topic,
						"Partitions": len(partitions),
						"Brokers":    c.config.Endpoints,
					}).Debug("validated topic")
				}
				return nil
			}
		}

		// If no broker has the topic yet, wait and retry
		time.Sleep(retryDelay)
		retryDelay *= 2 // Exponential backoff
		if retryDelay > time.Second {
			retryDelay = time.Second // Cap the retry delay
		}
	}

	return fmt.Errorf("timeout waiting for topic %s to be ready on any broker, last error: %v", topic, lastErr)
}

// deleteKafkaTopic creates kafka topic
func (c *ClientKafka) deleteKafkaTopic(
	topic string,
) (err error) {
	conn, closer, err := c.connect()
	if err != nil {
		return err
	}
	defer func() {
		closer()
	}()

	err = conn.DeleteTopics(topic)
	if err == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
		}).
			Infof("deleted topic")
		return err
	}
	return
}

// connect
func (c *ClientKafka) connect() (conn *kafka.Conn, closer connectionCloser, err error) {
	controller, err := c.findController()
	if err != nil {
		return nil, nil, err
	}

	conn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to controller: %w", err)
	}

	closer = func() {
		if err := conn.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close Kafka connection")
		}
	}

	return conn, closer, nil
}

// findController finds the Kafka controller broker
func (c *ClientKafka) findController() (*kafka.Broker, error) {
	// Try each broker until we find the controller
	for _, addr := range c.config.Endpoints {
		conn, err := kafka.Dial("tcp", addr)
		if err != nil {
			continue
		}

		controller, err := conn.Controller()
		_ = conn.Close()
		if err != nil {
			continue
		}

		return &controller, nil
	}

	return nil, fmt.Errorf("could not find controller in brokers: %v", c.config.Endpoints)
}

// closeProducer safely closes a producer
func (c *ClientKafka) closeProducer(topic string) {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if writer := c.producers[topic]; writer != nil {
		if err := writer.Close(); err != nil {
			logrus.WithError(err).WithField("Topic", topic).Error("Failed to close producer")
		}
		delete(c.producers, topic)
	}
}

// createProducer creates a new Kafka producer
func (c *ClientKafka) createProducer(topic string, disableBatching bool) (*kafka.Writer, error) {
	if err := c.checkTopic(topic); err != nil {
		return nil, fmt.Errorf("invalid topic: %w", err)
	}

	batchSize := defaultBatchSize
	batchTimeout := defaultBatchTimeout
	if disableBatching {
		batchSize = 1
		batchTimeout = 0
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(c.config.Endpoints...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
		BatchTimeout: batchTimeout,
		BatchSize:    batchSize,
		RequiredAcks: kafka.RequireOne,
		Async:        !disableBatching,
		MaxAttempts:  int(c.config.RetryMax),
		// CompressionCodec: nil, // You can enable compression if needed: kafka.Snappy.Codec()
		//Compression:  kafka.Snappy,
		//Logger:       logrus.New(),
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":       "ClientKafka",
			"Topic":           topic,
			"DisableBatching": disableBatching,
			"BatchSize":       batchSize,
			"BatchTimeout":    batchTimeout,
		}).Debug("created new producer")
	}

	return writer, nil
}

func (c *ClientKafka) getProducer(topic string) (*kafka.Writer, error) {
	if err := c.checkTopic(topic); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"Error":     err,
		}).
			Errorf("failed to validate topic %s for producer", topic)
	}
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	// Check existing producer
	if writer := c.producers[topic]; writer != nil {
		return writer, nil
	}

	// Create new producer
	writer, err := c.createProducer(topic, false)
	if err != nil {
		return nil, err
	}

	c.producers[topic] = writer
	return writer, nil
}

func (c *ClientKafka) createSendReceiveResponse(event *MessageEvent,
	subscription *kafkaSubscription) *SendReceiveResponse {
	// Create Kafka message from event
	kafkaMsg := kafka.Message{
		Topic:     event.Topic,
		Partition: event.Partition,
		Offset:    event.Offset,
		Headers:   make([]kafka.Header, 0, len(event.Properties)),
		Value:     event.Payload,
		Time:      event.PublishTime,
	}

	// Convert properties to Kafka headers
	for k, v := range event.Properties {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}
	return &SendReceiveResponse{
		Event: event,
		Ack: func() {
			if err := subscription.commitMessage(kafkaMsg); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     event.Topic,
					"Partition": event.Partition,
					"Offset":    event.Offset,
					"Error":     err,
				}).Error("Failed to commit message")
			}
		},
		Nack: func() {
			// For Kafka, not committing is equivalent to nack
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     event.Topic,
				"Partition": event.Partition,
				"Offset":    event.Offset,
			}).Debug("Message nacked")
		},
	}
}

func (c *ClientKafka) sendWithRetry(ctx context.Context, producer *kafka.Writer,
	topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
		msgID, err := c.sendWithProducer(ctx, producer, topic, payload, props)
		if err == nil {
			return msgID, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		time.Sleep(getRetryDelay(attempt))
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", maxReconnectAttempts, lastErr)
}

func (c *ClientKafka) sendWithProducer(ctx context.Context, producer *kafka.Writer,
	topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	// Convert headers
	headers := make([]kafka.Header, 0, len(props))
	for k, v := range props {
		headers = append(headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	// Create message
	msg := kafka.Message{
		Key:     []byte(props.GetMessageKey()),
		Value:   payload,
		Headers: headers,
		Time:    time.Now(),
	}

	// Send with context
	if err := producer.WriteMessages(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	// Generate message ID
	msgID := ulid.Make().String()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"MessageID": msgID,
			"Headers":   len(headers),
		}).Debug("sent successfully")
	}

	return []byte(msgID), nil
}

// getQueueDepth gets the current queue depth from Kafka
// getQueueDepth calculates the queue depth for the given topic in Kafka.
func (c *ClientKafka) getQueueDepth(ctx context.Context, topic string) (int64, error) {
	// Establish a connection to the Kafka broker.
	conn, err := kafka.Dial("tcp", c.config.Endpoints[0]) // Connect to the first broker.
	if err != nil {
		return 0, fmt.Errorf("failed to connect to Kafka broker: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Retrieve partitions for the given topic.
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return 0, fmt.Errorf("failed to read partitions for topic %s: %w", topic, err)
	}

	// Calculate the total depth across all partitions.
	var totalDepth int64
	for _, partition := range partitions {
		// Connect to a specific partition.
		partitionConn, err := kafka.DialPartition(ctx, "tcp", c.config.Endpoints[0], kafka.Partition{
			Topic: topic,
			ID:    partition.ID,
		})
		if err != nil {
			logrus.WithError(err).WithField("Partition", partition.ID).
				Error("Failed to connect to partition, skipping")
			continue
		}

		// GetArtifact the oldest and newest offsets for this partition.
		oldestOffset, err1 := partitionConn.ReadFirstOffset()
		newestOffset, err2 := partitionConn.ReadLastOffset()
		_ = partitionConn.Close()

		if err1 != nil {
			logrus.WithError(err1).WithField("Partition", partition.ID).
				Error("Failed to fetch oldest offset, skipping")
			continue
		} else if err2 != nil {
			logrus.WithError(err2).WithField("Partition", partition.ID).
				Error("Failed to fetch newest offset, skipping")
			continue
		}

		// Compute the queue depth for this partition.
		totalDepth += newestOffset - oldestOffset
	}

	return totalDepth, nil
}

// getOldestMessageTime gets the timestamp of the oldest message
func (c *ClientKafka) getOldestMessageTime(topic string) (*time.Time, error) {
	conn, closer, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer closer()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, err
	}

	var oldestTime *time.Time
	for _, partition := range partitions {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:     c.config.Endpoints,
			Topic:       topic,
			Partition:   partition.ID,
			StartOffset: kafka.FirstOffset,
			MinBytes:    1,
			MaxBytes:    10e6, // 10MB
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg, err := reader.ReadMessage(ctx)
		cancel()
		_ = reader.Close()

		if err != nil {
			continue
		}

		if oldestTime == nil || msg.Time.Before(*oldestTime) {
			oldestTime = &msg.Time
		}
	}

	return oldestTime, nil
}

// Consumer Management Methods

type ConsumerOptions struct {
	Topic  string
	Group  string
	Offset int64
	Shared bool
	Props  MessageHeaders
}

func (c *ClientKafka) getReaderConfig(opts ConsumerOptions) kafka.ReaderConfig {
	config := kafka.ReaderConfig{
		Brokers:         c.config.Endpoints,
		GroupID:         opts.Group,
		Topic:           opts.Topic,
		StartOffset:     opts.Offset,
		MinBytes:        1,
		MaxBytes:        defaultMaxBytes,
		MaxWait:         5 * time.Second,
		QueueCapacity:   defaultQueueCapacity,
		RetentionTime:   defaultRetentionTime,
		Dialer:          c.readerDialer,
		ReadLagInterval: -1,
		CommitInterval:  defaultCommitInterval,
		// Additional options based on configuration
	}

	if c.config.MaxFetchSize > 0 {
		config.MaxBytes = int(c.config.MaxFetchSize)
	}

	return config
}

// kafkaMessageToEvent maintains original conversion logic
func kafkaMessageToEvent(msg kafka.Message) *MessageEvent {
	event := &MessageEvent{
		Topic:       msg.Topic,
		Payload:     msg.Value,
		ID:          []byte(fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)),
		PublishTime: msg.Time,
		Properties:  make(map[string]string),
		Partition:   msg.Partition,
		Offset:      msg.Offset,
	}

	// Convert headers
	if msg.Headers != nil && len(msg.Headers) > 0 {
		for _, h := range msg.Headers {
			event.Properties[h.Key] = string(h.Value)
		}
	}

	event.ProducerName = event.Producer()
	return event
}

func topicGroupKey(topic string, group string) string {
	return topic + ":" + group
}

func (c *ClientKafka) CreateTopicIfNotExists(_ context.Context, topic string, cfg *TopicConfig) error {
	c.metricsTopicLock.Lock()
	defer c.metricsTopicLock.Unlock()

	if c.validTopics[topic] {
		return nil
	}

	conn, closer, err := c.connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer closer()

	// Check if topic exists
	partitions, err := conn.ReadPartitions(topic)
	if err == nil && len(partitions) > 0 {
		c.validTopics[topic] = true
		return nil
	}

	// Set default configuration if not provided
	if cfg == nil {
		cfg = &TopicConfig{
			NumPartitions:     1,
			ReplicationFactor: 1,
			RetentionTime:     24 * time.Hour,
		}
	}

	// Prepare topic configs
	configs := []kafka.ConfigEntry{
		{
			ConfigName:  "retention.ms",
			ConfigValue: strconv.FormatInt(cfg.RetentionTime.Milliseconds(), 10),
		},
	}

	// Add any additional configs
	for key, value := range cfg.Configs {
		configs = append(configs, kafka.ConfigEntry{
			ConfigName:  key,
			ConfigValue: value,
		})
	}

	// Create topic
	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     cfg.NumPartitions,
		ReplicationFactor: cfg.ReplicationFactor,
		ConfigEntries:     configs,
	})

	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	c.validTopics[topic] = true

	logrus.WithFields(logrus.Fields{
		"Component":         "ClientKafka",
		"Topic":             topic,
		"Partitions":        cfg.NumPartitions,
		"ReplicationFactor": cfg.ReplicationFactor,
	}).Info("created Kafka topic")

	// Wait for topic to be fully ready
	deadline := time.Now().Add(10 * time.Second)
	retryDelay := 100 * time.Millisecond
	for time.Now().Before(deadline) {
		ready := true
		for _, broker := range c.config.Endpoints {
			conn, err := kafka.Dial("tcp", broker)
			if err != nil {
				ready = false
				break
			}
			partitions, err := conn.ReadPartitions(topic)
			_ = conn.Close()
			if err != nil || len(partitions) == 0 {
				ready = false
				break
			}
		}
		if ready {
			c.validTopics[topic] = true
			time.Sleep(500 * time.Millisecond) // Extra delay for stability
			return nil
		}
		time.Sleep(retryDelay)
		retryDelay *= 2
	}

	return fmt.Errorf("timeout waiting for topic %s to be ready after creation", topic)
}
