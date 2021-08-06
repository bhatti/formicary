package queue

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
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
	consumers map[string]Callback
	done      chan bool
}

// redisQueueSubscription structure
type redisQueueSubscription struct {
	cb   Callback
	done chan bool
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
	id string,
	_ map[string]string,
	shared bool,
	cb Callback) (err error) {
	if cb == nil {
		return fmt.Errorf("callback function is not specified")
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id}).
			Debug("Creating goroutine to receive messages")
	}
	if shared {
		return c.addQueueSubscriber(ctx, topic, id, cb)
	}
	return c.addPubSubscriber(ctx, topic, id, cb)
}

// UnSubscribe - unsubscribe
func (c *ClientRedis) UnSubscribe(
	_ context.Context,
	topic string,
	id string) (err error) {
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
	key := buildKey(topic, id)
	delete(c.queueSubscribers, key)
	return nil
}

// Send - sends one message
func (c *ClientRedis) Send(
	ctx context.Context,
	topic string,
	_ map[string]string,
	payload []byte,
	_ bool) (messageID []byte, err error) {
	messageID = make([]byte, 0)
	_, err = c.redisClient.RPush(ctx, topic, payload).Result()
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
	props map[string]string,
	payload []byte,
	inTopic string,
) (event *MessageEvent, err error) {
	_, err = c.Send(ctx, outTopic, props, payload, true)
	if err != nil {
		return nil, err
	}
	event, err = c.pollQueue(ctx, inTopic)
	return
}

// Publish - publishes the message and caches producer if doesn't exist
func (c *ClientRedis) Publish(
	ctx context.Context,
	topic string,
	_ map[string]string,
	payload []byte,
	_ bool) (messageID []byte, err error) {
	messageID = make([]byte, 0)
	err = c.redisClient.Publish(ctx, topic, payload).Err()
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
		close(next.done)
	}
	c.closed = true
}

//////////////////////////// PRIVATE METHODS /////////////////////////////
func (c *ClientRedis) addQueueSubscriber(
	ctx context.Context,
	topic string,
	id string,
	cb Callback) (err error) {
	started := time.Now()
	key := buildKey(topic, id)
	c.lock.Lock()
	defer c.lock.Unlock()
	subscriber := c.queueSubscribers[key]
	if subscriber != nil {
		return nil
	}
	subscription := &redisQueueSubscription{
		cb:   cb,
		done: make(chan bool),
	}
	c.queueSubscribers[key] = subscription

	go func() {
		defer func() {
			c.lock.Lock()
			defer c.lock.Unlock()
			delete(c.queueSubscribers, key)
			close(subscription.done)
		}()
		for {
			event, err := c.pollQueue(ctx, topic)
			if err != nil {
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component": "ClientRedis",
						"Topic":     topic,
						"Error":     err,
						"ID":        id}).
						Debugf("failed to receive event!")
				}
			} else {
				_ = cb(ctx, event)
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

func (c *ClientRedis) addPubSubscriber(
	ctx context.Context,
	topic string,
	id string,
	cb Callback) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	sharedConn := c.pubsubConnections[topic]

	if sharedConn != nil {
		sharedConn.consumers[id] = cb
		return nil
	}

	sharedConn = &redisPubSubConnection{
		topic:     c.redisClient.Subscribe(ctx, topic),
		consumers: make(map[string]Callback),
		done:      make(chan bool),
	}
	sharedConn.consumers[id] = cb
	c.pubsubConnections[topic] = sharedConn

	maxWait := 30 * time.Second
	minWait := 10 * time.Millisecond
	curWait := minWait
	subscribed := true
	go func() {
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
				if err = sharedConn.topic.Subscribe(ctx, topic); err == nil {
					subscribed = true
				}
			}
			select {
			case msg := <-sharedConn.topic.Channel():
				c.notifyPubSubConsumers(ctx, topic, []byte(msg.Payload))
				curWait = minWait
			case <-sharedConn.done:
				return
			case <-ctx.Done():
				err = ctx.Err()
				return
			case <-time.After(math.MinDuration(maxWait, curWait)):
			}
		}
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

func (c *ClientRedis) pollQueue(ctx context.Context, inTopic string) (event *MessageEvent, err error) {
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
		event.Payload = []byte(res[1])
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
	data []byte) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	event := &MessageEvent{
		Topic:        topic,
		ProducerName: "",
		Properties:   make(map[string]string),
		ID:           make([]byte, 0),
		Payload:      data,
		PublishTime:  time.Now(),
		Ack:          func() {},
		Nack:         func() {},
	}

	connection := c.pubsubConnections[topic]
	if connection != nil {
		for _, next := range connection.consumers {
			_ = next(ctx, event)
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
