package queue

import (
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateMessageEvent(t *testing.T) {
	msg := kafka.Message{
		Key:   []byte("key"),
		Value: []byte("value"),
	}
	event := kafkaMessageToEvent(msg, nil, nil)
	require.Equal(t, 0, len(event.Properties))
	event.Properties = NewMessageHeaders(CorrelationIDKey, "123", replyTopicKey, "reply-topic")
	require.Equal(t, 2, len(event.Properties))
}
