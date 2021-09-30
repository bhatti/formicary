package queue

import (
	"context"
	"errors"
	"fmt"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"io"
	"net"
	"plexobject.com/formicary/internal/types"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// MessageKey constant
const MessageKey = "Key"

// ProducerKey constant
const ProducerKey = "Producer"

// ClientKafka structure implements interface for queuing messages using Apache Kafka
type ClientKafka struct {
	config           *types.CommonConfig
	lock             sync.Mutex
	producers        map[string]*kafka.Writer
	consumersByTopic map[string]*kafkaSubscription
	closed           bool
}

// kafkaSubscription structure
type kafkaSubscription struct {
	ctx      context.Context
	cancel   context.CancelFunc
	id       string
	topic    string
	group    string
	mx       *SendReceiveMultiplexer
	reader   *kafka.Reader
	received int64
	closed   bool
}

// newKafkaClient creates structure for implementing queuing operations
func newKafkaClient(config *types.CommonConfig) (Client, error) {
	return &ClientKafka{
		config:           config,
		producers:        make(map[string]*kafka.Writer),
		consumersByTopic: make(map[string]*kafkaSubscription),
	}, nil
}

// Subscribe - nonblocking subscribe that calls back handler upon message
func (c *ClientKafka) Subscribe(
	ctx context.Context,
	topic string,
	id string,
	props map[string]string,
	shared bool,
	cb Callback) (err error) {
	if cb == nil {
		return fmt.Errorf("callback function is not specified")
	}
	_, err = c.getOrCreateConsumer(ctx, topic, id, "", props, shared, cb)
	return
}

// UnSubscribe - unsubscribe
func (c *ClientKafka) UnSubscribe(
	_ context.Context,
	topic string,
	id string) (err error) {
	logrus.WithFields(logrus.Fields{
		"Component": "ClientKafka",
		"Topic":     topic,
		"ID":        id}).
		Infof("unsubscribed...")
	return c.closeConsumer(topic, id)
}

// Send - sends one message and closes producer at the end
func (c *ClientKafka) Send(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	_ bool) (messageID []byte, err error) {
	err = c.send(
		ctx,
		topic,
		props,
		payload)
	return
}

// SendReceive - Send and receive message
func (c *ClientKafka) SendReceive(
	ctx context.Context,
	outTopic string,
	props map[string]string,
	payload []byte,
	inTopic string,
) (event *MessageEvent, err error) {
	props[ReplyTopicKey] = inTopic
	props[CorrelationIDKey] = uuid.NewV4().String()
	id := uuid.NewV4().String()
	var subscription *kafkaSubscription
	cb := func(ctx context.Context, e *MessageEvent) error {
		event = e
		if subscription != nil {
			_ = subscription.close()
		}
		return nil
	}
	subscription, err = c.createConsumer(
		ctx,
		inTopic,
		id,
		props[CorrelationIDKey],
		0,
		props,
		false,
		cb)
	if err != nil {
		return nil, err
	}

	// send message
	if _, err := c.Publish(ctx, outTopic, props, payload, false); err != nil {
		return nil, err
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return
}

// Publish - publishes the message and caches producer if it doesn't exist
func (c *ClientKafka) Publish(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	_ bool) (messageID []byte, err error) {
	err = c.send(
		ctx,
		topic,
		props,
		payload,
	)
	return
}

// Close - closes all producers and consumers
func (c *ClientKafka) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, next := range c.producers {
		_ = next.Close()
	}
	for _, subscription := range c.consumersByTopic {
		subscription.cancel()
	}
	c.closed = true
}

//////////////////////////// PRIVATE METHODS /////////////////////////////
func (c *ClientKafka) closeConsumer(
	topic string,
	id string) (err error) {
	defer recoverNilMessage(topic, id)
	c.lock.Lock()
	defer c.lock.Unlock()
	subscription := c.consumersByTopic[topic]
	if subscription == nil {
		err = fmt.Errorf("could not find consumer for topic %s and id %s", topic, id)
		return
	}
	total := subscription.mx.Remove(id)
	if total == 0 {
		subscription.cancel()
		_ = subscription.reader.Close()
		delete(c.consumersByTopic, topic)
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Group":     subscription.group,
			"Topic":     topic,
			"Brokers":   c.config.Kafka.Brokers,
			"ID":        id}).
			Infof("closing subscriber")
	}
	return
}

func (c *ClientKafka) getOrCreateConsumer(
	ctx context.Context,
	topic string,
	id string,
	correlationID string,
	props map[string]string,
	shared bool,
	cb Callback) (subscription *kafkaSubscription, err error) {
	var offset int64
	if props["LastOffset"] != "" {
		offset = kafka.LastOffset
	} else if props["FirstOffset"] != "" {
		offset = kafka.FirstOffset
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	subscription = c.consumersByTopic[topic]

	if subscription != nil {
		subscription.mx.Add(id, correlationID, cb)
		return subscription, nil
	}
	subscription, err = c.createConsumer(
		ctx,
		topic,
		id,
		correlationID,
		offset,
		props,
		shared,
		cb)
	if err != nil {
		return nil, err
	}

	c.consumersByTopic[topic] = subscription
	return
}

func (c *ClientKafka) createConsumer(
	ctx context.Context,
	topic string,
	id string,
	correlationID string,
	offset int64,
	props map[string]string,
	shared bool,
	cb Callback) (subscription *kafkaSubscription, err error) {
	subscription = &kafkaSubscription{
		id:    id,
		topic: topic,
		mx:    NewSendReceiveMultiplexer(id, correlationID, cb),
		group: c.config.Kafka.Group,
	}
	subscription.ctx, subscription.cancel = context.WithCancel(ctx)
	if props["Group"] != "" {
		subscription.group = props["Group"]
	}
	subscription.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:         c.config.Kafka.Brokers,
		GroupID:         subscription.group,
		Topic:           topic,
		StartOffset:     offset,
		MaxBytes:        10e6,             // 10MB
		MaxWait:         30 * time.Second, // Maximum amount of time to wait for new data to come when fetching batches of messages from kafka.
		ReadLagInterval: -1,
		//QueueCapacity:   1,
		//MinBytes: 1e3, // 1KB
		//CommitInterval: time.Second,
	})

	go func() {
		defer func() {
			err = c.closeConsumer(topic, id)
			logrus.WithFields(logrus.Fields{
				"Component":     "ClientKafka",
				"ConsumerGroup": subscription.group,
				"ID":            id,
				"correlationID": correlationID,
				"Topic":         topic,
				"Offset":        offset,
				"Brokers":       c.config.Kafka.Brokers,
				"Shared":        shared,
				"Error":         err,
				"CtxError":      ctx.Err(),
			}).
				Infof("exiting subscription loop!")

		}()

		for {
			if subscription.ctx.Err() != nil {
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component": "ClientKafka",
						"Topic":     subscription.topic,
						"ID":            id,
						"correlationID": correlationID,
						"Shared":    shared,
						}).
						Debugf("received done signal from context")
				}
				return
			}
			//msg, err := reader.ReadMessage(subscription.ctx) // auto-commit
			msg, err := subscription.reader.FetchMessage(subscription.ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					continue
				}
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Group":     subscription.group,
					"Topic":     topic,
					"ID":            id,
					"correlationID": correlationID,
					"Message":   msg,
					"Shared":    shared,
					"Error":     err,
				}).Errorf("failed to receive message")
				return
			}
			atomic.AddInt64(&subscription.received, 1)
			ack := func() {
				_ = subscription.reader.CommitMessages(ctx, msg)
			}
			nack := func() {
			}
			event := kafkaMessageToEvent(msg, ack, nack)
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Group":     subscription.group,
					"ID":            id,
					"correlationID": correlationID,
					"Received":  subscription.received,
					"Partition": msg.Partition,
					"Offset":    msg.Offset,
					"Shared":    shared,
					"Event":     string(event.ID),
				}).Debugf("received")
			}
			_ = subscription.mx.Notify(subscription.ctx, event)
		}
	}()
	logrus.WithFields(logrus.Fields{
		"Component": "ClientKafka",
		"Group":     subscription.group,
		"Topic":     topic,
		"ID":            id,
		"correlationID": correlationID,
		"Offset":    offset,
		"Brokers":   c.config.Kafka.Brokers,
		"Shared":    shared,
		}).
		Infof("subscribed successfully!")
	return
}
func kafkaMessageToEvent(msg kafka.Message, ack AckHandler, nack AckHandler) (event *MessageEvent) {
	event = &MessageEvent{
		Topic:       msg.Topic,
		Payload:     msg.Value,
		ID:          []byte(fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)),
		PublishTime: msg.Time,
		Properties:  make(map[string]string),
		Partition:   msg.Partition,
		Offset:      msg.Offset,
		Ack:         ack,
		Nack:        nack,
	}
	if msg.Headers != nil && len(msg.Headers) > 0 {
		for _, h := range msg.Headers {
			event.Properties[h.Key] = string(h.Value)
		}
	}
	event.ProducerName = event.Properties[ProducerKey]
	return event
}

func (c *ClientKafka) send(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte) error {
	headers := make([]kafka.Header, len(props))
	i := 0
	for k, v := range props {
		headers[i] = kafka.Header{Key: k, Value: []byte(v)}
		i++
	}
	key := make([]byte, 0)
	if props[MessageKey] != "" {
		key = []byte(props[MessageKey])
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"Value":     string(payload),
		}).Debugf("sending message")
	}
	return c.getProducer(topic).WriteMessages(
		ctx,
		kafka.Message{
			Key:     key,
			Value:   payload,
			Headers: headers,
			Time:    time.Now(),
		})
}

// createKafkaTopic creates kafka topic
func (c *ClientKafka) createKafkaTopic(
	topic string,
	partitions int,
	replication int) (err error) {

	conn, err := kafka.Dial("tcp", c.config.Kafka.Brokers[0])
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer func() {
		_ = controllerConn.Close()
	}()

	topicConfigs := []kafka.TopicConfig{{Topic: topic, NumPartitions: partitions, ReplicationFactor: replication}}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
		}).
			Infof("created topic")
		return err
	}
	return
}

func (subscription *kafkaSubscription) close() (err error) {
	if subscription.closed {
		return
	}
	if subscription.reader != nil {
		err = subscription.reader.Close()
	}
	if subscription.ctx.Err() != nil {
		subscription.cancel()
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
			"ID":        subscription.id}).
			Debugf("closing subscription")
	}

	subscription.closed = true
	return
}

func (c *ClientKafka) getProducer(
	topic string,
) *kafka.Writer {
	c.lock.Lock()
	defer c.lock.Unlock()
	writer := c.producers[topic]
	if writer == nil {
		writer = &kafka.Writer{
			Addr:         kafka.TCP(c.config.Kafka.Brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			Compression:  kafka.Snappy,
			WriteTimeout: 1 * time.Second,
			ReadTimeout:  1 * time.Second,
			BatchTimeout: 10 * time.Millisecond,
			BatchSize:    10,
		}
		c.producers[topic] = writer
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Brokers":   c.config.Kafka.Brokers,
			"Topic":     topic,
		}).Infof("adding producer")
	}
	return writer
}
