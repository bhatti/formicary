package queue

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

var _ Client = &ClientWebSocketAnt{}

// antLocalSub tracks a subscription registered on the ant side
type antLocalSub struct {
	id       string
	topic    string
	subID    string // server-assigned subscription ID (from subscribe_ack)
	callback Callback
	filter   Filter
	ctx      context.Context
	cancel   context.CancelFunc
}

// pendingAck awaits an ack/nack/subscribe_ack response for a specific frame ID
type pendingAck struct {
	ch chan *WSFrame
}

// ClientWebSocketAnt implements queue.Client for ant workers.
// It maintains a single persistent WebSocket connection to the queen, multiplexes
// all topics over that connection, and buffers messages offline when disconnected.
type ClientWebSocketAnt struct {
	config  *types.QueueConfig
	wsCfg   *types.WebSocketConfig
	metrics *MetricsCollector

	conn     *websocket.Conn
	connMu   sync.Mutex
	connOnce sync.Once // used to re-init after reconnect
	writeMu  sync.Mutex

	// local subscriptions: localSubID -> antLocalSub
	subs   map[string]*antLocalSub
	subsMu sync.RWMutex

	// server-assigned subID -> localSubID (reverse lookup for inbound delivery)
	subsByServerID   map[string]string
	subsByServerIDMu sync.RWMutex

	// pendingAcks: frameID -> pendingAck
	pending   map[string]*pendingAck
	pendingMu sync.Mutex

	buffer  *wsOfflineBuffer
	closeCh chan struct{}
	closed  bool
	closeMu sync.RWMutex

	// reconnect loop notification
	reconnectCh chan struct{}
}

func newWebSocketAntClient(ctx context.Context, config *types.QueueConfig) (*ClientWebSocketAnt, error) {
	wsCfg := config.WebSocket
	if wsCfg == nil {
		wsCfg = &types.WebSocketConfig{}
		wsCfg.Validate()
	}

	buf, err := newWSOfflineBuffer(wsCfg.BufferDBPath, wsCfg.MaxBufferSize, wsCfg.BufferTTL)
	if err != nil {
		return nil, fmt.Errorf("open offline buffer: %w", err)
	}

	c := &ClientWebSocketAnt{
		config:         config,
		wsCfg:          wsCfg,
		metrics:        newMetricsCollector(ctx),
		subs:           make(map[string]*antLocalSub),
		subsByServerID: make(map[string]string),
		pending:        make(map[string]*pendingAck),
		buffer:         buf,
		closeCh:        make(chan struct{}),
		reconnectCh:    make(chan struct{}, 1),
	}

	// Attempt initial connection
	if err := c.connect(); err != nil {
		logrus.WithError(err).Warn("ClientWebSocketAnt: initial connect failed; will retry in background")
	}

	go c.reconnectLoop(ctx)
	return c, nil
}

// isConnected returns true if there is an active WebSocket connection
func (c *ClientWebSocketAnt) isConnected() bool {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn != nil
}

// connect dials the queen's WebSocket endpoint
func (c *ClientWebSocketAnt) connect() error {
	header := http.Header{}
	if c.config.Token != "" {
		header.Set("Authorization", "Bearer "+c.config.Token)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: *c.config.ConnectionTimeout,
		ReadBufferSize:   c.wsCfg.ReadBufferSize,
		WriteBufferSize:  c.wsCfg.WriteBufferSize,
	}

	conn, _, err := dialer.Dial(c.wsCfg.ServerEndpoint, header)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.wsCfg.ServerEndpoint, err)
	}
	conn.SetReadLimit(c.wsCfg.MaxMessageSize)

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	logrus.WithField("Endpoint", c.wsCfg.ServerEndpoint).Info("ClientWebSocketAnt: connected")

	go c.readPump()
	go c.pingLoop()

	// Re-subscribe all existing local subscriptions
	c.subsMu.RLock()
	subs := make([]*antLocalSub, 0, len(c.subs))
	for _, sub := range c.subs {
		subs = append(subs, sub)
	}
	c.subsMu.RUnlock()

	for _, sub := range subs {
		if err := c.sendSubscribe(sub); err != nil {
			logrus.WithError(err).WithField("Topic", sub.topic).Warn("ClientWebSocketAnt: re-subscribe failed")
		}
	}

	// Drain offline buffer
	go c.drainBuffer()

	return nil
}

// reconnectLoop tries to reconnect after a disconnection
func (c *ClientWebSocketAnt) reconnectLoop(ctx context.Context) {
	delay := c.wsCfg.ReconnectMinDelay
	for {
		select {
		case <-c.closeCh:
			return
		case <-ctx.Done():
			return
		case <-c.reconnectCh:
			// connection dropped, start reconnecting
		}

		if c.isClosed() {
			return
		}

		for {
			if c.isClosed() {
				return
			}
			if c.isConnected() {
				delay = c.wsCfg.ReconnectMinDelay
				break
			}

			logrus.WithField("Delay", delay).Info("ClientWebSocketAnt: reconnecting...")
			if err := c.connect(); err != nil {
				logrus.WithError(err).Warn("ClientWebSocketAnt: reconnect failed")
				select {
				case <-time.After(delay):
				case <-c.closeCh:
					return
				case <-ctx.Done():
					return
				}
				delay = minDuration(time.Duration(float64(delay)*2), c.wsCfg.ReconnectMaxDelay)
				// add jitter: +/- 10% to spread reconnect storms
				jitter := time.Duration(float64(delay) * 0.1 * (rand.Float64() - 0.5)) //nolint:gosec
				delay = delay + jitter
				if delay < c.wsCfg.ReconnectMinDelay {
					delay = c.wsCfg.ReconnectMinDelay
				}
			} else {
				delay = c.wsCfg.ReconnectMinDelay
				break
			}
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// triggerReconnect signals the reconnect loop that the connection was lost
func (c *ClientWebSocketAnt) triggerReconnect() {
	c.connMu.Lock()
	c.conn = nil
	c.connMu.Unlock()

	// Fail all pending acks
	c.pendingMu.Lock()
	for id, p := range c.pending {
		select {
		case p.ch <- newErrorFrame(id, "connection lost"):
		default:
		}
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	select {
	case c.reconnectCh <- struct{}{}:
	default:
	}
}

// readPump reads frames from the WebSocket and dispatches them to subscribers or pending acks
func (c *ClientWebSocketAnt) readPump() {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !c.isClosed() {
				logrus.WithError(err).Debug("ClientWebSocketAnt: read error, triggering reconnect")
				c.triggerReconnect()
			}
			return
		}

		frame, err := UnmarshalWSFrame(data)
		if err != nil {
			logrus.WithError(err).Warn("ClientWebSocketAnt: bad frame")
			continue
		}

		c.dispatchFrame(frame)
	}
}

// dispatchFrame routes an inbound frame to the appropriate handler
func (c *ClientWebSocketAnt) dispatchFrame(frame *WSFrame) {
	switch frame.Type {
	case WSFramePublish:
		c.deliverToSubscribers(frame)

	case WSFrameSubscribeAck, WSFrameAck, WSFrameNack, WSFrameError:
		c.pendingMu.Lock()
		p, ok := c.pending[frame.ID]
		c.pendingMu.Unlock()
		if ok {
			select {
			case p.ch <- frame:
			default:
			}
		}

	case WSFramePing:
		pong := newPongFrame(frame.ID)
		data, _ := MarshalWSFrame(pong)
		c.writeRaw(data)

	case WSFramePong:
		// keepalive response

	default:
		logrus.WithField("Type", frame.Type).Debug("ClientWebSocketAnt: unknown frame type")
	}
}

// deliverToSubscribers routes an inbound publish frame to local subscriber callbacks
func (c *ClientWebSocketAnt) deliverToSubscribers(frame *WSFrame) {
	// Look up subscription by server-assigned sub ID if provided
	var subs []*antLocalSub

	if frame.SubscriptionID != "" {
		c.subsByServerIDMu.RLock()
		localID, ok := c.subsByServerID[frame.SubscriptionID]
		c.subsByServerIDMu.RUnlock()
		if ok {
			c.subsMu.RLock()
			sub := c.subs[localID]
			c.subsMu.RUnlock()
			if sub != nil {
				subs = []*antLocalSub{sub}
			}
		}
	}

	if len(subs) == 0 {
		// Fall back: deliver to all subs on the matching topic
		c.subsMu.RLock()
		for _, sub := range c.subs {
			if sub.topic == frame.Topic {
				subs = append(subs, sub)
			}
		}
		c.subsMu.RUnlock()
	}

	event := &MessageEvent{
		ID:          []byte(frame.ID),
		Topic:       frame.Topic,
		Payload:     frame.Payload,
		Properties:  frame.Properties,
		PublishTime: frame.Timestamp,
	}

	ackFrame := newAckFrame(frame.ID)
	nackFrame := newNackFrame(frame.ID)
	ackData, _ := MarshalWSFrame(ackFrame)
	nackData, _ := MarshalWSFrame(nackFrame)

	// sync.Once ensures only one ack/nack is sent even when multiple subscribers match.
	var once sync.Once
	ack := func() { once.Do(func() { _ = c.writeRaw(ackData) }) }
	nack := func() { once.Do(func() { _ = c.writeRaw(nackData) }) }

	for _, sub := range subs {
		if sub.filter != nil && !sub.filter(sub.ctx, event) {
			continue
		}
		if err := sub.callback(sub.ctx, event, ack, nack); err != nil {
			logrus.WithError(err).WithField("Topic", frame.Topic).Debug("ClientWebSocketAnt: callback error")
		}
	}
	c.metrics.updateMetrics(frame.Topic, 0, 1, 0, 0, 0)
}

// pingLoop sends periodic ping frames
func (c *ClientWebSocketAnt) pingLoop() {
	ticker := time.NewTicker(c.wsCfg.PingInterval)
	defer ticker.Stop()

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return
	}

	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			if !c.isConnected() {
				return
			}
			ping := newPingFrame(ulid.Make().String())
			data, _ := MarshalWSFrame(ping)
			if err := c.writeRaw(data); err != nil {
				return
			}
		}
	}
}

// writeRaw writes raw bytes to the WebSocket connection, with write deadline.
// writeMu ensures only one goroutine writes at a time (gorilla/websocket single-writer requirement).
func (c *ClientWebSocketAnt) writeRaw(data []byte) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(c.wsCfg.WriteTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.triggerReconnect()
		return err
	}
	return nil
}

// sendFrameAwaitAck sends a frame and waits for the corresponding ack/nack/error
func (c *ClientWebSocketAnt) sendFrameAwaitAck(frame *WSFrame, timeout time.Duration) (*WSFrame, error) {
	data, err := MarshalWSFrame(frame)
	if err != nil {
		return nil, err
	}

	ch := make(chan *WSFrame, 1)
	c.pendingMu.Lock()
	c.pending[frame.ID] = &pendingAck{ch: ch}
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, frame.ID)
		c.pendingMu.Unlock()
	}()

	if err := c.writeRaw(data); err != nil {
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp := <-ch:
		return resp, nil
	case <-timer.C:
		return nil, fmt.Errorf("timeout awaiting ack for frame %s", frame.ID)
	}
}

// sendSubscribe sends a subscribe frame to the server and records the server-assigned subID
func (c *ClientWebSocketAnt) sendSubscribe(sub *antLocalSub) error {
	frame := newSubscribeFrame(ulid.Make().String(), sub.topic, "", false)
	resp, err := c.sendFrameAwaitAck(frame, *c.config.OperationTimeout)
	if err != nil {
		return err
	}
	if resp.Type == WSFrameError {
		return fmt.Errorf("subscribe error: %s", resp.Error)
	}
	if resp.SubscriptionID != "" {
		c.subsByServerIDMu.Lock()
		c.subsByServerID[resp.SubscriptionID] = sub.id
		c.subsByServerIDMu.Unlock()
	}
	return nil
}

// drainBuffer publishes all buffered offline messages after reconnection.
// Loops until the buffer is empty to catch messages enqueued concurrently during the drain.
func (c *ClientWebSocketAnt) drainBuffer() {
	total := 0
	for {
		msgs, err := c.buffer.DequeueAll()
		if err != nil {
			logrus.WithError(err).Warn("ClientWebSocketAnt: drain buffer query failed")
			return
		}
		if len(msgs) == 0 {
			break
		}
		for _, m := range msgs {
			if !c.isConnected() {
				return
			}
			frame := newPublishFrame(ulid.Make().String(), m.Topic, m.Payload, m.Properties)
			_, err := c.sendFrameAwaitAck(frame, *c.config.OperationTimeout)
			if err != nil {
				logrus.WithError(err).WithField("Topic", m.Topic).Warn("ClientWebSocketAnt: buffer drain send failed")
				return
			}
			if removeErr := c.buffer.Remove(m.ID); removeErr != nil {
				logrus.WithError(removeErr).Warn("ClientWebSocketAnt: buffer remove failed")
			}
		}
		total += len(msgs)
	}
	if total > 0 {
		logrus.WithField("Count", total).Info("ClientWebSocketAnt: drained offline buffer")
	}
}

// ---- queue.Client interface ----

// Subscribe registers a local subscriber and informs the server.
func (c *ClientWebSocketAnt) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
	if err := validateSubscribeOptions(&opts); err != nil {
		return "", err
	}

	subCtx, cancel := context.WithCancel(ctx)
	subID := ulid.Make().String()

	sub := &antLocalSub{
		id:       subID,
		topic:    opts.Topic,
		callback: opts.Callback,
		filter:   opts.Filter,
		ctx:      subCtx,
		cancel:   cancel,
	}

	c.subsMu.Lock()
	c.subs[subID] = sub
	c.subsMu.Unlock()

	c.metrics.setTopic(opts.Topic, true)

	// If connected, register with server
	if c.isConnected() {
		if err := c.sendSubscribe(sub); err != nil {
			logrus.WithError(err).WithField("Topic", opts.Topic).Warn("ClientWebSocketAnt: subscribe frame failed")
		}
	}

	return subID, nil
}

// UnSubscribe removes a local subscriber and informs the server.
func (c *ClientWebSocketAnt) UnSubscribe(_ context.Context, topic string, id string) error {
	c.subsMu.Lock()
	sub, ok := c.subs[id]
	if !ok {
		c.subsMu.Unlock()
		return nil
	}
	sub.cancel()
	delete(c.subs, id)
	c.subsMu.Unlock()

	// Remove reverse mapping
	if sub.subID != "" {
		c.subsByServerIDMu.Lock()
		delete(c.subsByServerID, sub.subID)
		c.subsByServerIDMu.Unlock()

		if c.isConnected() {
			frame := newUnsubscribeFrame(ulid.Make().String(), topic, sub.subID)
			data, _ := MarshalWSFrame(frame)
			_ = c.writeRaw(data)
		}
	}
	return nil
}

// Send publishes a message to the queen. Buffers offline if disconnected.
func (c *ClientWebSocketAnt) Send(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := validateSendRequest(topic, payload, props, c.config); err != nil {
		return nil, err
	}

	msgID := ulid.Make().String()

	if !c.isConnected() {
		// Buffer the message for later delivery
		if err := c.buffer.Enqueue(topic, payload, props); err != nil {
			return nil, fmt.Errorf("offline buffer: %w", err)
		}
		return []byte(msgID), nil
	}

	frame := newPublishFrame(msgID, topic, payload, props)
	resp, err := c.sendFrameAwaitAck(frame, *c.config.OperationTimeout)
	if err != nil {
		// Try buffering
		if bufErr := c.buffer.Enqueue(topic, payload, props); bufErr != nil {
			return nil, fmt.Errorf("send failed and buffer failed: send=%w buffer=%v", err, bufErr)
		}
		return []byte(msgID), nil
	}
	if resp.Type == WSFrameError {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}

	c.metrics.updateMetrics(topic, 1, 0, 0, 0, 0)
	return []byte(msgID), nil
}

// Publish is an alias for Send.
func (c *ClientWebSocketAnt) Publish(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	return c.Send(ctx, topic, payload, props)
}

// SendReceive publishes to OutTopic and awaits a correlated reply on InTopic.
func (c *ClientWebSocketAnt) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	if err := validateSendReceiveRequest(req, c.config); err != nil {
		return nil, err
	}

	responseCh := make(chan *MessageEvent, 1)
	correlationID := ulid.Make().String()

	subOpts := SubscribeOptions{
		Topic: req.InTopic,
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			if msg.CoRelationID() == correlationID {
				select {
				case responseCh <- msg:
					ack()
				default:
					nack()
				}
			}
			return nil
		},
	}
	subID, err := c.Subscribe(ctx, subOpts)
	if err != nil {
		return nil, fmt.Errorf("SendReceive subscribe: %w", err)
	}
	defer func() {
		_ = c.UnSubscribe(ctx, req.InTopic, subID)
		close(responseCh)
	}()

	props := make(MessageHeaders)
	for k, v := range req.Props {
		props[k] = v
	}
	props.SetCorrelationID(correlationID)
	props.SetReplyTopic(req.InTopic)

	if _, err := c.Send(ctx, req.OutTopic, req.Payload, props); err != nil {
		return nil, fmt.Errorf("SendReceive send: %w", err)
	}

	timeoutCtx, cancel := createTimeoutContext(ctx, req.Timeout)
	defer cancel()

	select {
	case msg := <-responseCh:
		if msg == nil {
			return nil, fmt.Errorf("SendReceive: nil response")
		}
		return &SendReceiveResponse{Event: msg, Ack: func() {}, Nack: func() {}}, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("SendReceive timeout on topic %s", req.OutTopic)
	}
}

// GetMetrics returns metrics for a topic.
func (c *ClientWebSocketAnt) GetMetrics(ctx context.Context, topic string) (*QueueMetrics, error) {
	return c.metrics.GetMetrics(ctx, topic)
}

// CreateTopicIfNotExists is a no-op for the ant client.
func (c *ClientWebSocketAnt) CreateTopicIfNotExists(_ context.Context, _ string, _ *TopicConfig) error {
	return nil
}

// Close shuts down the ant client.
func (c *ClientWebSocketAnt) Close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	c.closeMu.Unlock()

	close(c.closeCh)

	c.connMu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	if c.buffer != nil {
		_ = c.buffer.Close()
	}
}

func (c *ClientWebSocketAnt) isClosed() bool {
	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	return c.closed
}
