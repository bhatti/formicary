package queue

import (
	"context"
	"fmt"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/types"
	"sync"
	"time"
)

// ReplyTopicKey reply topic key
const ReplyTopicKey = "ReplyTopic"

// See https://pulsar.apache.org/docs/en/reference-configuration/

// ClientPulsar structure implements interface for queuing messages using Apache Pulsar
type ClientPulsar struct {
	config    *types.PulsarConfig
	client    pulsar.Client
	lock      sync.Mutex
	producers map[string]pulsar.Producer
	consumers map[string]*pulsarSubscription
	closed    bool
}

// pulsarSubscription structure
type pulsarSubscription struct {
	consumer        pulsar.Consumer
	done            chan bool
	consumerChannel chan pulsar.ConsumerMessage
}

// newPulsarClient creates structure for implementing queuing operations
func newPulsarClient(config *types.PulsarConfig) (Client, error) {
	opts := pulsar.ClientOptions{
		URL:               config.URL,
		OperationTimeout:  config.ConnectionTimeout * time.Second,
		ConnectionTimeout: config.ConnectionTimeout * time.Second,
	}
	if len(config.OAuth2) > 0 {
		opts.Authentication = config.OAuth2
	}
	client, err := pulsar.NewClient(opts)
	if err != nil {
		return nil, err
	}
	return &ClientPulsar{
		config:    config,
		client:    client,
		producers: make(map[string]pulsar.Producer),
		consumers: make(map[string]*pulsarSubscription),
	}, nil
}

// Subscribe - nonblocking subscribe that calls back handler upon message
func (c *ClientPulsar) Subscribe(
	ctx context.Context,
	topic string,
	id string,
	_ map[string]string,
	shared bool,
	cb Callback) (err error) {
	if cb == nil {
		return fmt.Errorf("callback function is not specified")
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
			"ID":        id}).
			Debug("Creating goroutine to receive messages")
	}
	go func() {
		var subscription *pulsarSubscription
		for {
			subscription, err = c.getConsumer(topic, id, shared)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientPulsar",
					"Topic":     topic,
					"Error":     err,
					"ID":        id}).
					Warn("failed to get consumer for subscription, will try again")
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}

		for {
			if c.doReceive(ctx, topic, id, subscription, cb) {
				return
			}
		}
	}()
	return nil
}

// UnSubscribe - unsubscribe
func (c *ClientPulsar) UnSubscribe(
	_ context.Context,
	topic string,
	id string) (err error) {
	return c.closeConsumer(topic, id)
}

// Send - sends one message and closes producer at the end
func (c *ClientPulsar) Send(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	reusableTopic bool) (messageID []byte, err error) {
	if reusableTopic {
		return c.sendPulsarMessageWithReusableProducer(
			ctx,
			topic,
			props,
			payload,
			true)
	}
	return c.sendPulsarMessageWithoutReusableProducer(
		ctx,
		topic,
		props,
		payload)
}

// SendReceive - Send and receive message
func (c *ClientPulsar) SendReceive(
	ctx context.Context,
	outTopic string,
	props map[string]string,
	payload []byte,
	inTopic string,
) (event *MessageEvent, err error) {
	props[ReplyTopicKey] = inTopic
	props[CorrelationIDKey] = uuid.NewV4().String()

	// subscribe first
	var consumer pulsar.Consumer
	// sendPulsarMessage-receive consumer will retry for a limit time
	opts := pulsar.ConsumerOptions{
		Topic:                inTopic,
		SubscriptionName:     uuid.NewV4().String(),
		Type:                 pulsar.Exclusive,
		ReceiverQueueSize:    1,
		MaxReconnectToBroker: &c.config.MaxReconnectToBroker,
	}
	consumer, err = c.client.Subscribe(opts)
	if err != nil {
		return nil, err
	}

	// sendPulsarMessage message
	if _, err := c.Publish(ctx, outTopic, props, payload, false); err != nil {
		return nil, err
	}

	// receive synchronously
	msg, err := consumer.Receive(ctx)
	if err != nil {
		consumer.Close()
		return nil, err
	}

	event = &MessageEvent{
		Topic:        msg.Topic(),
		ProducerName: msg.ProducerName(),
		Properties:   msg.Properties(),
		Payload:      msg.Payload(),
		ID:           msg.ID().Serialize(),
		PublishTime:  msg.PublishTime(),
		Ack: func() {
			defer consumer.Close()
			consumer.Ack(msg)
		},
		Nack: func() {
			defer consumer.Close()
			consumer.Nack(msg)
		},
	}

	return
}

// Publish - publishes the message and caches producer if doesn't exist
func (c *ClientPulsar) Publish(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	disableBatching bool) (messageID []byte, err error) {
	return c.sendPulsarMessageWithReusableProducer(
		ctx,
		topic,
		props,
		payload,
		disableBatching)
}

// Close - closes all producers and consumers
func (c *ClientPulsar) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, next := range c.producers {
		next.Close()
	}
	for _, next := range c.consumers {
		_ = next.consumer.Unsubscribe()
		next.consumer.Close()
		close(next.consumerChannel)
		close(next.done)
	}
	c.client.Close()
	c.closed = true
}

//////////////////////////// PRIVATE METHODS /////////////////////////////
func (c *ClientPulsar) closeConsumer(
	topic string,
	id string) (err error) {
	defer recoverNilMessage(topic, id)
	c.lock.Lock()
	defer c.lock.Unlock()
	key := buildKey(topic, id)
	subscription := c.consumers[key]
	if subscription == nil {
		err = fmt.Errorf("could not find consumer for topic %s and id %s", topic, id)
		return
	}

	err = subscription.consumer.Unsubscribe()
	subscription.consumer.Close()
	close(subscription.consumerChannel)
	close(subscription.done)
	delete(c.consumers, key)
	return
}

func (c *ClientPulsar) getConsumer(
	topic string,
	id string,
	shared bool) (subscription *pulsarSubscription, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	key := buildKey(topic, id)
	subscription = c.consumers[key]

	if subscription != nil {
		return subscription, nil
	}

	var subscriptionType pulsar.SubscriptionType
	if shared {
		subscriptionType = pulsar.Shared
	} else {
		subscriptionType = pulsar.Failover
	}
	consumerChannel := make(chan pulsar.ConsumerMessage, c.config.ChannelBuffer)
	var consumer pulsar.Consumer
	// default consumer will retry indefinitely
	opts := pulsar.ConsumerOptions{
		Topic:             topic,
		SubscriptionName:  id,
		Type:              subscriptionType,
		MessageChannel:    consumerChannel,
		ReceiverQueueSize: c.config.ChannelBuffer,
	}
	if opts.Type == pulsar.Failover {
		opts.RetryEnable = false
	}

	// subscribe
	consumer, err = c.client.Subscribe(opts)
	if err != nil {
		return
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Type":      opts.Type,
			"Topic":     topic,
			"ID":        id}).
			Debug("subscribed successfully!")
	}

	subscription = &pulsarSubscription{
		consumer:        consumer,
		consumerChannel: consumerChannel,
		done:            make(chan bool),
	}

	c.consumers[key] = subscription
	return
}

func (c *ClientPulsar) getProducer(
	topic string,
	disableBatching bool) (producer pulsar.Producer, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	producer = c.producers[topic]
	if producer == nil {
		producer, err = c.createProducer(topic, disableBatching)
		if err != nil {
			return nil, err
		}
		c.producers[topic] = producer
	}
	return
}

func (c *ClientPulsar) createProducer(
	topic string,
	disableBatching bool) (producer pulsar.Producer, err error) {
	opts := pulsar.ProducerOptions{
		Topic:                topic,
		DisableBatching:      disableBatching,
		MaxPendingMessages:   c.config.ChannelBuffer,
		MaxReconnectToBroker: &c.config.MaxReconnectToBroker,
	}
	if disableBatching {
		opts.MaxPendingMessages = 1
	}
	return c.client.CreateProducer(opts)
}

func (c *ClientPulsar) doReceive(
	ctx context.Context,
	topic string,
	id string,
	subscription *pulsarSubscription,
	cb Callback) bool {
	defer recoverNilMessage(topic, id)
	select {
	case <-ctx.Done():
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientPulsar",
				"Topic":     topic,
				"ID":        id}).
				Debug("received done signal from context")
		}
		_ = c.closeConsumer(topic, id)
		return true
	case <-subscription.done:
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
			"ID":        id}).
			Warn("received done signal from channel")
		_ = c.closeConsumer(topic, id)
		return true
	case msg := <-subscription.consumer.Chan():
		event := MessageEvent{
			Topic:        msg.Topic(),
			ProducerName: msg.ProducerName(),
			Properties:   msg.Properties(),
			Payload:      msg.Payload(),
			ID:           msg.ID().Serialize(),
			PublishTime:  msg.PublishTime(),
			Ack: func() {
				subscription.consumer.Ack(msg)
			},
			Nack: func() {
				subscription.consumer.Nack(msg)
			},
		}
		if err := cb(ctx, &event); err != nil {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientPulsar",
					"Topic":     topic,
					"ID":        id,
					"Message":   string(msg.Payload())}).
					Debug("failed to handle message")
			}
		}
	}
	return false
}

func recoverNilMessage(topic string, id string) {
	if r := recover(); r != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
			"ID":        id,
			"Recover":   r,
		}).Error("recovering from panic")
	}
}

func sendPulsarMessage(
	ctx context.Context,
	producer pulsar.Producer,
	props map[string]string,
	payload []byte) (messageID []byte, err error) {
	var resp pulsar.MessageID
	resp, err = producer.Send(ctx, &pulsar.ProducerMessage{
		Payload:    payload,
		Properties: props,
	})
	if err != nil {
		return
	}
	return resp.Serialize(), nil
}

func (c *ClientPulsar) sendPulsarMessageWithReusableProducer(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	disableBatching bool) (b []byte, err error) {
	var producer pulsar.Producer
	producer, err = c.getProducer(topic, disableBatching)
	if err != nil {
		return
	}
	return sendPulsarMessage(ctx, producer, props, payload)
}

func (c *ClientPulsar) sendPulsarMessageWithoutReusableProducer(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte) (b []byte, err error) {
	var producer pulsar.Producer
	producer, err = c.createProducer(topic, true)
	if err != nil {
		return
	}
	defer producer.Close()
	return sendPulsarMessage(ctx, producer, props, payload)
}

func buildKey(topic string, id string) string {
	return topic + "::" + id
}
