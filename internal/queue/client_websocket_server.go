package queue

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

var _ Client = &ClientWebSocketServer{}

// wsServerConn represents a single WebSocket connection from an ant worker
type wsServerConn struct {
	id      string
	conn    *websocket.Conn
	sendCh  chan []byte
	closeCh chan struct{}
	once    sync.Once
	// topic -> []subscriptionID registered on this connection
	subs   map[string][]string
	subsMu sync.RWMutex
}

func newWSServerConn(id string, conn *websocket.Conn) *wsServerConn {
	return &wsServerConn{
		id:      id,
		conn:    conn,
		sendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
		subs:    make(map[string][]string),
	}
}

func (c *wsServerConn) addSub(topic, subID string) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	c.subs[topic] = append(c.subs[topic], subID)
}

func (c *wsServerConn) removeSub(topic, subID string) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	ids := c.subs[topic]
	for i, id := range ids {
		if id == subID {
			c.subs[topic] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(c.subs[topic]) == 0 {
		delete(c.subs, topic)
	}
}

// send enqueues a frame for writing; drops if full to avoid blocking
func (c *wsServerConn) send(data []byte) bool {
	select {
	case c.sendCh <- data:
		return true
	default:
		return false
	}
}

func (c *wsServerConn) close() {
	c.once.Do(func() {
		close(c.closeCh)
		_ = c.conn.Close()
	})
}

// wsRemoteSub tracks a subscription registered by a remote ant connection
type wsRemoteSub struct {
	id    string
	conn  *wsServerConn
	topic string
	group string
}

// wsLocalSub tracks a subscription registered by the local queen process
type wsLocalSub struct {
	id       string
	topic    string
	callback Callback
	filter   Filter
	ctx      context.Context
	cancel   context.CancelFunc
}

// wsTopic holds all local and remote subscriptions for a topic
type wsTopic struct {
	name       string
	localSubs  map[string]*wsLocalSub
	remoteSubs map[string]*wsRemoteSub // subID -> remoteSub
	mx         *SendReceiveMultiplexer
}

// ClientWebSocketServer implements queue.Client for the queen side.
// It listens for incoming WebSocket connections from ant workers, routes published
// messages to both local (queen-process) and remote (ant) subscribers.
type ClientWebSocketServer struct {
	config   *types.QueueConfig
	wsCfg    *types.WebSocketConfig
	metrics  *MetricsCollector
	upgrader websocket.Upgrader

	// topics: topic name -> wsTopic
	topics   map[string]*wsTopic
	topicsMu sync.RWMutex

	// conns: connection id -> wsServerConn
	conns   map[string]*wsServerConn
	connsMu sync.RWMutex

	closed   bool
	closedMu sync.RWMutex
}

func newWebSocketServerClient(ctx context.Context, config *types.QueueConfig) (*ClientWebSocketServer, error) {
	wsCfg := config.WebSocket
	if wsCfg == nil {
		wsCfg = &types.WebSocketConfig{}
		wsCfg.Validate()
	}

	s := &ClientWebSocketServer{
		config:  config,
		wsCfg:   wsCfg,
		metrics: newMetricsCollector(ctx),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  wsCfg.ReadBufferSize,
			WriteBufferSize: wsCfg.WriteBufferSize,
			CheckOrigin:     func(_ *http.Request) bool { return true },
		},
		topics: make(map[string]*wsTopic),
		conns:  make(map[string]*wsServerConn),
	}
	return s, nil
}

// HTTPHandlerProvider is implemented by ClientWebSocketServer to expose its WebSocket endpoint
// so the queen's HTTP server can mount it.
type HTTPHandlerProvider interface {
	HTTPHandler() http.HandlerFunc
	WebSocketPath() string
}

// WebSocketPath returns the URL path at which the WebSocket endpoint is served.
func (s *ClientWebSocketServer) WebSocketPath() string {
	return s.wsCfg.Path
}

// HTTPHandler returns an http.HandlerFunc that upgrades connections and registers ant workers.
// Mount this on the queen's web server at the configured path (default: /ws/queue).
func (s *ClientWebSocketServer) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Token != "" {
			if r.Header.Get("Authorization") != "Bearer "+s.config.Token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.WithError(err).Warn("WebSocketServer: upgrade failed")
			return
		}
		id := ulid.Make().String()
		sc := newWSServerConn(id, conn)

		s.connsMu.Lock()
		s.conns[id] = sc
		s.connsMu.Unlock()

		logrus.WithField("ConnID", id).Info("WebSocketServer: ant connected")

		go s.writePump(sc)
		go s.readPump(r.Context(), sc)
	}
}

// Subscribe registers a local (queen-process) subscriber on the given topic.
func (s *ClientWebSocketServer) Subscribe(ctx context.Context, opts SubscribeOptions) (string, error) {
	if err := validateSubscribeOptions(&opts); err != nil {
		return "", err
	}

	subID := ulid.Make().String()
	subCtx, cancel := context.WithCancel(ctx)

	sub := &wsLocalSub{
		id:       subID,
		topic:    opts.Topic,
		callback: opts.Callback,
		filter:   opts.Filter,
		ctx:      subCtx,
		cancel:   cancel,
	}

	t := s.getOrCreateTopic(ctx, opts.Topic)
	t.mx.Add(subCtx, subID, opts.Props.GetCorrelationID(), opts.Callback, opts.Filter, nil) //nolint:errcheck

	s.topicsMu.Lock()
	t.localSubs[subID] = sub
	s.topicsMu.Unlock()

	s.metrics.setTopic(opts.Topic, true)
	return subID, nil
}

// UnSubscribe removes a local subscriber.
func (s *ClientWebSocketServer) UnSubscribe(_ context.Context, topic string, id string) error {
	s.topicsMu.Lock()
	defer s.topicsMu.Unlock()

	t, ok := s.topics[topic]
	if !ok {
		return nil
	}
	sub, ok := t.localSubs[id]
	if !ok {
		// might be a remote sub - ignore
		return nil
	}
	sub.cancel()
	t.mx.Remove(id)
	delete(t.localSubs, id)
	s.maybeDeleteTopic(topic)
	return nil
}

// Send publishes a message to all subscribers (local + remote) of the topic.
func (s *ClientWebSocketServer) Send(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := validateSendRequest(topic, payload, props, s.config); err != nil {
		return nil, err
	}

	msgID := []byte(ulid.Make().String())
	event := &MessageEvent{
		ID:          msgID,
		Topic:       topic,
		Payload:     payload,
		Properties:  props,
		PublishTime: time.Now(),
	}

	s.metrics.updateMetrics(topic, 1, 0, 0, 0, 0)

	ack := func() { s.metrics.updateMetrics(topic, 0, 1, 0, 0, 0) }
	nack := func() { s.metrics.updateMetrics(topic, 0, 0, 1, 0, 0) }

	t := s.getOrCreateTopic(ctx, topic)

	// Notify local subscribers
	sent := t.mx.Notify(ctx, event, ack, nack)

	// Deliver to remote ant subscribers
	s.topicsMu.RLock()
	remotes := make([]*wsRemoteSub, 0, len(t.remoteSubs))
	for _, rs := range t.remoteSubs {
		remotes = append(remotes, rs)
	}
	s.topicsMu.RUnlock()

	frame := newPublishFrame(string(msgID), topic, payload, props)
	data, err := MarshalWSFrame(frame)
	if err == nil {
		for _, rs := range remotes {
			if !rs.conn.send(data) {
				logrus.WithFields(logrus.Fields{
					"ConnID": rs.conn.id,
					"Topic":  topic,
				}).Warn("WebSocketServer: send channel full, dropping message to remote subscriber")
			} else {
				sent++
			}
		}
	}

	if sent == 0 {
		ack()
	}
	return msgID, nil
}

// Publish is an alias for Send.
func (s *ClientWebSocketServer) Publish(ctx context.Context, topic string, payload []byte, props MessageHeaders) ([]byte, error) {
	return s.Send(ctx, topic, payload, props)
}

// SendReceive publishes to OutTopic and waits for a correlated response on InTopic.
func (s *ClientWebSocketServer) SendReceive(ctx context.Context, req *SendReceiveRequest) (*SendReceiveResponse, error) {
	if err := validateSendReceiveRequest(req, s.config); err != nil {
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
	subID, err := s.Subscribe(ctx, subOpts)
	if err != nil {
		return nil, fmt.Errorf("SendReceive subscribe: %w", err)
	}
	defer func() {
		_ = s.UnSubscribe(ctx, req.InTopic, subID)
		close(responseCh)
	}()

	props := make(MessageHeaders)
	for k, v := range req.Props {
		props[k] = v
	}
	props.SetCorrelationID(correlationID)
	props.SetReplyTopic(req.InTopic)

	if _, err := s.Send(ctx, req.OutTopic, req.Payload, props); err != nil {
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
func (s *ClientWebSocketServer) GetMetrics(ctx context.Context, topic string) (*QueueMetrics, error) {
	return s.metrics.GetMetrics(ctx, topic)
}

// CreateTopicIfNotExists is a no-op for the WebSocket provider.
func (s *ClientWebSocketServer) CreateTopicIfNotExists(ctx context.Context, topic string, _ *TopicConfig) error {
	s.getOrCreateTopic(ctx, topic)
	return nil
}

// Close shuts down the server and all connections.
func (s *ClientWebSocketServer) Close() {
	s.closedMu.Lock()
	if s.closed {
		s.closedMu.Unlock()
		return
	}
	s.closed = true
	s.closedMu.Unlock()

	s.connsMu.RLock()
	conns := make([]*wsServerConn, 0, len(s.conns))
	for _, c := range s.conns {
		conns = append(conns, c)
	}
	s.connsMu.RUnlock()

	for _, c := range conns {
		c.close()
	}
}

// maybeDeleteTopic removes a topic from the map if it has no remaining subscribers.
// Must be called under topicsMu write lock.
func (s *ClientWebSocketServer) maybeDeleteTopic(name string) {
	t, ok := s.topics[name]
	if !ok {
		return
	}
	if len(t.localSubs) == 0 && len(t.remoteSubs) == 0 {
		delete(s.topics, name)
	}
}

// getOrCreateTopic returns or creates a topic struct (under write lock if creating)
func (s *ClientWebSocketServer) getOrCreateTopic(ctx context.Context, name string) *wsTopic {
	s.topicsMu.Lock()
	defer s.topicsMu.Unlock()
	t, ok := s.topics[name]
	if !ok {
		t = &wsTopic{
			name:       name,
			localSubs:  make(map[string]*wsLocalSub),
			remoteSubs: make(map[string]*wsRemoteSub),
			mx:         NewSendReceiveMultiplexer(ctx, name, 30*time.Second),
		}
		s.topics[name] = t
	}
	return t
}

// writePump drains the connection's send channel and writes frames to the WebSocket.
func (s *ClientWebSocketServer) writePump(sc *wsServerConn) {
	ticker := time.NewTicker(s.wsCfg.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-sc.closeCh:
			return
		case data, ok := <-sc.sendCh:
			if !ok {
				return
			}
			_ = sc.conn.SetWriteDeadline(time.Now().Add(s.wsCfg.WriteTimeout))
			if err := sc.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logrus.WithField("ConnID", sc.id).WithError(err).Debug("WebSocketServer: write error")
				s.removeConn(sc)
				return
			}
		case <-ticker.C:
			pingFrame := newPingFrame(ulid.Make().String())
			data, _ := MarshalWSFrame(pingFrame)
			_ = sc.conn.SetWriteDeadline(time.Now().Add(s.wsCfg.WriteTimeout))
			if err := sc.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				s.removeConn(sc)
				return
			}
		}
	}
}

// readPump reads frames from the WebSocket and dispatches them.
func (s *ClientWebSocketServer) readPump(ctx context.Context, sc *wsServerConn) {
	defer s.removeConn(sc)

	sc.conn.SetReadLimit(s.wsCfg.MaxMessageSize)
	// Initial deadline: must receive something within 3 ping intervals.
	_ = sc.conn.SetReadDeadline(time.Now().Add(s.wsCfg.PingInterval * 3))

	for {
		_, data, err := sc.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logrus.WithField("ConnID", sc.id).WithError(err).Debug("WebSocketServer: read error")
			}
			return
		}
		// Reset deadline on every received frame (ping, pong, or data).
		_ = sc.conn.SetReadDeadline(time.Now().Add(s.wsCfg.PingInterval * 3))

		frame, err := UnmarshalWSFrame(data)
		if err != nil {
			logrus.WithField("ConnID", sc.id).WithError(err).Warn("WebSocketServer: bad frame")
			continue
		}

		s.handleFrame(ctx, sc, frame)
	}
}

// handleFrame dispatches a received frame from an ant connection
func (s *ClientWebSocketServer) handleFrame(ctx context.Context, sc *wsServerConn, frame *WSFrame) {
	switch frame.Type {
	case WSFrameSubscribe:
		s.handleSubscribeFrame(ctx, sc, frame)

	case WSFrameUnsubscribe:
		s.handleUnsubscribeFrame(sc, frame)

	case WSFramePublish:
		// Ant is publishing a message (e.g., registration heartbeat, task result)
		s.handlePublishFromAnt(ctx, sc, frame)

	case WSFrameAck, WSFrameNack:
		// Delivery acknowledgements from ant — not currently tracked on server side
		// but could be extended for at-least-once delivery guarantees

	case WSFramePing:
		pong := newPongFrame(frame.ID)
		if data, err := MarshalWSFrame(pong); err == nil {
			sc.send(data)
		}

	case WSFramePong:
		// keepalive response, no action needed

	default:
		logrus.WithFields(logrus.Fields{
			"ConnID": sc.id,
			"Type":   frame.Type,
		}).Debug("WebSocketServer: unknown frame type")
	}
}

func (s *ClientWebSocketServer) handleSubscribeFrame(ctx context.Context, sc *wsServerConn, frame *WSFrame) {
	subID := ulid.Make().String()

	rs := &wsRemoteSub{
		id:    subID,
		conn:  sc,
		topic: frame.Topic,
		group: frame.Group,
	}

	t := s.getOrCreateTopic(ctx, frame.Topic)

	s.topicsMu.Lock()
	t.remoteSubs[subID] = rs
	s.topicsMu.Unlock()

	sc.addSub(frame.Topic, subID)

	ack := &WSFrame{
		Type:           WSFrameSubscribeAck,
		ID:             frame.ID,
		SubscriptionID: subID,
		Timestamp:      time.Now(),
	}
	if data, err := MarshalWSFrame(ack); err == nil {
		sc.send(data)
	}

	logrus.WithFields(logrus.Fields{
		"ConnID": sc.id,
		"Topic":  frame.Topic,
		"SubID":  subID,
	}).Debug("WebSocketServer: remote subscription registered")
}

func (s *ClientWebSocketServer) handleUnsubscribeFrame(sc *wsServerConn, frame *WSFrame) {
	subID := frame.SubscriptionID
	topic := frame.Topic

	s.topicsMu.Lock()
	t, ok := s.topics[topic]
	if ok {
		delete(t.remoteSubs, subID)
	}
	s.topicsMu.Unlock()

	sc.removeSub(topic, subID)

	ackFrame := newAckFrame(frame.ID)
	if data, err := MarshalWSFrame(ackFrame); err == nil {
		sc.send(data)
	}
}

func (s *ClientWebSocketServer) handlePublishFromAnt(ctx context.Context, sc *wsServerConn, frame *WSFrame) {
	// Deliver to local subscribers on this topic (queen-side consumers)
	ack := func() {}
	nack := func() {}

	event := &MessageEvent{
		ID:          []byte(frame.ID),
		Topic:       frame.Topic,
		Payload:     frame.Payload,
		Properties:  frame.Properties,
		PublishTime: frame.Timestamp,
	}

	s.metrics.updateMetrics(frame.Topic, 1, 0, 0, 0, 0)

	t := s.getOrCreateTopic(ctx, frame.Topic)
	t.mx.Notify(ctx, event, ack, nack)

	// Send ack back to ant
	ackFrame := newAckFrame(frame.ID)
	if data, err := MarshalWSFrame(ackFrame); err == nil {
		sc.send(data)
	}
}

// removeConn removes a connection and cleans up its subscriptions
func (s *ClientWebSocketServer) removeConn(sc *wsServerConn) {
	sc.close()

	s.connsMu.Lock()
	delete(s.conns, sc.id)
	s.connsMu.Unlock()

	// Remove all remote subscriptions belonging to this connection
	sc.subsMu.RLock()
	topicSubs := make(map[string][]string)
	for topic, ids := range sc.subs {
		topicSubs[topic] = append([]string{}, ids...)
	}
	sc.subsMu.RUnlock()

	s.topicsMu.Lock()
	for topic, ids := range topicSubs {
		t, ok := s.topics[topic]
		if !ok {
			continue
		}
		for _, id := range ids {
			delete(t.remoteSubs, id)
		}
		s.maybeDeleteTopic(topic)
	}
	s.topicsMu.Unlock()

	logrus.WithField("ConnID", sc.id).Info("WebSocketServer: ant disconnected")
}
