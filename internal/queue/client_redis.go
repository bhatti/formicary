package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/async"
	"plexobject.com/formicary/internal/math"
	"plexobject.com/formicary/internal/types"
	"sync"
	"time"
)

// ClientRedis implements the queue.Client interface using Redis
type ClientRedis struct {
	*MetricsCollector
	redisClient          *redis.Client
	pubsubConnections    map[string]*redisPubSubConnection
	queueSubscribers     map[string]*redisQueueSubscription
	maxWait              time.Duration
	config               *types.CommonConfig
	consumerProducerLock sync.RWMutex
}

type redisPubSubConnection struct {
	topic     *redis.PubSub
	consumers map[string]*pulsarSubscription
	done      chan bool
}

type redisQueueSubscription struct {
	topic    string
	callback Callback
	filter   Filter
	mx       *SendReceiveMultiplexer
	ctx      context.Context
	cancel   context.CancelFunc
}

func newRedisClient(ctx context.Context, config *types.CommonConfig, _ string) (*ClientRedis, error) {
	if config.Redis.Host == "" || config.Redis.Port == 0 {
		return nil, fmt.Errorf("redis is not configured %v", config.Redis)
	}

	opts := &redis.Options{
		Addr: fmt.Sprintf("%s:%d", config.Redis.Host, config.Redis.Port),
		DB:   0,
	}
	if config.Redis.Password != "" {
		opts.Password = config.Redis.Password
	}

	redisClient := redis.NewClient(opts)

	return &ClientRedis{
		redisClient:       redisClient,
		config:            config,
		maxWait:           config.Redis.MaxPopWait,
		pubsubConnections: make(map[string]*redisPubSubConnection),
		queueSubscribers:  make(map[string]*redisQueueSubscription),
		MetricsCollector:  newMetricsCollector(ctx),
	}, nil
}

// Subscribe implements queue.Client interface
func (c *ClientRedis) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
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

	if opts.Shared {
		err := c.addQueueSubscriber(ctx, opts.Topic, subID, opts)
		if err != nil {
			return "", fmt.Errorf("failed to create queue subscription: %w", err)
		}
	} else {
		err := c.addPubSubscriber(ctx, opts.Topic, subID, opts)
		if err != nil {
			return "", fmt.Errorf("failed to create pub/sub subscription: %w", err)
		}
	}

	return subID, nil
}

// UnSubscribe implements queue.Client interface
func (c *ClientRedis) UnSubscribe(_ context.Context, topic string, id string) error {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if sharedConn := c.pubsubConnections[topic]; sharedConn != nil {
		delete(sharedConn.consumers, id)
		if len(sharedConn.consumers) == 0 {
			close(sharedConn.done)
			delete(c.pubsubConnections, topic)
		}
	}

	if subscription := c.queueSubscribers[topic]; subscription != nil {
		subscription.cancel()
		delete(c.queueSubscribers, topic)
	}

	return nil
}

// Send implements queue.Client interface
func (c *ClientRedis) Send(ctx context.Context, topic string, payload []byte,
	props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context is cancelled: %w", ctx.Err())
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	if props == nil {
		props = make(MessageHeaders)
	}
	// Validate input
	if err := validateSendRequest(topic, payload, props, c.config.Queue); err != nil {
		return nil, err
	}

	data, err := toHeadersPayloadData(props, payload)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1)
		return nil, err
	}

	_, err = c.redisClient.RPush(ctx, topic, data).Result()
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1)
		return nil, err
	}

	c.updateMetrics(topic, 1, 0, 0, 0, -1)
	return []byte{}, nil // Redis doesn't have message IDs
}

// SendReceive implements queue.Client interface
func (c *ClientRedis) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}
	// Validate request
	if err := validateSendReceiveRequest(req, c.config.Queue); err != nil {
		return nil, err
	}

	// Make reply topic unique for Redis
	replyTopic := ulid.Make().String()
	req.Props.SetReplyTopic(replyTopic)
	req.Props.SetCorrelationID(ulid.Make().String())

	// Send request
	_, err := c.Send(ctx, req.OutTopic, req.Payload, req.Props)
	if err != nil {
		return nil, err
	}

	// Wait for response
	event, err := c.pollQueue(ctx, replyTopic)
	if err != nil {
		return nil, err
	}

	return &SendReceiveResponse{
		Event: event,
		Ack:   func() {}, // Redis doesn't need explicit acks
		Nack:  func() {},
	}, nil
}

// Publish implements queue.Client interface
func (c *ClientRedis) Publish(ctx context.Context, topic string, payload []byte,
	props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}
	if props == nil {
		props = make(MessageHeaders)
	}
	// Validate input
	if err := validateSendRequest(topic, payload, props, c.config.Queue); err != nil {
		return nil, err
	}

	id := props.GetMessageKey()
	if id == "" {
		id = ulid.Make().String()
	}
	props.SetMessageKey(id)
	data, err := toHeadersPayloadData(props, payload)
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1)
		return nil, err
	}

	err = c.redisClient.Publish(ctx, topic, data).Err()
	if err != nil {
		c.updateMetrics(topic, 0, 0, 0, 1, -1)
		return nil, err
	}

	c.updateMetrics(topic, 1, 0, 0, 0, -1)
	return []byte(id), nil
}

// CreateTopicIfNotExists implements queue.Client interface
func (c *ClientRedis) CreateTopicIfNotExists(_ context.Context, topic string, _ *TopicConfig) error {
	// Redis doesn't need explicit topic creation
	return validateTopic(topic)
}

// Close implements queue.Client interface
func (c *ClientRedis) Close() {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	if c.closed {
		return
	}

	for _, conn := range c.pubsubConnections {
		close(conn.done)
	}

	for _, sub := range c.queueSubscribers {
		sub.cancel()
	}

	_ = c.redisClient.Close()
	c.closed = true
}

// Helper methods for Redis queue implementation

func (c *ClientRedis) addQueueSubscriber(ctx context.Context, topic, id string, opts SubscribeOptions) error {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	qCtx, cancel := context.WithCancel(context.Background())
	commitTimeout := 30 * time.Second
	if c.config.Queue.CommitTimeout != nil {
		commitTimeout = *c.config.Queue.CommitTimeout
	}
	subscription := &redisQueueSubscription{
		topic:    topic,
		callback: opts.Callback,
		filter:   opts.Filter,
		ctx:      qCtx,
		cancel:   cancel,
		mx:       NewSendReceiveMultiplexer(ctx, topic, commitTimeout),
	}
	c.queueSubscribers[topic] = subscription

	// Register message processing
	go func() {
		c.doQueueReceive(ctx, subscription)
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id,
		}).Debug("Queue subscription created successfully")
	}

	return nil
}

func (c *ClientRedis) doQueueReceive(ctx context.Context, subscription *redisQueueSubscription) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithError(fmt.Errorf("%v", r)).Error("Recovered from panic in message processing")
		}
		subscription.cancel()
	}()

	for {
		select {
		case <-subscription.ctx.Done():
			return
		case <-ctx.Done():
			return
		default:
			event, err := c.pollQueue(ctx, subscription.topic)
			if err != nil {
				if ctx.Err() == nil {
					logrus.WithFields(logrus.Fields{
						"Component": "ClientRedis",
						"Topic":     subscription.topic,
						"Error":     err,
					}).Error("Failed to receive message")
					c.updateMetrics(subscription.topic, 0, 0, 1, 0, -1)
				}
				continue
			}

			if subscription.filter == nil || subscription.filter(ctx, event) {
				ackOnce := sync.Once{}
				nackOnce := sync.Once{}

				ack := func() {
					ackOnce.Do(func() {
						c.updateMetrics(subscription.topic, 0, 1, 0, 0, -1)
					})
				}

				nack := func() {
					nackOnce.Do(func() {
						c.updateMetrics(subscription.topic, 0, 0, 1, 0, -1)
					})
				}

				if err := subscription.callback(ctx, event, ack, nack); err != nil {
					c.updateMetrics(subscription.topic, 0, 0, 0, 1, -1)
					logrus.WithError(err).Error("failed to process message")
				}
			}
		}
	}
}

func (c *ClientRedis) addPubSubscriber(ctx context.Context, topic, id string, opts SubscribeOptions) error {
	c.consumerProducerLock.Lock()
	defer c.consumerProducerLock.Unlock()

	var sharedConn *redisPubSubConnection
	sharedConn = c.pubsubConnections[topic]

	if sharedConn == nil {
		sharedConn = &redisPubSubConnection{
			topic:     c.redisClient.Subscribe(ctx, topic),
			consumers: make(map[string]*pulsarSubscription),
			done:      make(chan bool),
		}
		c.pubsubConnections[topic] = sharedConn

		// Register pub/sub message processing
		go func() {
			c.doPubSubReceive(ctx, sharedConn, topic)
		}()
	}

	pCtx, cancel := context.WithCancel(context.Background())
	subscription := &pulsarSubscription{
		topic:    topic,
		callback: opts.Callback,
		filter:   opts.Filter,
		ctx:      pCtx,
		cancel:   cancel,
	}
	sharedConn.consumers[id] = subscription

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ClientRedis",
			"Topic":     topic,
			"ID":        id,
		}).Debug("Pub/Sub subscription created successfully")
	}

	return nil
}

func (c *ClientRedis) doPubSubReceive(ctx context.Context, conn *redisPubSubConnection, topic string) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithError(fmt.Errorf("%v", r)).Error("Recovered from panic in pub/sub processing")
		}
		_ = conn.topic.Close()
	}()

	maxWait := 30 * time.Second
	minWait := 10 * time.Millisecond
	curWait := minWait

	for {
		select {
		case msg := <-conn.topic.Channel():
			hp := toHeadersPayload([]byte(msg.Payload))
			event := &MessageEvent{
				Topic:       topic,
				Payload:     hp.Payload,
				Properties:  hp.Properties,
				ID:          nil, // Redis doesn't provide message IDs
				PublishTime: time.Now(),
			}

			c.consumerProducerLock.RLock()
			for _, subscription := range conn.consumers {
				if subscription.filter == nil || subscription.filter(ctx, event) {
					ackOnce := sync.Once{}
					nackOnce := sync.Once{}

					ack := func() {
						ackOnce.Do(func() {
							c.updateMetrics(topic, 0, 1, 0, 0, -1)
						})
					}

					nack := func() {
						nackOnce.Do(func() {
							c.updateMetrics(topic, 0, 0, 1, 0, -1)
						})
					}

					if err := subscription.callback(ctx, event, ack, nack); err != nil {
						c.updateMetrics(topic, 0, 0, 0, 1, -1)
						logrus.WithError(err).Error("Failed to process pub/sub message")
					}
				}
			}
			c.consumerProducerLock.RUnlock()
			curWait = minWait

		case <-conn.done:
			return
		case <-ctx.Done():
			return
		case <-time.After(math.MinDuration(maxWait, curWait)):
			// Check connection health
			if err := c.redisClient.Ping(ctx).Err(); err != nil {
				logrus.WithError(err).Error("Redis connection error")
				if curWait < maxWait {
					curWait *= 2
				}
			}
		}
	}
}

func (c *ClientRedis) pollQueue(ctx context.Context, topic string) (*MessageEvent, error) {
	handler := func(ctx context.Context, payload any) (bool, any, error) {
		res, err := c.redisClient.BLPop(ctx, c.maxWait, topic).Result()
		if err != nil || len(res) < 2 {
			return false, nil, nil
		}
		return true, []byte(res[1]), nil
	}
	minWait := 10 * time.Millisecond
	future := async.ExecutePollingWithSignal(
		ctx,
		handler,
		async.NoAbort,
		0,
		minWait,
		10*minWait)
	res, err := future.Await(ctx)
	if err != nil {
		return nil, err
	}
	hp := toHeadersPayload(res.([]byte))
	return &MessageEvent{
		Topic:       topic,
		Payload:     hp.Payload,
		Properties:  hp.Properties,
		ID:          []byte(hp.Properties[messageKey]), // Redis doesn't provide message IDs
		PublishTime: time.Now(),
	}, nil
}

type HeadersPayload struct {
	Properties map[string]string `json:"properties"`
	Payload    []byte            `json:"payload"`
}

func toHeadersPayloadData(props MessageHeaders, payload []byte) ([]byte, error) {
	hp := HeadersPayload{
		Properties: props,
		Payload:    payload,
	}
	return json.Marshal(hp)
}

func toHeadersPayload(data []byte) *HeadersPayload {
	hp := &HeadersPayload{
		Properties: make(map[string]string),
	}

	if err := json.Unmarshal(data, hp); err != nil {
		// If unmarshaling fails, treat the entire data as payload
		hp.Payload = data
	}
	return hp
}
