package queue

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	kafka "github.com/segmentio/kafka-go"
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

// ClientKafka structure implements interface for queuing messages using Apache Kafka
type ClientKafka struct {
	config                *types.CommonConfig
	topicLock             sync.Mutex
	validTopics           map[string]bool
	consumerProducerLock  sync.Mutex
	producers             map[string]*kafka.Writer
	consumersByTopicGroup map[string]*kafkaSubscription
	consumersByID         map[string]*kafkaSubscription
	closed                bool
	readerDialer          *kafka.Dialer
}

// kafkaSubscription structure
type kafkaSubscription struct {
	ctx      context.Context
	cancel   context.CancelFunc
	topic    string
	group    string
	mx       *SendReceiveMultiplexer
	reader   *kafka.Reader
	received int64
	closed   bool
}

// connectionCloser for cleanup
type connectionCloser func()

// newKafkaClient creates structure for implementing queuing operations
func newKafkaClient(config *types.CommonConfig) (Client, error) {
	return &ClientKafka{
		config:                config,
		validTopics:           make(map[string]bool),
		producers:             make(map[string]*kafka.Writer),
		consumersByTopicGroup: make(map[string]*kafkaSubscription),
		consumersByID:         make(map[string]*kafkaSubscription),
		readerDialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			ClientID:  "reader-" + config.ID,
		},
	}, nil
}

// Subscribe - nonblocking subscribe that calls back handler upon message
func (c *ClientKafka) Subscribe(
	ctx context.Context,
	topic string,
	shared bool,
	cb Callback,
	filter Filter,
	props MessageHeaders,
) (id string, err error) {
	if c.closed {
		return "", fmt.Errorf("kafka client is closed")
	}
	if cb == nil {
		return "", fmt.Errorf("callback function is not specified")
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	id, _, _, err = c.getOrCreateConsumer(
		ctx,
		topic,
		shared,
		cb,
		filter,
		props,
	)
	return
}

// UnSubscribe - unsubscribe
func (c *ClientKafka) UnSubscribe(
	_ context.Context,
	topic string,
	id string,
) (err error) {
	if c.closed {
		return fmt.Errorf("kafka client is closed")
	}
	logrus.WithFields(logrus.Fields{
		"Component": "ClientKafka",
		"Topic":     topic,
		"ID":        id}).
		Infof("unsubscribed...")
	return c.closeConsumer(
		topic,
		id,
		false)
}

// Send - sends one message and closes producer at the end
func (c *ClientKafka) Send(
	ctx context.Context,
	topic string,
	payload []byte,
	props MessageHeaders,
) (messageID []byte, err error) {
	if c.closed {
		return nil, fmt.Errorf("kafka client is closed")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	err = c.send(
		ctx,
		topic,
		payload,
		props,
	)
	return
}

// SendReceive - Send and receive message
func (c *ClientKafka) SendReceive(
	ctx context.Context,
	outTopic string,
	payload []byte,
	inTopic string,
	props MessageHeaders,
) (event *MessageEvent, err error) {
	if c.closed {
		return nil, fmt.Errorf("kafka client is closed")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	props.SetReplyTopic(inTopic)
	props.SetCorrelationID(ulid.Make().String())
	//props.SetLastOffset("0")
	props.SetDisableBatching(true)
	id, subscription, consumerChannel, err := c.getOrCreateConsumer(
		ctx,
		inTopic,
		false,
		func(ctx context.Context, e *MessageEvent) error {
			return nil
		},
		nil,
		props,
	)
	if err != nil {
		return nil, err
	}

	// send message
	if _, err := c.Publish(ctx, outTopic, payload, props); err != nil {
		return nil, err
	}

	defer func() {
		_ = c.closeConsumer(subscription.topic, id, false)
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"OutTopic":  outTopic,
			"InTopic":   inTopic,
			"Req":       props,
			"ID":        id}).
			Debugf("waiting to receive reply from send/receive")
	}

	// receive message
	select {
	case <-ctx.Done():
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"OutTopic":  outTopic,
				"InTopic":   inTopic,
				"ID":        id}).
				Debug("received done signal from context")
		}
		return nil, ctx.Err()
	case event = <-consumerChannel:
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"OutTopic":  outTopic,
				"InTopic":   inTopic,
				"Event":     string(event.Payload),
				"ID":        id}).
				Debugf("received reply from send/receive")
		}

		return event, nil
	}
}

// Publish - publishes the message and caches producer if it doesn't exist
func (c *ClientKafka) Publish(
	ctx context.Context,
	topic string,
	payload []byte,
	props MessageHeaders,
) (messageID []byte, err error) {
	if c.closed {
		return nil, fmt.Errorf("kafka client is closed")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	err = c.send(
		ctx,
		topic,
		payload,
		props,
	)
	return
}

// Close - closes all producers and consumers
func (c *ClientKafka) Close() {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()
	if c.closed {
		return
	}
	for _, next := range c.producers {
		_ = next.Close()
	}
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
		err = fmt.Errorf("could not find consumer for topic %s and id %s", topic, id)
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"Error":     err,
			"ID":        id}).
			Warnf("could not find subscription for the consumer")
		return
	}

	// delete subscriber id and remove it from multiplexer
	delete(c.consumersByID, id)
	total := subscription.mx.Remove(id)

	if total == 0 && purgeSubscription {
		_ = subscription.close()
		delete(c.consumersByTopicGroup, topicGroupKey(topic, subscription.group))
		logrus.WithFields(logrus.Fields{
			"Component":         "ClientKafka",
			"SubscriptionGroup": subscription.group,
			"Topic":             topic,
			"ID":                id}).
			Infof("closing subscriber")
	}
	return
}

func (c *ClientKafka) getOrCreateConsumer(
	ctx context.Context,
	topic string,
	shared bool,
	cb Callback,
	filter Filter,
	props MessageHeaders,
) (id string, subscription *kafkaSubscription, consumerChannel chan *MessageEvent, err error) {
	id = ulid.Make().String()
	var offset int64
	if props.GetLastOffset() != "" {
		offset = kafka.LastOffset
	} else if props.GetFirstOffset() != "" {
		offset = kafka.FirstOffset
	}
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	// use given group otherwise default group
	group := props.GetGroup(c.config.Kafka.Group)

	// find existing subscription by topic and group
	subscription = c.consumersByTopicGroup[topicGroupKey(topic, group)]

	// only create consumer-channel for send/receive because otherwise we just call callback
	// so that we don't create orphan channels or waste memory
	// TODO replace all callbacks with channels
	if props.GetCorrelationID() != "" {
		consumerChannel = make(chan *MessageEvent, c.config.Kafka.ChannelBuffer)
	}

	// if subscription exists then add subscriber to multiplexer
	if subscription != nil {
		c.consumersByID[id] = subscription
		subscription.mx.Add(ctx, id, props.GetCorrelationID(), cb, filter, consumerChannel)
		return
	}

	// otherwise, create new consumer
	subscription, err = c.createConsumer(
		ctx,
		topic,
		group,
		id,
		props.GetCorrelationID(),
		offset,
		shared,
		cb,
		filter,
		consumerChannel)
	if err != nil {
		close(consumerChannel)
		return id, nil, nil, err
	}

	// store reference of subscription to topic/group and subscriber lookup tables
	c.consumersByTopicGroup[topicGroupKey(topic, group)] = subscription
	c.consumersByID[id] = subscription
	return
}

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
	consumerChannel chan *MessageEvent) (subscription *kafkaSubscription, err error) {
	// validate topics in case we use new topic without properly creating them in kafka
	if err := c.checkTopic(topic); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ConsumerGroup": group,
			"ID":            id,
			"Topic":         topic,
			"Offset":        offset,
			"Shared":        shared,
			"Error":         err,
		}).
			Errorf("failed to validate topic %s for consumer", topic)
	}

	// creating new subscription
	subscription = &kafkaSubscription{
		topic: topic,
		mx: NewSendReceiveMultiplexer(
			ctx,
			topic,
			int64(c.config.Kafka.CommitTimeout.Seconds())),
		group: group,
	}
	// adding subscriber
	_ = subscription.mx.Add(ctx, id, correlationID, cb, filter, consumerChannel)

	// Note: using fresh context instead of ctx parameter because we need to keep subscription alive
	// even when a single subscriber dies as we have to multiplex a queue to multiple subscribers,
	// otherwise subscription will be cancelled as soon the first subscriber is done.
	subscription.ctx, subscription.cancel = context.WithCancel(context.Background())

	// using local function for defining reader because we may need to resubscribe in some cases
	initReader := func() {
		if subscription.reader != nil {
			_ = subscription.reader.Close()
		}
		subscription.reader = kafka.NewReader(kafka.ReaderConfig{
			Brokers:         c.config.Kafka.Brokers,
			GroupID:         subscription.group,
			Topic:           topic,
			StartOffset:     offset,
			MinBytes:        1,
			MaxBytes:        1024 * 1024,     // 1MB
			MaxWait:         5 * time.Second, // Maximum amount of time to wait for new data to come when fetching batches of messages from kafka.
			QueueCapacity:   100,
			RetentionTime:   time.Hour * 2, // default 24
			Dialer:          c.readerDialer,
			ReadLagInterval: -1,
			CommitInterval:  time.Second,
			//Logger:          logrus.New(),
		})
	}
	initReader()

	// receive messages asynchronously
	go func() {
		c.receiveMessages(topic, id, subscription, initReader)
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ArtifactGroup": subscription.group,
			"Topic":         topic,
			"ID":            id,
			"Offset":        offset,
			"Shared":        shared,
		}).
			Debugf("subscribed successfully!")
	}
	return
}

func (c *ClientKafka) receiveMessages(
	topic string,
	id string,
	subscription *kafkaSubscription,
	initReader func(),
) {
	defer func() {
		err := c.closeConsumer(topic, id, false)
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ConsumerGroup": subscription.group,
			"ID":            id,
			"Topic":         topic,
			"Error":         err,
			"CtxError":      subscription.ctx.Err(),
		}).
			Infof("exiting subscription loop!")
	}()

	for {
		if subscription.ctx.Err() != nil {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     subscription.topic,
					"ID":        id,
				}).
					Debugf("received done signal from context")
			}
			return
		}

		// reading messages
		//msg, err := subscription.reader.ReadMessage(subscription.ctx) // auto-commit
		msg, err := subscription.reader.FetchMessage(subscription.ctx)
		if err != nil {
			if !errors.Is(err, io.EOF) && subscription.ctx.Err() == nil {
				logrus.WithFields(logrus.Fields{
					"Component":     "ClientKafka",
					"ArtifactGroup": subscription.group,
					"Topic":         topic,
					"ID":            id,
					"Message":       msg,
					"Error":         err,
					"ErrorType":     reflect.TypeOf(err),
				}).Errorf("failed to fetch message from kafka")
			}
			if errors.Is(err, io.EOF) {
				continue
			} else if strings.Contains(err.Error(), "Rebalance In Progress") {
				initReader()
				continue
			}
			return
		}

		// defining manual ack and notifying messages
		atomic.AddInt64(&subscription.received, 1)
		ack := func() {
			_ = subscription.reader.CommitMessages(subscription.ctx, msg)
		}
		nack := func() {
		}
		event := kafkaMessageToEvent(msg, ack, nack)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":     "ClientKafka",
				"ArtifactGroup": subscription.group,
				"ID":            id,
				"Received":      subscription.received,
				"Partition":     msg.Partition,
				"Offset":        msg.Offset,
				"Event":         string(event.ID),
			}).Debugf("received")
		}
		go func() {
			_ = subscription.mx.Notify(subscription.ctx, event)
		}()
	}
}

func (c *ClientKafka) send(
	ctx context.Context,
	topic string,
	payload []byte,
	props MessageHeaders,
) (err error) {
	// building headers
	headers := make([]kafka.Header, len(props))
	i := 0
	for k, v := range props {
		headers[i] = kafka.Header{Key: k, Value: []byte(v)}
		i++
	}
	key := make([]byte, 0)
	if props.GetMessageKey() != "" {
		key = []byte(props.GetMessageKey())
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":  "ClientKafka",
			"Topic":      topic,
			"Properties": props,
			"Value":      string(payload),
		}).Debugf("sending message")
	}

	// sending messages
	err = c.getProducer(topic).WriteMessages(
		ctx,
		kafka.Message{
			Key:     key,
			Value:   payload,
			Headers: headers,
			Time:    time.Now(),
		})
	if err != nil {
		c.closeProducer(topic)
	}
	return
}

// createKafkaTopic creates kafka topic
func (c *ClientKafka) createKafkaTopic(
	topic string,
	partitions int,
	replication int) (err error) {
	conn, closer, err := c.connect()
	if err != nil {
		return err
	}
	defer func() {
		closer()
	}()
	topicConfigs := []kafka.TopicConfig{{Topic: topic, NumPartitions: partitions, ReplicationFactor: replication}}
	return conn.CreateTopics(topicConfigs...)
}

// createKafkaTopic creates kafka topic
func (c *ClientKafka) checkTopic(
	topic string,
) error {
	c.topicLock.Lock()
	defer c.topicLock.Unlock()
	if c.validTopics[topic] {
		return nil
	}
	conn, closer, err := c.connect()
	if err != nil {
		return err
	}
	defer func() {
		closer()
	}()
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return err
	}
	c.validTopics[topic] = true
	logrus.WithFields(logrus.Fields{
		"Component":  "ClientKafka",
		"Topic":      topic,
		"Partitions": partitions,
	}).
		Infof("validated topic %s", topic)
	return nil
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

func (subscription *kafkaSubscription) close() (err error) {
	if subscription.closed {
		return
	}
	if subscription.reader != nil {
		err = subscription.reader.Close()
	}
	if subscription.cancel != nil && subscription.ctx.Err() == nil {
		subscription.cancel()
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
		}).
			Debugf("closing subscription")
	}
	for _, id := range subscription.mx.SubscriberIDs() {
		subscription.mx.Remove(id)
	}

	subscription.closed = true
	return
}

// connect
func (c *ClientKafka) connect() (conn *kafka.Conn, closer connectionCloser, err error) {
	conn, err = kafka.Dial("tcp", c.config.Kafka.Brokers[0])
	closer = func() {
	}
	if err != nil {
		return
	}
	closer = func() {
		_ = conn.Close()
	}

	controller, err := conn.Controller()
	if err != nil {
		return conn, closer, err
	}
	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return conn, closer, err
	}
	closer = func() {
		_ = controllerConn.Close()
		_ = conn.Close()
	}
	return controllerConn, closer, nil
}

func (c *ClientKafka) closeProducer(
	topic string,
) {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()
	writer := c.producers[topic]
	if writer != nil {
		_ = writer.Close()
	}
	delete(c.producers, topic)
}

func (c *ClientKafka) getProducer(
	topic string,
) *kafka.Writer {
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
	writer := c.producers[topic]
	if writer == nil {
		writer = &kafka.Writer{
			Addr:         kafka.TCP(c.config.Kafka.Brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			WriteTimeout: 5 * time.Second,
			ReadTimeout:  5 * time.Second,
			BatchTimeout: 10 * time.Millisecond,
			BatchSize:    10,
			RequiredAcks: 1,
			//Compression:  kafka.Snappy,
			//Logger:       logrus.New(),
		}
		c.producers[topic] = writer
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     topic,
			}).Debugf("adding producer")
		}
	}
	return writer
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
	event.ProducerName = event.Producer()
	return event
}

func topicGroupKey(topic string, group string) string {
	return topic + ":" + group
}
