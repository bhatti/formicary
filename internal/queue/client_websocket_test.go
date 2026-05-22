package queue

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
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

		// Give time for handshake
		time.Sleep(50 * time.Millisecond)
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
