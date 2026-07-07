package queue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// buildTestQueueConfig returns a QueueConfig wired for WebSocket with defaults
func buildTestQueueConfig() *types.QueueConfig {
	cfg := &types.QueueConfig{
		Provider:      types.WebSocketMessagingProvider,
		RetryMax:      3,
		MaxMessageSize: 1024 * 1024,
		MaxConnections: 256,
	}
	connTimeout := 5 * time.Second
	opTimeout := 5 * time.Second
	cfg.ConnectionTimeout = &connTimeout
	cfg.OperationTimeout = &opTimeout
	cfg.WebSocket = &types.WebSocketConfig{}
	cfg.WebSocket.Validate()
	return cfg
}

// startTestServer creates a queen-side server and an httptest.Server,
// returning the server client and a function that creates a connected ant client.
func startTestServer(t *testing.T) (*ClientWebSocketServer, func() *ClientWebSocketAnt) {
	t.Helper()
	ctx := context.Background()
	cfg := buildTestQueueConfig()

	serverClient, err := newWebSocketServerClient(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(serverClient.Close)

	ts := httptest.NewServer(serverClient.HTTPHandler())
	t.Cleanup(ts.Close)

	makeAnt := func() *ClientWebSocketAnt {
		antCfg := buildTestQueueConfig()
		// Convert http:// to ws://
		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + antCfg.WebSocket.Path
		antCfg.WebSocket.ServerEndpoint = wsURL
		antCfg.WebSocket.BufferDBPath = ":memory:"

		ant, err := newWebSocketAntClient(ctx, antCfg)
		require.NoError(t, err)
		t.Cleanup(ant.Close)

		// Wait until the ant is registered on the server side (no arbitrary sleep).
		require.Eventually(t, func() bool {
			return serverClient.ConnectedAntCount() > 0
		}, 2*time.Second, 5*time.Millisecond, "ant did not connect within timeout")
		return ant
	}

	return serverClient, makeAnt
}

// TestWebSocket_ServerSubscribeAntPublish: queen subscribes, ant publishes
func TestWebSocket_ServerSubscribeAntPublish(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	received := make(chan *MessageEvent, 1)
	_, err := serverClient.Subscribe(ctx, SubscribeOptions{
		Topic: "formicary-queue-registration",
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			received <- msg
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	payload := []byte(`{"ant_id":"test-ant-1"}`)
	_, err = ant.Send(ctx, "formicary-queue-registration", payload, nil)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, "formicary-queue-registration", msg.Topic)
		assert.Equal(t, payload, msg.Payload)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: server did not receive message from ant")
	}
}

// TestWebSocket_AntSubscribeServerPublish: ant subscribes, queen publishes (task dispatch)
func TestWebSocket_AntSubscribeServerPublish(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	received := make(chan *MessageEvent, 1)
	_, err := ant.Subscribe(ctx, SubscribeOptions{
		Topic: "formicary-queue-ant-request",
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			received <- msg
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	// Allow subscribe frame to propagate
	time.Sleep(100 * time.Millisecond)

	payload := []byte(`{"task_id":"task-42"}`)
	_, err = serverClient.Send(ctx, "formicary-queue-ant-request", payload, nil)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, "formicary-queue-ant-request", msg.Topic)
		assert.Equal(t, payload, msg.Payload)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: ant did not receive task from server")
	}
}

// TestWebSocket_SendReceive: server sends to ant, ant replies (task dispatch pattern)
func TestWebSocket_SendReceive(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	outTopic := "formicary-queue-ant-request"
	inTopic := "formicary-queue-ant-reply"

	// Ant subscribes to request topic and replies
	_, err := ant.Subscribe(ctx, SubscribeOptions{
		Topic: outTopic,
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			ack()
			// Echo back with same correlation ID
			respProps := make(MessageHeaders)
			respProps.SetCorrelationID(msg.CoRelationID())
			_, sendErr := ant.Send(ctx, inTopic, []byte(`{"status":"done"}`), respProps)
			return sendErr
		},
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Server sends and waits for response
	resp, err := serverClient.SendReceive(ctx, &SendReceiveRequest{
		OutTopic: outTopic,
		InTopic:  inTopic,
		Payload:  []byte(`{"action":"execute"}`),
		Timeout:  3 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, `{"status":"done"}`, string(resp.Event.Payload))
}

// TestWebSocket_MultipleAnts: multiple ants subscribe to same topic
func TestWebSocket_MultipleAnts(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant1 := makeAnt()
	ant2 := makeAnt()
	ctx := context.Background()

	const topic = "formicary-queue-broadcast"
	var mu sync.Mutex
	received := make([]string, 0)

	for i, ant := range []*ClientWebSocketAnt{ant1, ant2} {
		antNum := i + 1
		_, err := ant.Subscribe(ctx, SubscribeOptions{
			Topic: topic,
			Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
				mu.Lock()
				received = append(received, string(msg.Payload)+"-ant"+func() string {
					if antNum == 1 {
						return "1"
					}
					return "2"
				}())
				mu.Unlock()
				ack()
				return nil
			},
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err := serverClient.Send(ctx, topic, []byte("hello"), nil)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Both ants should have received the message
	assert.Len(t, received, 2)
}

// TestWebSocket_AntPublish_MessageHeaders: headers are preserved end-to-end
func TestWebSocket_AntPublish_MessageHeaders(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	received := make(chan *MessageEvent, 1)
	_, err := serverClient.Subscribe(ctx, SubscribeOptions{
		Topic: "formicary-queue-registration",
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			received <- msg
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	props := NewMessageHeaders(CorrelationIDKey, "my-corr", replyTopicKey, "formicary-queue-ant-reply")
	_, err = ant.Send(ctx, "formicary-queue-registration", []byte("body"), props)
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, "my-corr", msg.Properties.GetCorrelationID())
		assert.Equal(t, "formicary-queue-ant-reply", msg.Properties.GetReplyTopic())
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message with headers")
	}
}

// TestWebSocket_Unsubscribe: ant unsubscribes and no longer receives messages
func TestWebSocket_Unsubscribe(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	const topic = "formicary-queue-ant-request"
	receivedCount := 0
	var mu sync.Mutex

	subID, err := ant.Subscribe(ctx, SubscribeOptions{
		Topic: topic,
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			mu.Lock()
			receivedCount++
			mu.Unlock()
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Send one message - should be received
	_, err = serverClient.Send(ctx, topic, []byte("msg-1"), nil)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Unsubscribe
	require.NoError(t, ant.UnSubscribe(ctx, topic, subID))
	time.Sleep(50 * time.Millisecond)

	// Send another - should NOT be received
	_, err = serverClient.Send(ctx, topic, []byte("msg-2"), nil)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, receivedCount)
}

// TestWebSocket_ServerClose: server closes and client handles it gracefully
func TestWebSocket_ServerClose(t *testing.T) {
	ctx := context.Background()
	cfg := buildTestQueueConfig()

	serverClient, err := newWebSocketServerClient(ctx, cfg)
	require.NoError(t, err)

	ts := httptest.NewServer(serverClient.HTTPHandler())

	antCfg := buildTestQueueConfig()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + antCfg.WebSocket.Path
	antCfg.WebSocket.ServerEndpoint = wsURL
	antCfg.WebSocket.BufferDBPath = ":memory:"
	antCfg.WebSocket.ReconnectMinDelay = 10 * time.Millisecond
	antCfg.WebSocket.ReconnectMaxDelay = 50 * time.Millisecond

	ant, err := newWebSocketAntClient(ctx, antCfg)
	require.NoError(t, err)
	defer ant.Close()

	time.Sleep(50 * time.Millisecond)
	assert.True(t, ant.isConnected())

	// Close server
	ts.Close()
	serverClient.Close()

	// Wait for ant to detect disconnect
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !ant.isConnected() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	// After server close, ant should not be connected
	assert.False(t, ant.isConnected())
}

// TestWebSocket_GetMetrics: metrics track published messages
func TestWebSocket_GetMetrics(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	const topic = "formicary-queue-ant-request"
	_, err := ant.Subscribe(ctx, SubscribeOptions{
		Topic: topic,
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		_, err := serverClient.Send(ctx, topic, []byte("payload"), nil)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	m, err := serverClient.GetMetrics(ctx, topic)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, m.MessagesPublished, int64(3))
}

// TestWebSocket_CreateTopicIfNotExists: no-op for WebSocket
func TestWebSocket_CreateTopicIfNotExists(t *testing.T) {
	serverClient, _ := startTestServer(t)
	ctx := context.Background()

	err := serverClient.CreateTopicIfNotExists(ctx, "formicary-queue-some-topic", nil)
	assert.NoError(t, err)
}

// TestWebSocket_AntOfflineBuffer: messages buffered when ant disconnected, drained on reconnect
func TestWebSocket_AntOfflineBuffer(t *testing.T) {
	ctx := context.Background()
	cfg := buildTestQueueConfig()

	// Start server
	serverClient, err := newWebSocketServerClient(ctx, cfg)
	require.NoError(t, err)
	defer serverClient.Close()

	ts := httptest.NewServer(serverClient.HTTPHandler())
	defer ts.Close()

	// Create ant with short reconnect delay
	antCfg := buildTestQueueConfig()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + antCfg.WebSocket.Path
	antCfg.WebSocket.ServerEndpoint = wsURL
	antCfg.WebSocket.BufferDBPath = ":memory:"
	antCfg.WebSocket.ReconnectMinDelay = 20 * time.Millisecond
	antCfg.WebSocket.ReconnectMaxDelay = 100 * time.Millisecond

	ant, err := newWebSocketAntClient(ctx, antCfg)
	require.NoError(t, err)
	defer ant.Close()

	time.Sleep(50 * time.Millisecond)
	require.True(t, ant.isConnected())

	// Disconnect by closing the connection forcibly
	ant.connMu.Lock()
	if ant.conn != nil {
		_ = ant.conn.Close()
		ant.conn = nil
	}
	ant.connMu.Unlock()

	// Enqueue directly into offline buffer while disconnected
	payload := []byte(`{"buffered":true}`)
	err = ant.buffer.Enqueue("formicary-queue-registration", payload, nil)
	require.NoError(t, err)

	count, err := ant.buffer.Count()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	// Subscribe on server to receive the drained message
	received := make(chan *MessageEvent, 1)
	_, err = serverClient.Subscribe(ctx, SubscribeOptions{
		Topic: "formicary-queue-registration",
		Callback: func(ctx context.Context, msg *MessageEvent, ack, nack AckHandler) error {
			received <- msg
			ack()
			return nil
		},
	})
	require.NoError(t, err)

	// Trigger reconnect
	select {
	case ant.reconnectCh <- struct{}{}:
	default:
	}

	// Wait for reconnection and buffer drain
	select {
	case msg := <-received:
		assert.Equal(t, payload, msg.Payload)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: buffered message not delivered after reconnect")
	}
}

// TestWebSocket_ValidateErrors: send with empty payload returns error
func TestWebSocket_ValidateErrors(t *testing.T) {
	serverClient, makeAnt := startTestServer(t)
	ant := makeAnt()
	ctx := context.Background()

	// Empty payload
	_, err := ant.Send(ctx, "formicary-queue-registration", []byte{}, nil)
	assert.Error(t, err)

	_, err = serverClient.Send(ctx, "formicary-queue-registration", []byte{}, nil)
	assert.Error(t, err)

	// Empty topic
	_, err = ant.Send(ctx, "", []byte("data"), nil)
	assert.Error(t, err)
}

// TestWebSocket_FactoryCreation: CreateClient returns correct implementation
func TestWebSocket_FactoryCreation(t *testing.T) {
	ctx := context.Background()

	commonCfg := &types.CommonConfig{
		ID: "test-node",
		Queue: &types.QueueConfig{
			Provider: types.WebSocketMessagingProvider,
			WebSocket: &types.WebSocketConfig{
				// No ServerEndpoint → server mode
			},
		},
	}
	opTimeout := 5 * time.Second
	connTimeout := 5 * time.Second
	commonCfg.Queue.OperationTimeout = &opTimeout
	commonCfg.Queue.ConnectionTimeout = &connTimeout
	commonCfg.Queue.RetryMax = 3
	commonCfg.Queue.MaxMessageSize = 1024 * 1024
	commonCfg.Queue.WebSocket.Validate()

	client, err := CreateClient(ctx, commonCfg)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.IsType(t, &ClientWebSocketServer{}, client)
	client.Close()
}

// buildSignedJWT creates a test JWT signed with the given secret and token_type claim.
func buildSignedJWT(t *testing.T, secret, tokenType string) string {
	t.Helper()
	claims := &web.JwtClaims{
		UserID:    "test-user",
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// serverWithJWTSecret builds a ClientWebSocketServer with a JWT secret configured.
func serverWithJWTSecret(t *testing.T, secret string) *ClientWebSocketServer {
	t.Helper()
	cfg := buildTestQueueConfig()
	cfg.JWTSecret = secret
	srv, err := newWebSocketServerClient(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(srv.Close)
	return srv
}

// serverWithStaticToken builds a ClientWebSocketServer with a static token configured.
func serverWithStaticToken(t *testing.T, token string) *ClientWebSocketServer {
	t.Helper()
	cfg := buildTestQueueConfig()
	cfg.Token = token
	srv, err := newWebSocketServerClient(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(srv.Close)
	return srv
}

func makeAuthRequest(t *testing.T, bearer string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "/ws/queue", nil)
	require.NoError(t, err)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	return r
}

// TestAuthenticate_NoAuth: when neither JWTSecret nor Token is set, all connections are accepted.
func TestAuthenticate_NoAuth(t *testing.T) {
	cfg := buildTestQueueConfig()
	srv, err := newWebSocketServerClient(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	assert.NoError(t, srv.authenticate(makeAuthRequest(t, "")))
	assert.NoError(t, srv.authenticate(makeAuthRequest(t, "anything")))
}

// TestAuthenticate_JWTSecret_ValidAPIToken: valid JWT with token_type=api is accepted.
func TestAuthenticate_JWTSecret_ValidAPIToken(t *testing.T) {
	const secret = "test-secret-32-bytes-long-enough!"
	srv := serverWithJWTSecret(t, secret)

	token := buildSignedJWT(t, secret, web.TokenTypeAPI)
	assert.NoError(t, srv.authenticate(makeAuthRequest(t, token)))
}

// TestAuthenticate_JWTSecret_SessionTokenRejected: session tokens must NOT connect as ants.
func TestAuthenticate_JWTSecret_SessionTokenRejected(t *testing.T) {
	const secret = "test-secret-32-bytes-long-enough!"
	srv := serverWithJWTSecret(t, secret)

	token := buildSignedJWT(t, secret, web.TokenTypeSession)
	err := srv.authenticate(makeAuthRequest(t, token))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token_type")
}

// TestAuthenticate_JWTSecret_WrongSecret: token signed with a different secret is rejected.
func TestAuthenticate_JWTSecret_WrongSecret(t *testing.T) {
	const secret = "test-secret-32-bytes-long-enough!"
	srv := serverWithJWTSecret(t, secret)

	token := buildSignedJWT(t, "completely-different-secret-xyz!!", web.TokenTypeAPI)
	err := srv.authenticate(makeAuthRequest(t, token))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JWT")
}

// TestAuthenticate_JWTSecret_MissingHeader: no Authorization header is rejected.
func TestAuthenticate_JWTSecret_MissingHeader(t *testing.T) {
	srv := serverWithJWTSecret(t, "test-secret-32-bytes-long-enough!")
	err := srv.authenticate(makeAuthRequest(t, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bearer")
}

// TestAuthenticate_JWTSecret_ExpiredToken: expired JWT is rejected.
func TestAuthenticate_JWTSecret_ExpiredToken(t *testing.T) {
	const secret = "test-secret-32-bytes-long-enough!"
	srv := serverWithJWTSecret(t, secret)

	claims := &web.JwtClaims{
		TokenType: web.TokenTypeAPI,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	authErr := srv.authenticate(makeAuthRequest(t, signed))
	require.Error(t, authErr)
	assert.Contains(t, authErr.Error(), "invalid JWT")
}

// TestAuthenticate_StaticToken_Valid: correct static token is accepted.
func TestAuthenticate_StaticToken_Valid(t *testing.T) {
	srv := serverWithStaticToken(t, "my-static-secret")
	assert.NoError(t, srv.authenticate(makeAuthRequest(t, "my-static-secret")))
}

// TestAuthenticate_StaticToken_Wrong: wrong static token is rejected.
func TestAuthenticate_StaticToken_Wrong(t *testing.T) {
	srv := serverWithStaticToken(t, "my-static-secret")
	err := srv.authenticate(makeAuthRequest(t, "wrong-token"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

// TestAuthenticate_StaticToken_MissingHeader: no Authorization header is rejected.
func TestAuthenticate_StaticToken_MissingHeader(t *testing.T) {
	srv := serverWithStaticToken(t, "my-static-secret")
	err := srv.authenticate(makeAuthRequest(t, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bearer")
}

// TestAuthenticate_HTTPHandler_RejectsUnauth: HTTPHandler returns 401 for unauthenticated requests.
func TestAuthenticate_HTTPHandler_RejectsUnauth(t *testing.T) {
	srv := serverWithJWTSecret(t, "test-secret-32-bytes-long-enough!")
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/ws/queue")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
