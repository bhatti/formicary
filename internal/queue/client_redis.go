package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/async"
	"plexobject.com/formicary/internal/math"
	"plexobject.com/formicary/internal/types"
	"sync"
	"time"
)

// ClientRedis structure implements interface for queuing messages using Redis
type ClientRedis struct {
	redisClient       *redis.Client
	pubsubConnections map[string]*redisPubSubConnection
	queueSubscribers  map[string]*redisQueueSubscription
	maxWait           time.Duration
	lock              sync.RWMutex
	closed            bool
}

// redisPubSubConnection structure
type redisPubSubConnection struct {
	topic     *redis.PubSub
	consumers map[string]redisPubSubSubscription
	done      chan bool
}

// redisPubSubSubscription structure
type redisPubSubSubscription struct {
	callback Callback
	filter   Filter
}

// redisQueueSubscription structure
type redisQueueSubscription struct {
	mx   *SendReceiveMultiplexer
	done chan bool
}

// HeadersPayload structure
type HeadersPayload struct {
	Properties map[string]string
	Payload    []byte
}

// newClientRedis creates structure for implementing queuing operations
func newClientRedis(
	config *types.RedisConfig,
) (Client, error) {
	if config.Host == "" || config.Port == 0 {
		return nil, fmt.Errorf("redis is not configured %s:%d", config.Host, config.Port)
	}
	opts := &redis.Options{
		Addr: fmt.Sprintf("%s:%d", config.Host, config.Port),
		DB:   0,
	}
	if config.Password != "" {
		opts.Password = config.Password
	}
	redisClient := redis.NewClient(opts)
	return &ClientRedis{
		redisClient:       redisClient,
		maxWait:           config.MaxPopWait,
		pubsubConnections: make(map[string]*redisPubSubConnection),
		queueSubscribers:  make(map[string]*redisQueueSubscription),
	}, nil
}

// Subscribe - nonblocking subscribe that calls back handler upon message
func (c *ClientRedis) Subscribe(
	ctx context.Context,
	topic string,
	shared bool,
	cb Callback,
	filter Filter,
	_ MessageHeaders,
) (id string, err error) {
	id = uuid.NewV4().String()
	if cb == nil {
		return id, fmt.Errorf("callback function is not specified")
	}
	if ctx.Err() != nil {
		return id, ctx.Err()
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id}).
			Debug("Creating goroutine to receive messages")
	}
	if shared {
		return id, c.addQueueSubscriber(ctx, topic, id, cb, filter)
	}
	return id, c.addPubSubscriber(ctx, topic, id, cb, filter)
}

// UnSubscribe - unsubscribe
func (c *ClientRedis) UnSubscribe(
	_ context.Context,
	topic string,
	id string,
) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	sharedConn := c.pubsubConnections[topic]
	if sharedConn != nil {
		delete(sharedConn.consumers, id)
		if len(sharedConn.consumers) == 0 {
			close(sharedConn.done)
			delete(c.pubsubConnections, topic)
		}
	}
	subscription := c.queueSubscribers[topic]
	if subscription != nil {
		count := subscription.mx.Remove(id)
		if count == 0 {
			delete(c.queueSubscribers, topic)
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientRedis",
				"ID":        id,
				"Topic":     topic}).
				Debugf("unsubscribing redis topic")
		}
	}
	return nil
}

// Send - sends one message
func (c *ClientRedis) Send(
	ctx context.Context,
	topic string,
	payload []byte,
	props MessageHeaders,
) (messageID []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	messageID = make([]byte, 0)
	data, err := toHeadersPayloadData(props, payload)
	if err != nil {
		return nil, err
	}
	_, err = c.redisClient.RPush(ctx, topic, data).Result()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Error":     err,
			"Topic":     topic}).
			Debugf("Sent message using RPush!")
	}
	return
}

// SendReceive - Send and receive message
func (c *ClientRedis) SendReceive(
	ctx context.Context,
	outTopic string,
	payload []byte,
	inTopic string,
	props MessageHeaders,
) (event *MessageEvent, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// redis doesn't support consumer groups so only one subscriber can consume it so making reply-topic unique
	inTopic = inTopic + "-" + uuid.NewV4().String() // make it unique
	props.SetCorrelationID(uuid.NewV4().String())
	props.SetReplyTopic(inTopic)

	_, err = c.Send(ctx, outTopic, payload, props)
	if err != nil {
		return nil, err
	}
	event, err = c.pollQueue(ctx, inTopic)
	return
}

// Publish - publishes the message and caches producer if it doesn't exist
func (c *ClientRedis) Publish(
	ctx context.Context,
	topic string,
	payload []byte,
	props MessageHeaders,
) (messageID []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	messageID = make([]byte, 0)
	data, err := toHeadersPayloadData(props, payload)
	if err != nil {
		return nil, err
	}
	err = c.redisClient.Publish(ctx, topic, data).Err()
	return
}

// Close - closes all subscribers
func (c *ClientRedis) Close() {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, next := range c.pubsubConnections {
		close(next.done)
	}
	for _, next := range c.queueSubscribers {
		for _, id := range next.mx.SubscriberIDs() {
			next.mx.Remove(id)
		}
		close(next.done)
	}
	c.closed = true
}

//////////////////////////// PRIVATE METHODS /////////////////////////////
func (c *ClientRedis) addQueueSubscriber(
	ctx context.Context,
	topic string,
	id string,
	cb Callback,
	filter Filter) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	subscription := c.queueSubscribers[topic]
	if subscription != nil {
		_ = subscription.mx.Add(ctx, id, "", cb, filter, nil)
		return nil
	}
	subscription = &redisQueueSubscription{
		mx: NewSendReceiveMultiplexer(
			ctx,
			topic,
			0),
		done: make(chan bool),
	}
	c.queueSubscribers[topic] = subscription
	_ = subscription.mx.Add(ctx, id, "", cb, filter, nil)

	go func() {
		c.doQueueReceive(ctx, subscription, topic, id)
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id}).
			Debug("subscribed successfully!")
	}

	return
}

func (c *ClientRedis) doQueueReceive(
	ctx context.Context,
	subscription *redisQueueSubscription,
	topic string,
	id string,
) {
	started := time.Now()
	defer func() {
		c.lock.Lock()
		defer c.lock.Unlock()
		delete(c.queueSubscribers, topic)
		close(subscription.done)
	}()
	for {
		event, err := c.pollQueue(ctx, topic)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientRedis",
				"Topic":     topic,
				"Error":     err,
				"ID":        id}).
				Warnf("failed to receive event!")
		} else {
			subscription.mx.Notify(ctx, event)
		}
		select {
		case <-subscription.done:
			return
		case <-ctx.Done():
			elapsed := time.Since(started)
			err = ctx.Err()
			logrus.WithFields(logrus.Fields{
				"Component": "ClientRedis",
				"Topic":     topic,
				"Elapsed":   elapsed,
				"Error":     err,
				"ID":        id}).
				Warn("context done while receiving event!")
			return
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

func (c *ClientRedis) addPubSubscriber(
	ctx context.Context,
	topic string,
	id string,
	cb Callback,
	filter Filter) (err error) {
	var sharedConn *redisPubSubConnection
	{
		c.lock.Lock()
		defer c.lock.Unlock()
		sharedConn = c.pubsubConnections[topic]

		if sharedConn != nil {
			sharedConn.consumers[id] = redisPubSubSubscription{
				callback: cb,
				filter:   filter,
			}
			return nil
		}

		sharedConn = &redisPubSubConnection{
			topic:     c.redisClient.Subscribe(ctx, topic),
			consumers: make(map[string]redisPubSubSubscription),
			done:      make(chan bool),
		}
		sharedConn.consumers[id] = redisPubSubSubscription{
			callback: cb,
			filter:   filter,
		}
		c.pubsubConnections[topic] = sharedConn
	}

	go func() {
		c.doPubSubReceive(ctx, sharedConn, topic)
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id}).
			Debug("subscribed successfully!")
	}

	return
}

func (c *ClientRedis) doPubSubReceive(
	ctx context.Context,
	sharedConn *redisPubSubConnection,
	topic string,
) {
	maxWait := 30 * time.Second
	minWait := 10 * time.Millisecond
	curWait := minWait
	subscribed := true
	defer func() {
		_ = sharedConn.topic.Unsubscribe(ctx, topic)
		_ = sharedConn.topic.Close()
	}()
	for {
		if err := c.redisClient.Ping(ctx).Err(); err != nil {
			if curWait < maxWait {
				curWait *= 2
			}
			subscribed = false
		}
		if !subscribed {
			if err := sharedConn.topic.Subscribe(ctx, topic); err == nil {
				subscribed = true
			}
		}
		select {
		case msg := <-sharedConn.topic.Channel():
			hp := toHeadersPayload([]byte(msg.Payload))
			c.notifyPubSubConsumers(ctx, topic, hp.Properties, hp.Payload)
			curWait = minWait
		case <-sharedConn.done:
			return
		case <-ctx.Done():
			return
		case <-time.After(math.MinDuration(maxWait, curWait)):
		}
	}
}

func (c *ClientRedis) pollQueue(
	ctx context.Context,
	inTopic string,
) (event *MessageEvent, err error) {
	event = &MessageEvent{
		Topic:        inTopic,
		ProducerName: "",
		Properties:   make(map[string]string),
		ID:           make([]byte, 0),
		PublishTime:  time.Now(),
		Ack:          func() {},
		Nack:         func() {},
	}
	minWait := 10 * time.Millisecond
	started := time.Now()
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		res, err := c.redisClient.BLPop(ctx, c.maxWait, inTopic).Result()
		if err != nil || len(res) < 2 {
			return false, nil, nil
		}
		hp := toHeadersPayload([]byte(res[1]))
		event.Properties = hp.Properties
		event.Payload = hp.Payload
		return true, nil, nil
	}

	future := async.ExecutePollingWithSignal(
		ctx,
		handler,
		async.NoAbort,
		0,
		minWait,
		10*minWait)
	_, err = future.Await(ctx)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Elapsed":   time.Since(started),
			"Error":     err,
			"Topic":     inTopic}).
			Debugf("received message!")
	}
	return event, err
}

// notifyPubSubConsumers all subscribers for pub/sub
func (c *ClientRedis) notifyPubSubConsumers(
	ctx context.Context,
	topic string,
	props map[string]string,
	data []byte) {
	// build event
	event := &MessageEvent{
		Topic:        topic,
		ProducerName: "",
		Properties:   props,
		ID:           make([]byte, 0),
		Payload:      data,
		PublishTime:  time.Now(),
		Ack:          func() {},
		Nack:         func() {},
	}

	c.lock.RLock()
	defer c.lock.RUnlock()
	connection := c.pubsubConnections[topic]
	if connection != nil {
		// notify subscribers
		for _, next := range connection.consumers {
			if next.filter == nil || next.filter(ctx, event) {
				_ = next.callback(ctx, event)
			}
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ClientRedis",
				"Topic":     topic,
				"Consumers": len(connection.consumers)}).
				Debugf("consumers notified!")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"Data":      len(data)}).
			Warn("no consumers found!")
	}
}

func toHeadersPayloadData(props map[string]string, payload []byte) ([]byte, error) {
	hp := HeadersPayload{Properties: props, Payload: payload}
	return json.Marshal(hp)
}

func toHeadersPayload(data []byte) *HeadersPayload {
	hp := HeadersPayload{}
	err := json.Unmarshal(data, &hp)
	if err != nil {
		hp.Properties = make(map[string]string)
		hp.Payload = data
	}
	return &hp
}
