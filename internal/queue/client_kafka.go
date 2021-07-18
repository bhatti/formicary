package queue

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/types"
	"strings"
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
	config    *types.CommonConfig
	lock      sync.Mutex
	producers map[string]sarama.SyncProducer
	consumers map[string]*kafkaSubscription
	closed    bool
}

// newKafkaClient creates structure for implementing queuing operations
func newKafkaClient(config *types.CommonConfig) (Client, error) {
	return &ClientKafka{
		config:    config,
		producers: make(map[string]sarama.SyncProducer),
		consumers: make(map[string]*kafkaSubscription),
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
	if c.config.Kafka.Group == "" {
		return c.partitionSubscribe(ctx, topic, id, props, shared, cb)
	}
	return c.groupSubscribe(ctx, topic, id, props, shared, cb)
}

// partitionSubscribe - nonblocking subscribe that calls back handler upon message
func (c *ClientKafka) partitionSubscribe(
	ctx context.Context,
	topic string,
	id string,
	props map[string]string,
	shared bool,
	cb Callback) (err error) {
	go func() {
		for {
			_, err = c.getOrCreateConsumer(ctx, topic, id, props, shared, cb)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     topic,
					"Error":     err,
					"ID":        id}).
					Warn("failed to get consumer for subscription, will try again")
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
	}()
	return nil
}

// groupSubscribe - subscribe to consumer group
func (c *ClientKafka) groupSubscribe(
	ctx context.Context,
	topic string,
	id string,
	props map[string]string,
	shared bool,
	cb Callback) (err error) {
	// creating consumer in loop
	go func() {
		for {
			_, err = c.getOrCreateConsumer(ctx, topic, id, props, shared, cb)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     topic,
					"Error":     err,
					"ID":        id}).
					Warn("failed to get consumer group for subscription, will try again")
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
		// will call doReceive in ConsumeClaim
	}()
	return nil
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
	reusableTopic bool) (messageID []byte, err error) {
	if reusableTopic {
		return c.sendKafkaMessageWithReusableProducer(
			ctx,
			topic,
			props,
			payload,
			true)
	}
	return c.sendKafkaMessageWithoutReusableProducer(
		ctx,
		topic,
		props,
		payload)
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
	id := uuid.NewV4().String()
	var subscription *kafkaSubscription
	cb := func(ctx context.Context, e *MessageEvent) error {
		event = e
		if subscription != nil {
			_ = subscription.close()
		}
		return nil
	}
	subscription, err = c.createConsumer(ctx, inTopic, id, props, false, sarama.OffsetNewest, cb)
	if err != nil {
		return nil, err
	}

	// send message
	if _, err := c.Publish(ctx, outTopic, props, payload, false); err != nil {
		return nil, err
	}

	<-subscription.done
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return
}

// Publish - publishes the message and caches producer if doesn't exist
func (c *ClientKafka) Publish(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	disableBatching bool) (messageID []byte, err error) {
	return c.sendKafkaMessageWithReusableProducer(
		ctx,
		topic,
		props,
		payload,
		disableBatching)
}

// Close - closes all producers and consumers
func (c *ClientKafka) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, next := range c.producers {
		_ = next.Close()
	}
	for _, subscription := range c.consumers {
		_ = subscription.close()
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
	key := buildKey(topic, id)
	subscription := c.consumers[key]
	if subscription == nil {
		err = fmt.Errorf("could not find consumer for topic %s and id %s", topic, id)
		return
	}
	err = subscription.close()
	delete(c.consumers, key)
	return
}

func (c *ClientKafka) getOrCreateConsumer(
	ctx context.Context,
	topic string,
	id string,
	props map[string]string,
	shared bool,
	cb Callback) (subscription *kafkaSubscription, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	key := buildKey(topic, id)
	subscription = c.consumers[key]

	if subscription != nil {
		return subscription, nil
	}
	subscription, err = c.createConsumer(
		ctx,
		topic,
		id,
		props,
		shared,
		sarama.OffsetNewest,
		cb)
	if err != nil {
		return nil, err
	}

	c.consumers[key] = subscription
	return
}

func (c *ClientKafka) createConsumer(
	ctx context.Context,
	topic string,
	id string,
	props map[string]string,
	_ bool,
	offset int64,
	cb Callback) (subscription *kafkaSubscription, err error) {
	subscription = &kafkaSubscription{
		id:    id,
		topic: topic,
		cb:    cb,
		done:  make(chan bool),
	}
	subscription.ctx, subscription.cancel = context.WithCancel(ctx)

	clientConfig, err := c.config.Kafka.BuildSaramaConfig(c.config.Debug, props["oldest"] == "true")
	if err != nil {
		return nil, err
	}
	subscription.consumer, err = sarama.NewConsumer(c.config.Kafka.Brokers, clientConfig)
	if err != nil {
		return nil, err
	}
	if c.config.Kafka.Group == "" {
		partitions, err := subscription.consumer.Partitions(topic)
		if err != nil {
			return nil, err
		}
		for _, partition := range partitions {
			partitionedConsumer, err := subscription.consumer.ConsumePartition(topic, partition, offset)
			if err != nil {
				return nil, err
			}
			subscription.partitionedConsumers = append(subscription.partitionedConsumers, partitionedConsumer)
			go func() {
				for {
					_, _, done := kafkaReceive(
						ctx,
						subscription,
						partitionedConsumer.Messages(),
						partitionedConsumer.Errors())
					if done {
						_ = c.closeConsumer(topic, id)
						return
					}
				}
			}()
		}
	} else {
		subscription.consumerGroupReady = make(chan bool)
		subscription.client, err = sarama.NewConsumerGroup(c.config.Kafka.Brokers, c.config.Kafka.Group, clientConfig)
		if err != nil {
			return nil, err
		}

		go func() {
			logrus.WithFields(logrus.Fields{
				"Component":     "ClientKafka",
				"ConsumerGroup": c.config.Kafka.Group,
				"Topic":         topic,
				"Offset":        offset,
				"Brokers":       c.config.Kafka.Brokers,
				"Error":         err,
				"ID":            id}).
				Infof(">>>>consumer group BEGIN!")
			for {
				// `Consume` should be called inside an infinite loop, when a
				// server-side rebalanced happens, the consumer session will need to be
				// recreated to get the new claims
				if err := subscription.client.Consume(ctx, strings.Split(topic, ","), subscription); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":     "ClientKafka",
						"ConsumerGroup": c.config.Kafka.Group,
						"Topic":         topic,
						"Offset":        offset,
						"Brokers":       c.config.Kafka.Brokers,
						"Error":         err,
						"ID":            id}).
						Errorf("failed to rebalance consumer group!")
				}
				logrus.WithFields(logrus.Fields{
					"Component":     "ClientKafka",
					"ConsumerGroup": c.config.Kafka.Group,
					"Topic":         topic,
					"Offset":        offset,
					"Brokers":       c.config.Kafka.Brokers,
					"Error":         err,
					"ID":            id}).
					Infof(">>>>consumer group END!")
				// check if context was cancelled, signaling that the consumer should stop
				if ctx.Err() != nil {
					return
				}
				subscription.consumerGroupReady = make(chan bool)
			}
		}()
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ConsumerGroup": c.config.Kafka.Group,
			"Topic":         topic,
			"Offset":        offset,
			"Brokers":       c.config.Kafka.Brokers,
			"ID":            id}).
			Debugf("subscribed successfully!")
	}

	return
}

func (c *ClientKafka) getOrCreateProducer(
	topic string,
) (producer sarama.SyncProducer, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	producer = c.producers[topic]
	if producer == nil {
		clientConfig, err := c.config.Kafka.BuildSaramaConfig(c.config.Debug, false)
		if err != nil {
			return nil, err
		}
		producer, err = sarama.NewSyncProducer(c.config.Kafka.Brokers, clientConfig)
		if err != nil {
			return nil, err
		}
		c.producers[topic] = producer
	}
	return
}

// CreateKafkaTopic creates kafka topic
func (c *ClientKafka) CreateKafkaTopic(
	topic string,
	partitions int32,
	replication int16) (err error) {
	broker := sarama.NewBroker(c.config.Kafka.Brokers[0])
	defer func() {
		_ = broker.Close()
	}()

	clientConfig, err := c.config.Kafka.BuildSaramaConfig(c.config.Debug, false)
	if err != nil {
		return err
	}
	err = broker.Open(clientConfig)
	if err != nil {
		return err
	}

	_, err = broker.Connected()
	if err != nil {
		return err
	}

	topicDetail := &sarama.TopicDetail{}
	topicDetail.NumPartitions = partitions
	topicDetail.ReplicationFactor = replication
	topicDetail.ConfigEntries = make(map[string]*string)

	topicDetails := make(map[string]*sarama.TopicDetail)
	topicDetails[topic] = topicDetail

	request := sarama.CreateTopicsRequest{
		Timeout:      time.Second * 15,
		TopicDetails: topicDetails,
	}

	// Send request to Broker
	response, err := broker.CreateTopics(&request)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"Error":     err,
		}).
			Errorf("failed to create topic")
		return err
	}

	for key, val := range response.TopicErrors {
		if val.ErrMsg != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     topic,
				"Message":   val.ErrMsg,
				"Key":       key,
				"Error":     val.Err,
			}).
				Warnf("failed to create topic")
			err = val.Err
		}
	}
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

func (c *ClientKafka) sendKafkaMessageWithReusableProducer(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte,
	_ bool) (b []byte, err error) {
	var producer sarama.SyncProducer
	producer, err = c.getOrCreateProducer(topic)
	if err != nil {
		return
	}
	return sendKafkaMessage(ctx, producer, topic, props, payload)
}

func (c *ClientKafka) sendKafkaMessageWithoutReusableProducer(
	ctx context.Context,
	topic string,
	props map[string]string,
	payload []byte) (b []byte, err error) {
	var producer sarama.SyncProducer
	clientConfig, err := c.config.Kafka.BuildSaramaConfig(c.config.Debug, false)
	if err != nil {
		return nil, err
	}
	producer, err = sarama.NewSyncProducer(c.config.Kafka.Brokers, clientConfig)
	if err != nil {
		return
	}
	defer func() {
		_ = producer.Close()
	}()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     topic,
			"Producer":  producer}).
			Debugf("sending single message...")
	}
	return sendKafkaMessage(ctx, producer, topic, props, payload)
}

// kafkaSubscription structure
type kafkaSubscription struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	id                   string
	topic                string
	cb                   Callback
	consumer             sarama.Consumer
	partitionedConsumers []sarama.PartitionConsumer
	done                 chan bool
	consumerGroupReady   chan bool // groupConsumer represents a Sarama consumer group consumer
	client               sarama.ConsumerGroup
	received             int64
	closed               bool
}

func (subscription *kafkaSubscription) close() (err error) {
	if subscription.closed {
		return
	}
	for _, partitionedConsumer := range subscription.partitionedConsumers {
		err = partitionedConsumer.Close()
	}
	if subscription.consumer != nil {
		err = subscription.consumer.Close()
	}
	if subscription.client != nil {
		err = subscription.client.Close()
		close(subscription.consumerGroupReady)
	}
	if subscription.ctx.Err() != nil {
		subscription.cancel()
	}
	//close(subscription.consumerChannel)
	close(subscription.done)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "ClientKafka",
			"ConsumerGroup": subscription.client != nil,
			"Topic":         subscription.topic,
			"ID":            subscription.id}).
			Debugf("closing subscription")
	}

	subscription.closed = true
	return
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (subscription *kafkaSubscription) Setup(sarama.ConsumerGroupSession) error {
	logrus.WithFields(logrus.Fields{
		"Component":     "ClientKafka",
		"ConsumerGroup": subscription.client != nil,
		"Topic":         subscription.topic,
		"ID":            subscription.id}).
		Infof("setup consumer session")
	// Mark the consumer as consumerGroupReady
	close(subscription.consumerGroupReady)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (subscription *kafkaSubscription) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (subscription *kafkaSubscription) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	logrus.WithFields(logrus.Fields{
		"Component":     "ClientKafka",
		"ConsumerGroup": subscription.client != nil,
		"Topic":         subscription.topic,
		"ID":            subscription.id}).
		Infof("claiming messages")
	for {
		message, ack, done := kafkaReceive(
			subscription.ctx,
			subscription,
			claim.Messages(),
			make(chan *sarama.ConsumerError))
		if message != nil {
			if ack {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     message.Topic,
					"Message":   string(message.Key),
					"Timestamp": message.Timestamp}).
					Infof("message claimed")
				session.MarkMessage(message, "")
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     message.Topic,
					"Message":   string(message.Key),
					"Timestamp": message.Timestamp}).
					Infof("skip claiming message")
			}
		}
		if done {
			break
		}
	}
	return nil
}

func kafkaReceive(
	ctx context.Context,
	subscription *kafkaSubscription,
	consumerChannel <-chan *sarama.ConsumerMessage,
	errorChannel <-chan *sarama.ConsumerError) (*sarama.ConsumerMessage, bool, bool) {
	defer recoverNilMessage(subscription.topic, subscription.id)
	ackFlag := false
	select {
	case <-ctx.Done():
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientKafka",
				"Topic":     subscription.topic,
				"ID":        subscription.id}).
				Debug("received done signal from context")
		}
		return nil, false, true
	case <-subscription.done:
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
			"ID":        subscription.id}).
			Warn("received done signal from channel")
		return nil, false, true
	case consumerError := <-errorChannel:
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
			"Error":     consumerError,
			"ID":        subscription.id}).
			Warn("received error from consumer")
		return nil, false, true
	case msg := <-consumerChannel:
		if msg == nil {
			return nil, false, false
		}
		atomic.AddInt64(&subscription.received, 1)
		ack := func() {
			ackFlag = true
		}
		nack := func() {
			ackFlag = false
		}
		event := kafkaMessageToEvent(msg, ack, nack)
		logrus.WithFields(logrus.Fields{
			"Component": "ClientKafka",
			"Topic":     subscription.topic,
			"Received":  subscription.received,
			"Partition": msg.Partition,
			"Offset":    msg.Offset,
			"Event":     string(event.ID),
			"ID":        subscription.id}).
			Infof("received next message...")
		if err := subscription.cb(ctx, event); err != nil {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "ClientKafka",
					"Topic":     subscription.topic,
					"ID":        subscription.id,
					"Message":   string(msg.Value)}).
					Debug("failed to handle message")
			}
		}
		return msg, ackFlag, false
	}
}

func sendKafkaMessage(
	_ context.Context,
	producer sarama.SyncProducer,
	topic string,
	props map[string]string,
	payload []byte) ([]byte, error) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}
	if props[MessageKey] != "" {
		msg.Key = sarama.StringEncoder(props[MessageKey])
	}
	for k, v := range props {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}
	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		return nil, err
	}
	msgID := fmt.Sprintf("%d:%d", partition, offset)
	logrus.WithFields(logrus.Fields{
		"Component": "ClientKafka",
		"Topic":     topic,
		"Partition": partition,
		"Offset":    offset,
		"ID":        msgID}).
		Infof("sent message...")
	return []byte(msgID), nil
}

func kafkaMessageToEvent(msg *sarama.ConsumerMessage, ack AckHandler, nack AckHandler) (event *MessageEvent) {
	event = &MessageEvent{
		Topic:       msg.Topic,
		Payload:     msg.Value,
		ID:          []byte(fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)),
		PublishTime: msg.Timestamp,
		Properties:  make(map[string]string),
		Ack:         ack,
		Nack:        nack,
	}
	if msg.Headers != nil && len(msg.Headers) > 0 {
		for _, h := range msg.Headers {
			event.Properties[string(h.Key)] = string(h.Value)
		}
	}
	event.ProducerName = event.Properties[ProducerKey]
	return event
}

//consumerChannel      <-chan *sarama.ConsumerMessage
