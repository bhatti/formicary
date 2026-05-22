package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSFrameMarshalUnmarshal(t *testing.T) {
	t.Run("publish frame round-trip", func(t *testing.T) {
		props := MessageHeaders{
			CorrelationIDKey: "corr-123",
			replyTopicKey:    "formicary-queue-ant-reply",
		}
		orig := newPublishFrame("frame-1", "formicary-queue-ant-request", []byte(`{"action":"test"}`), props)

		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)

		assert.Equal(t, WSFramePublish, got.Type)
		assert.Equal(t, "frame-1", got.ID)
		assert.Equal(t, "formicary-queue-ant-request", got.Topic)
		assert.Equal(t, []byte(`{"action":"test"}`), got.Payload)
		assert.Equal(t, "corr-123", got.Properties.GetCorrelationID())
		assert.Equal(t, "formicary-queue-ant-reply", got.Properties.GetReplyTopic())
	})

	t.Run("subscribe frame", func(t *testing.T) {
		orig := newSubscribeFrame("f1", "my-topic", "my-group", true)
		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFrameSubscribe, got.Type)
		assert.Equal(t, "f1", got.ID)
		assert.Equal(t, "my-topic", got.Topic)
		assert.Equal(t, "my-group", got.Group)
		assert.True(t, got.Shared)
	})

	t.Run("unsubscribe frame", func(t *testing.T) {
		orig := newUnsubscribeFrame("f2", "my-topic", "sub-999")
		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFrameUnsubscribe, got.Type)
		assert.Equal(t, "sub-999", got.SubscriptionID)
	})

	t.Run("ack frame", func(t *testing.T) {
		orig := newAckFrame("ref-123")
		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFrameAck, got.Type)
		assert.Equal(t, "ref-123", got.ID)
	})

	t.Run("nack frame", func(t *testing.T) {
		orig := newNackFrame("ref-456")
		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFrameNack, got.Type)
	})

	t.Run("error frame", func(t *testing.T) {
		orig := newErrorFrame("ref-789", "something went wrong")
		data, err := MarshalWSFrame(orig)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFrameError, got.Type)
		assert.Equal(t, "something went wrong", got.Error)
	})

	t.Run("ping and pong frames", func(t *testing.T) {
		ping := newPingFrame("ping-1")
		data, err := MarshalWSFrame(ping)
		require.NoError(t, err)

		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.Equal(t, WSFramePing, got.Type)

		pong := newPongFrame(got.ID)
		data2, err := MarshalWSFrame(pong)
		require.NoError(t, err)

		got2, err := UnmarshalWSFrame(data2)
		require.NoError(t, err)
		assert.Equal(t, WSFramePong, got2.Type)
		assert.Equal(t, "ping-1", got2.ID)
	})

	t.Run("timestamp auto-set on marshal", func(t *testing.T) {
		f := &WSFrame{Type: WSFramePing, ID: "t1"}
		assert.True(t, f.Timestamp.IsZero())
		data, err := MarshalWSFrame(f)
		require.NoError(t, err)
		got, err := UnmarshalWSFrame(data)
		require.NoError(t, err)
		assert.False(t, got.Timestamp.IsZero())
		assert.WithinDuration(t, time.Now(), got.Timestamp, 2*time.Second)
	})

	t.Run("unmarshal invalid JSON", func(t *testing.T) {
		_, err := UnmarshalWSFrame([]byte(`not json`))
		assert.Error(t, err)
	})
}
