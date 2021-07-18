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

func Test_ShouldNotReceiveWithoutSend(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	topic := uuid.NewV4().String() + "-orphan"
	id := uuid.NewV4().String()
	events := make([]*testEvent, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		err = kafka.CreateKafkaTopic(topic, 1, 1)
	}

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
			_, _ = cli.Send(ctx, msg.ReplyTopic, make(map[string]string), event.Payload, true)
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
	started := time.Now()
	inTopic := uuid.NewV4().String() + "-request"
	replyTopic := uuid.NewV4().String() + "-reply"
	id := uuid.NewV4().String()
	events := make([]*testEvent, 0)
	var sentWait sync.WaitGroup
	sentWait.Add(15)
	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		err = kafka.CreateKafkaTopic(inTopic, 1, 1)
		err = kafka.CreateKafkaTopic(replyTopic, 1, 1)
	}

	received := 0
	// WHEN subscribing that expects to receive 15 messages
	if err = cli.Subscribe(
		context.Background(),
		inTopic,
		id,
		make(map[string]string),
		true,
		func(ctx context.Context, event *MessageEvent) error {
			msg := unmarshalTestEvent(event.Payload)
			received++
			events = append(events, msg)
			_, _ = cli.Send(ctx, msg.ReplyTopic, make(map[string]string), event.Payload, false)
			if len(events) <= 15 {
				sentWait.Done()
			}
			return nil
		}); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	// sending more messages because some queues won't send message until subscription is completed
	for i := 0; i < 20; {
		if _, err = cli.SendReceive(
			context.Background(),
			inTopic,
			make(map[string]string),
			marshalTestEvent(newTestEvent(replyTopic)),
			replyTopic); err == nil {
			i++
		} else {
			//t.Logf("failed to publish %v", err)
			time.Sleep(10 * time.Millisecond)
		}
	}
	sentWait.Wait()
	// THEN it should receive messages
	require.True(t, len(events) >= 15) // subscriber must be active before publishing
	t.Logf("received %d, elapsed %v", received, time.Since(started))
}

func Test_ShouldSubscribePubSub(t *testing.T) {
	// GIVEN queue client
	cli, err := newStub()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	topic := uuid.NewV4().String() + "-pubsub"

	sentWait := make(map[string]*sync.WaitGroup)
	switch cli.(type) {
	case *ClientKafka:
		kafka := cli.(*ClientKafka)
		err = kafka.CreateKafkaTopic(topic, 1, 1)
	}
	// subscribing 5 times
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("pubsub-subscriber-%d", i)
		var wg sync.WaitGroup
		wg.Add(15)
		sentWait[id] = &wg
		var received int32
		// WHEN subscribing that expects to receive 15 messages
		if err = cli.Subscribe(
			context.Background(),
			topic,
			id,
			make(map[string]string),
			false,
			func(ctx context.Context, event *MessageEvent) error {
				atomic.AddInt32(&received, 1)
				if received <= 15 {
					sentWait[id].Done()
				}
				return nil
			}); err != nil {
			t.Fatalf("unexpected error %s", err)
		}
	}
	// sending more messages because some queues won't send message until subscription is completed
	for i := 0; i < 20; {
		if _, err = cli.Publish(
			context.Background(),
			topic,
			make(map[string]string),
			[]byte(fmt.Sprintf("%s:test-payload-%d", uuid.NewV4().String(), i)),
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
}

func newConfig() (*types.CommonConfig, error) {
	c := &types.CommonConfig{
		ID:                "test-client",
		Pulsar:            types.PulsarConfig{URL: "pulsar://localhost:6650", ConnectionTimeout: 1 * time.Second},
		Kafka:             types.KafkaConfig{Brokers: []string{"localhost:9092"}, Algorithm: "sha256"},
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
	return NewStubClient(cfg)
	//cfg.Kafka.Group = ""
	//return newKafkaClient(cfg)
	//return newPulsarClient(&cfg.Pulsar)
	//return newClientRedis(&cfg.Redis)
}

type testEvent struct {
	Message    string
	ReplyTopic string
}

func marshalTestEvent(e testEvent) (b []byte) {
	b, _ = json.Marshal(e)
	return
}

func newTestEvent(reply string) (e testEvent) {
	e.Message = uuid.NewV4().String()
	e.ReplyTopic = reply
	return
}

func unmarshalTestEvent(b []byte) (e *testEvent) {
	e = &testEvent{}
	_ = json.Unmarshal(b, e)
	return
}
