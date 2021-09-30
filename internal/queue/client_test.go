package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twinj/uuid"
)

var randomTopic = false

func Test_ShouldNotReceiveWithoutSend(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	topic, err := buildTopic(cli, "-orphan")
	require.NoError(t, err)
	id := uuid.NewV4().String()
	events := make([]*testEvent, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var received int32
	// WHEN subscribing with timeout of 1 seconds
	if err = cli.Subscribe(
		ctx,
		topic,
		id,
		make(map[string]string),
		true,
		func(ctx context.Context, event *MessageEvent) error {
			// This is callback function from subscription
			atomic.AddInt32(&received, 1)
			msg := unmarshalTestEvent(event.Payload)
			events = append(events, msg)
			_, _ = cli.Send(ctx, event.ReplyTopic(), make(map[string]string), event.Payload, true)
			return nil
		}); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

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
	ctx := context.Background()
	started := time.Now()
	max := 20

	reqTopic, err := buildTopic(cli, "-request")
	require.NoError(t, err)
	replyTopic, err := buildTopic(cli, "-reply")
	require.NoError(t, err)

	id := uuid.NewV4().String()
	var sentWait sync.WaitGroup
	sentWait.Add(max*5)

	var received int32
	// WHEN subscribing that expects to receive messages
	if err = cli.Subscribe(
		ctx,
		reqTopic,
		id,
		make(map[string]string),
		true,
		func(ctx context.Context, event *MessageEvent) error {
			defer event.Ack()
			msg := unmarshalTestEvent(event.Payload)
			if msg.ID < started.Unix() {
				t.Logf("send/receive receiving stale message id %d", msg.ID-started.Unix())
			} else {
				msg.Replied = true
				payload := marshalTestEvent(msg)
				_, _ = cli.Send(ctx, event.ReplyTopic(), make(map[string]string), payload, false)
			}
			return nil
		}); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	var n int64 = 0
	for i:=0; i<5; i++ {
		go func() {
			added := atomic.AddInt64(&n, 1)
			for j := 0; j < max; {
				event := newTestEvent(started.Unix()+added)
				if _, err := cli.SendReceive(
					ctx,
					reqTopic,
					make(map[string]string),
					marshalTestEvent(event),
					replyTopic); err == nil {
					sentWait.Done()
					atomic.AddInt32(&received, 1)
					j++
				} else {
					//t.Logf("failed to publish %v", err)
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	sentWait.Wait()
	// THEN it should receive messages
	require.True(t, atomic.AddInt32(&received, 0) >= int32(max)) // subscriber must be active before publishing
}

func Test_ShouldSubscribePubSub(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	ctx := context.Background()
	topic, err := buildTopic(cli, "-pubsub")
	require.NoError(t, err)

	sentWait := make(map[string]*sync.WaitGroup)

	started := time.Now()
	var max int32 = 20

	// subscribing 5 times
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("pubsub-subscriber-%d", i)
		var wg sync.WaitGroup
		wg.Add(int(max))
		sentWait[id] = &wg
		var received int32
		group := fmt.Sprintf("group-%d", i)
		// WHEN subscribing that expects to receive messages
		if err = cli.Subscribe(
			ctx,
			topic,
			id,
			map[string]string{"Group": group, "LastOffset": "true"},
			false,
			func(ctx context.Context, event *MessageEvent) error {
				defer event.Ack()
				msg := unmarshalTestEvent(event.Payload)
				if msg.ID < started.Unix() {
					t.Logf("pub/sub received stale %d, id %d, message %s, offset %d, group %s, topic %s",
						received, msg.ID-started.Unix(), msg.Message, event.Offset, group, topic)
				} else {
					msg.Replied = true
					atomic.AddInt32(&received, 1)
					if received <= max {
						sentWait[id].Done()
					}
				}
				return nil
			}); err != nil {
			t.Fatalf("unexpected error %s", err)
		}
	}
	// sending more messages because some queues won't send message until subscription is completed
	var i int32
	for i = 0; i < max; {
		if _, err = cli.Publish(
			ctx,
			topic,
			make(map[string]string),
			marshalTestEvent(newTestEvent(started.Unix()+int64(i))),
			false); err == nil {
			i++
		} else {
			//t.Logf("failed to publish %v", err)
			time.Sleep(10 * time.Millisecond)
		}
	}
	// THEN it should receive messages
	for _, wg := range sentWait {
		wg.Wait()
	}
	t.Logf("pub/sub done")
}

func newConfig() (*types.CommonConfig, error) {
	c := &types.CommonConfig{
		ID:     "test-client",
		Pulsar: types.PulsarConfig{URL: "pulsar://localhost:6650", ConnectionTimeout: 1 * time.Second},
		//Kafka:             types.KafkaConfig{Brokers: []string{"localhost:9092"}},
		Kafka:             types.KafkaConfig{Brokers: []string{"192.168.1.104:19092", "192.168.1.104:29092", "192.168.1.104:39092"}},
		Redis:             types.RedisConfig{Host: "localhost", Port: 6379},
		MessagingProvider: types.KafkaMessagingProvider,
		S3:                types.S3Config{AccessKeyID: "admin", SecretAccessKey: "password", Bucket: "test-bucket"},
	}
	return c, c.Validate(make([]string, 0))
}

func newStub() (Client, error) {
	cfg, err := newConfig()
	if err != nil {
		return nil, err
	}
	cfg.Kafka.Group = "test-dev-group"
	//return NewStubClient(cfg), nil
	//return newKafkaClient(cfg)
	//return newPulsarClient(&cfg.Pulsar)
	return newClientRedis(&cfg.Redis)
}

type testEvent struct {
	Message       string
	ID            int64
	Replied       bool
}

func marshalTestEvent(e *testEvent) (b []byte) {
	b, _ = json.Marshal(e)
	return
}

func newTestEvent(id int64) (e *testEvent) {
	e = &testEvent{}
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
	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		err = kafka.createKafkaTopic(topic, 1, 1)
	}
	return
}
