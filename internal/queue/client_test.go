package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twinj/uuid"
)

var randomTopic = false

func Test_ShouldCreateDeleteTopics(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	err = createKafkaTopic(cli, "formicary-queue-my-test")
	require.NoError(t, err)
	err = deleteKafkaTopic(cli, "formicary-queue-my-test")
	require.NoError(t, err)
}

func Test_ShouldNotReceiveWithoutSend(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	topic, err := buildTopic(cli, "-orphan")
	require.NoError(t, err)
	events := make([]*testEvent, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var received int32
	// WHEN subscribing with timeout of 1 seconds
	id, err := cli.Subscribe(
		ctx,
		topic,
		true,
		func(ctx context.Context, event *MessageEvent) error {
			// This is callback function from subscription
			atomic.AddInt32(&received, 1)
			msg := unmarshalTestEvent(event.Payload)
			events = append(events, msg)
			_, _ = cli.Send(ctx, event.ReplyTopic(), event.Payload, make(map[string]string))
			return nil
		},
		nil,
		make(map[string]string),
	)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	defer func() {
		_ = cli.UnSubscribe(ctx, topic, id)
	}()

	select {
	case <-ctx.Done():
		require.Equal(t, int32(0), received)
		return
	}
}

func Test_ShouldSendReceive(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	started := time.Now()
	groups := 5
	max := 20

	reqTopic, err := buildTopic(cli, "-request")
	require.NoError(t, err)
	replyTopic, err := buildTopic(cli, "-reply")
	require.NoError(t, err)

	var received int32
	// WHEN subscribing that expects to receive messages
	id, err := cli.Subscribe(
		ctx,
		reqTopic,
		true,
		func(ctx context.Context, event *MessageEvent) error {
			defer event.Ack()
			msg := unmarshalTestEvent(event.Payload)
			if msg.ID < started.Unix() {
				//t.Logf("send/receive receiving stale message id %d, elapsed %f corelID %s",
				//	msg.ID-started.Unix(), time.Now().Sub(started).Seconds(), event.CoRelationID())
			} else {
				msg.Replied = true
				payload := marshalTestEvent(msg)
				_, _ = cli.Send(
					ctx,
					event.ReplyTopic(),
					payload,
					NewMessageHeaders(CorrelationIDKey, event.CoRelationID()),
				)
				t.Logf("send/receive receiving message id %d, elapsed %f corelID %s",
					msg.ID-started.Unix(), time.Now().Sub(started).Seconds(), event.CoRelationID())
			}
			return nil
		},
		nil,
		make(map[string]string),
	)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	defer func() {
		_ = cli.UnSubscribe(ctx, reqTopic, id)
	}()

	var n int64 = 0
	for i := 0; i < groups; i++ {
		added := atomic.AddInt64(&n, 1)
		for j := 0; j < max; {
			event := newTestEvent(started.Unix() + added)
			if _, err := cli.SendReceive(
				ctx,
				reqTopic,
				marshalTestEvent(event),
				replyTopic,
				make(map[string]string),
			); err == nil {
				atomic.AddInt32(&received, 1)
				j++
			} else {
				t.Fatalf(err.Error())
			}
		}
	}

	// THEN it should receive messages
	require.True(t, atomic.AddInt32(&received, 0) >= int32(max)) // subscriber must be active before publishing
}

func Test_ShouldSubscribePubSub(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	topic, err := buildTopic(cli, "-pubsub")
	require.NoError(t, err)
	_, err = buildTopic(cli, "-reply")
	require.NoError(t, err)

	started := time.Now()
	groups := 5
	var max = 20
	ids := make([]string, groups)
	receiveCh := make(chan int, groups*max)

	// subscribing multiple times for each group so that it receives its own copy
	for i := 0; i < groups; i++ {
		group := fmt.Sprintf("group-%d", i)
		// WHEN subscribing that expects to receive messages
		ids[i], err = cli.Subscribe(
			ctx,
			topic,
			false,
			func(ctx context.Context, event *MessageEvent) error {
				defer event.Ack()
				msg := unmarshalTestEvent(event.Payload)
				if msg.ID < started.Unix() {
					//t.Logf("pub/sub received stale id %d, message %s, offset %d, group %s, topic %s, corelID %s",
					//	msg.ID-started.Unix(), msg.Message, event.Offset, group, topic, event.CoRelationID())
				} else {
					t.Logf("pub/sub received id %d, message %s, offset %d, group %s, topic %s, corelID %s",
						msg.ID-started.Unix(), msg.Message, event.Offset, group, topic, event.CoRelationID())
					receiveCh <- 1
				}
				return nil
			},
			nil,
			map[string]string{groupKey: group, "LastOffset": "true"},
		)
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
	}
	time.Sleep(time.Millisecond)
	// sending more messages because some queues won't send message until subscription is completed
	for i := 0; i < max; {
		if _, err = cli.Publish(
			ctx,
			topic,
			marshalTestEvent(newTestEvent(started.Unix()+int64(i))),
			make(map[string]string),
		); err == nil {
			i++
			t.Logf("published %d message to %s", i, topic)
		} else {
			//t.Logf("failed to publish %v", err)
			time.Sleep(10 * time.Millisecond)
		}
	}

	defer func() {
		for _, id := range ids {
			_ = cli.UnSubscribe(ctx, topic, id)
		}
	}()
	received := 0
	// wait for confirmation
	for {
		select {
		case <-ctx.Done():
			if received < groups*max {
				t.Fatalf("failed to receive all messages %s, received %d", ctx.Err(), received)
			}
			return
		case _ = <-receiveCh:
			received++
			if received >= groups*max {
				// THEN it should receive messages
				t.Logf("pub/sub done %d", received)
				return
			}
		}
	}
}

func newConfig() (*types.CommonConfig, error) {
	c := &types.CommonConfig{
		ID:     "test-client",
		Pulsar: types.PulsarConfig{URL: "pulsar://localhost:6650", ConnectionTimeout: 1 * time.Second},
		//Kafka:             types.KafkaConfig{Brokers: []string{"localhost:9092"}},
		Kafka:             types.KafkaConfig{Brokers: []string{"192.168.1.102:19092", "192.168.1.102:29092", "192.168.1.102:39092"}},
		Redis:             types.RedisConfig{Host: "localhost", Port: 6379},
		MessagingProvider: types.KafkaMessagingProvider,
		S3:                types.S3Config{AccessKeyID: "admin", SecretAccessKey: "password", Bucket: "test-bucket"},
	}
	c.Kafka.CommitTimeout = 5 * time.Minute // commit older than 5 minute
	return c, c.Validate(make([]string, 0))
}

func newStub() (Client, error) {
	cfg, err := newConfig()
	if err != nil {
		return nil, err
	}
	cfg.Kafka.Group = "test-dev-group"
	return NewStubClient(cfg), nil
	//return newKafkaClient(cfg)
	//return newPulsarClient(&cfg.Pulsar)
	//return newClientRedis(&cfg.Redis)
}

type testEvent struct {
	Version string
	Message string
	ID      int64
	Replied bool
}

func marshalTestEvent(e *testEvent) (b []byte) {
	b, _ = json.Marshal(e)
	return
}

func newTestEvent(id int64) (e *testEvent) {
	e = &testEvent{}
	e.Version = "V1.0.0"
	e.ID = id
	e.Message = uuid.NewV4().String()
	return
}

func unmarshalTestEvent(b []byte) (e *testEvent) {
	e = &testEvent{}
	_ = json.Unmarshal(b, e)
	return
}

func buildTopic(cli Client, suffix string) (topic string, err error) {
	if randomTopic {
		topic = uuid.NewV4().String() + suffix
	} else {
		topic = "dev-test-topic" + suffix
	}
	//_ = deleteKafkaTopic(cli, topic)
	err = createKafkaTopic(cli, topic)
	return
}

func createKafkaTopic(cli Client, topic string) error {
	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		return kafka.createKafkaTopic(topic, 2, 2)
	}
	return nil
}

func deleteKafkaTopic(cli Client, topic string) error {
	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		return kafka.deleteKafkaTopic(topic)
	}
	return nil
}
