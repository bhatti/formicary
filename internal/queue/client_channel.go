package queue

import (
	"context"
	"fmt"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
	"strconv"
	"sync"
	"time"
)

var _ Client = &ClientChannel{}

type SendReceivePayloadFunc func(ctx context.Context, req *SendReceiveRequest) ([]byte, error)

type ClientChannel struct {
	config                     *types.QueueConfig
	topicLock                  sync.RWMutex
	subscribesLock             sync.RWMutex
	topics                     map[string]*topicChannel
	closed                     bool
	metrics                    *MetricsCollector
	testSendReceivePayloadFunc SendReceivePayloadFunc
}

type topicChannel struct {
	name         string
	subscribers  map[string]*subscriber
	multiplexer  *SendReceiveMultiplexer
	redeliveryCh chan *MessageEvent // Channel for redelivery
}

type subscriber struct {
	id         string
	topic      string
	callback   Callback
	filter     Filter
	shared     bool
	group      string
	ctx        context.Context
	cancel     context.CancelFunc
	msgChan    chan *MessageEvent // Individual channel for each subscriber
	maxRetries int
	dlqTopic   string
	retries    map[string]int // Message ID -> retry count
	retryLock  sync.RWMutex
}

func newChannelClient(ctx context.Context, config *types.QueueConfig, _ string) (*ClientChannel, error) {
	return &ClientChannel{
		config:  config,
		topics:  make(map[string]*topicChannel),
		metrics: newMetricsCollector(ctx),
	}, nil
}

func (c *ClientChannel) Send(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	// Check context first before any other operation
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	}

	if err := validateSendRequest(topic, payload, props, c.config); err != nil {
		return nil, err
	}

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	if props == nil {
		props = make(MessageHeaders)
	}

	msg := &MessageEvent{
		ID:          []byte(ulid.Make().String()),
		Topic:       topic,
		Payload:     payload,
		Properties:  props,
		PublishTime: time.Now(),
	}

	t, err := c.getOrCreateTopic(ctx, topic)
	if err != nil {
		return nil, err
	}

	// Use multiplexer to notify all subscribers
	processed := false
	ack := func() {
		if !processed {
			processed = true

			// Only track published messages here
			c.metrics.updateMetrics(topic, 1, 0, 0, 0, 0)
		}
	}
	nack := func() {
		if !processed {
			processed = true
			// Queue for redelivery
			select {
			case t.redeliveryCh <- msg:
				// Track retry attempt
				c.metrics.updateMetrics(topic, 0, 0, 1, 0, 0)
			default:
				logrus.Warn("Redelivery channel full, message lost")
			}
		}
	}

	// Update metrics for published message
	c.metrics.updateMetrics(topic, 1, 0, 0, 0, 0)

	sent := t.multiplexer.Notify(ctx, msg, ack, nack)
	if sent == 0 {
		ack() // Still acknowledge if no subscribers
	}

	return msg.ID, nil
}

func (c *ClientChannel) handleRedelivery(topic *topicChannel) {
	for {
		select {
		case msg := <-topic.redeliveryCh:
			if msg == nil {
				continue
			}

			// Add small delay before redelivery
			time.Sleep(100 * time.Millisecond)

			processed := false
			ack := func() {
				if !processed {
					processed = true
					c.metrics.updateMetrics(topic.name, 0, 1, 0, 0, 0)
				}
			}
			nack := func() {
				if !processed {
					processed = true
					// Queue for another redelivery attempt
					select {
					case topic.redeliveryCh <- msg:
						c.metrics.updateMetrics(topic.name, 0, 0, 1, 0, 0)
					default:
						logrus.Warn("Redelivery channel full, message lost")
					}
				}
			}

			sent := topic.multiplexer.Notify(context.Background(), msg, ack, nack)
			if sent == 0 {
				ack()
			}
		}
	}
}

func (c *ClientChannel) Publish(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	return c.Send(ctx, topic, payload, props)
}

// SetSendReceivePayloadFunc is used for testing purpose
func (c *ClientChannel) SetSendReceivePayloadFunc(cb SendReceivePayloadFunc) {
	c.testSendReceivePayloadFunc = cb
}

func (c *ClientChannel) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
	if err := validateSubscribeOptions(&opts); err != nil {
		return "", err
	}

	subID := ulid.Make().String()

	t, err := c.getOrCreateTopic(ctx, opts.Topic)
	if err != nil {
		return "", err
	}

	// Parse maxRetries from props
	maxRetries := int(c.config.RetryMax)

	// Create retry tracking for this subscriber
	retries := make(map[string]int)
	var retryLock sync.RWMutex

	// Wrap callback to handle retries and DLQ
	// Wrap callback to handle retries, DLQ and metrics
	wrappedCallback := func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
		var err error
		processed := false

		wrappedAck := func() {
			if !processed {
				processed = true
				// Update metrics for successful consumption
				c.metrics.updateMetrics(opts.Topic, 0, 1, 0, 0, 0)
				ack()
			}
		}

		wrappedNack := func() {
			if !processed {
				processed = true
				retryLock.Lock()
				retries[string(msg.ID)]++
				retryCount := retries[string(msg.ID)]
				retryLock.Unlock()

				if retryCount >= maxRetries {
					// Forward to DLQ if configured
					if dlqTopic := opts.Props["DeadLetterQueue"]; dlqTopic != "" {
						dlqMsg := *msg
						dlqMsg.Properties = make(MessageHeaders)
						for k, v := range msg.Properties {
							dlqMsg.Properties[k] = v
						}
						dlqMsg.Properties["RetryCount"] = strconv.Itoa(retryCount)
						dlqMsg.Properties["OriginalTopic"] = msg.Topic
						dlqMsg.Properties["Error"] = "Max retries exceeded"

						if _, err := c.Send(ctx, dlqTopic, msg.Payload, dlqMsg.Properties); err != nil {
							logrus.Errorf("Failed to forward message to DLQ: %v", err)
						} else {
							retryLock.Lock()
							delete(retries, string(msg.ID))
							retryLock.Unlock()
						}
					}
					wrappedAck() // Acknowledge after DLQ forwarding
				} else {
					// Update metrics for retry
					c.metrics.updateMetrics(opts.Topic, 0, 0, 1, 0, 0)

					// Requeue for retry with delay
					go func() {
						time.Sleep(time.Second * time.Duration(retryCount))
						if _, sendErr := c.Send(ctx, msg.Topic, msg.Payload, msg.Properties); sendErr != nil {
							logrus.Errorf("Failed to requeue with delay: %v", sendErr)
						}
					}()
					nack()
				}
			}
		}

		err = opts.Callback(ctx, msg, wrappedAck, wrappedNack)
		if err != nil {
			// Update metrics for failure
			c.metrics.updateMetrics(opts.Topic, 0, 0, 0, 1, 0)
		}
		return err
	}

	// Create filter for group handling
	filter := func(ctx context.Context, msg *MessageEvent) bool {
		// Apply user-provided filter if any
		if opts.Filter != nil && !opts.Filter(ctx, msg) {
			return false
		}

		return true
	}

	// Add to multiplexer
	_, err = t.multiplexer.Add(ctx, subID,
		opts.Props.GetCorrelationID(),
		wrappedCallback,
		filter,
		nil) // No need for message channel as multiplexer handles delivery

	if err != nil {
		return "", fmt.Errorf("failed to add to multiplexer: %w", err)
	}

	// Store subscriber info for group management
	c.subscribesLock.Lock()
	t.subscribers[subID] = &subscriber{
		id:         subID,
		topic:      opts.Topic,
		shared:     opts.Shared,
		group:      opts.Group,
		ctx:        ctx,
		maxRetries: maxRetries,
		dlqTopic:   opts.Props["DeadLetterQueue"],
		retries:    retries,
		retryLock:  sync.RWMutex{},
	}

	c.subscribesLock.Unlock()

	return subID, nil
}

func (c *ClientChannel) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	if err := validateSendReceiveRequest(req, c.config); err != nil {
		return nil, err
	}
	started := time.Now()
	//ctx, cancel := context.WithCancel(ctx)
	//defer cancel()
	// For testing purpose
	if c.testSendReceivePayloadFunc != nil {
		b, err := c.testSendReceivePayloadFunc(ctx, req)
		if err != nil {
			return nil, err
		}
		return &SendReceiveResponse{
			Event: &MessageEvent{
				Topic:      req.InTopic,
				Properties: req.Props,
				Payload:    b,
			},
			Ack:  func() {},
			Nack: func() {},
		}, nil
	}

	// Create buffered response channel
	responseChan := make(chan *MessageEvent, 1)

	// Generate correlation ID
	correlationID := ulid.Make().String()

	// Subscribe to response topic first
	subOpts := SubscribeOptions{
		Topic:  req.InTopic,
		Shared: false, // Use non-shared subscription for reliability
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			logrus.Debugf("Received potential response message with correlation ID: %s (expecting: %s)",
				msg.CoRelationID(), correlationID)

			if msg.CoRelationID() == correlationID {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case responseChan <- msg:
					ack()
				default:
					nack()
				}
			}
			return nil
		},
	}

	// Subscribe before sending the request
	subID, err := c.Subscribe(ctx, subOpts)
	if err != nil {
		close(responseChan)
		return nil, fmt.Errorf("failed to create response subscription: %w", err)
	}

	// Ensure cleanup
	defer func() {
		_ = c.UnSubscribe(ctx, req.InTopic, subID)
		close(responseChan)
	}()

	// Prepare request properties
	props := make(MessageHeaders)
	if req.Props != nil {
		for k, v := range req.Props {
			props[k] = v
		}
	}
	props.SetCorrelationID(correlationID)
	props.SetReplyTopic(req.InTopic)

	// Send request
	msgID, err := c.Send(ctx, req.OutTopic, req.Payload, props)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	logrus.Debugf("Sent request message ID: %s with correlation ID: %s", string(msgID), correlationID)

	// Wait for response with timeout
	timeoutCtx, cancel := createTimeoutContext(ctx, req.Timeout)
	if false {
		defer cancel()
	}

	select {
	case msg := <-responseChan:
		if msg == nil {
			return nil, fmt.Errorf("received nil response")
		}
		return &SendReceiveResponse{
			Event: msg,
			Ack:   func() { logrus.Debugf("Response acknowledged for correlation ID: %s", correlationID) },
			Nack:  func() { logrus.Debugf("Response not acknowledged for correlation ID: %s", correlationID) },
		}, nil

	case <-timeoutCtx.Done():
		//debug.PrintStack()
		elapsed := time.Since(started)
		return nil, fmt.Errorf("timeout waiting for response (elapsed: %s, configured timeout: %s)", elapsed, req.Timeout)
	}
}

func (c *ClientChannel) CreateTopicIfNotExists(ctx context.Context, topic string, _ *TopicConfig) error {
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	_, err := c.getOrCreateTopic(ctx, topic)
	return err
}

func (c *ClientChannel) UnSubscribe(ctx context.Context, topic string, id string) error {
	t, err := c.getOrCreateTopic(ctx, topic)
	if err != nil {
		return err
	}
	c.subscribesLock.Lock()
	defer c.subscribesLock.Unlock()

	sub, exists := t.subscribers[id]
	if !exists {
		return fmt.Errorf("subscription not found: %s", id)
	}

	// Cancel the subscription context
	if sub.cancel != nil {
		sub.cancel()
	}

	// Remove subscriber
	delete(t.subscribers, id)

	// If no more subscribers, cleanup topic
	if len(t.subscribers) == 0 {
		c.topicLock.Lock()
		delete(c.topics, topic)
		c.topicLock.Unlock()
		c.metrics.setTopic(topic, false) // Mark topic as invalid
	}

	return nil
}

func (c *ClientChannel) Close() {
	c.subscribesLock.Lock()
	defer c.subscribesLock.Unlock()

	if c.closed {
		return
	}

	// Stop all topic channels and cancel subscriber contexts
	for topic, t := range c.topics {
		close(t.redeliveryCh) // Stop redelivery channel
		for _, sub := range t.subscribers {
			if sub.cancel != nil {
				sub.cancel()
			}
		}
		c.metrics.setTopic(topic, false) // Mark all topics as invalid
	}

	c.closed = true
}

// GetMetrics delegates to MetricsCollector
func (c *ClientChannel) GetMetrics(ctx context.Context, topic string) (*QueueMetrics, error) {
	return c.metrics.GetMetrics(ctx, topic)
}

// Helper functions

func (c *ClientChannel) getOrCreateTopic(ctx context.Context, topic string) (*topicChannel, error) {
	c.topicLock.Lock()
	defer c.topicLock.Unlock()

	if t, exists := c.topics[topic]; exists {
		return t, nil
	}

	t := &topicChannel{
		name:         topic,
		subscribers:  make(map[string]*subscriber),
		multiplexer:  NewSendReceiveMultiplexer(ctx, topic, 30*time.Second),
		redeliveryCh: make(chan *MessageEvent, c.config.MaxConnections),
	}
	c.topics[topic] = t
	c.metrics.setTopic(topic, true)

	// Register redelivery handler
	go c.handleRedelivery(t)

	return t, nil
}
