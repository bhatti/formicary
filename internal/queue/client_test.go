package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/utils"
	"os"
	"path"
	"plexobject.com/formicary/internal/types"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func getTestProviders() (res []types.MessagingProvider) {
	providers := os.Getenv("TEST_QUEUE_PROVIDERS")
	if providers == "" {
		return []types.MessagingProvider{
			//types.KafkaMessagingProvider,
			//types.PulsarMessagingProvider,
			//types.RedisMessagingProvider,
			types.ChannelMessagingProvider,
		}
	}
	all := []string{
		string(types.KafkaMessagingProvider),
		string(types.PulsarMessagingProvider),
		string(types.RedisMessagingProvider),
		string(types.ChannelMessagingProvider),
	}
	for _, provider := range strings.Split(providers, ",") {
		provider = strings.ToUpper(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		if utils.Contains(all, provider) {
			res = append(res, types.MessagingProvider(provider))
		}
	}
	return res
}

// TestConfig provides test configuration
type TestConfig struct {
	provider          types.MessagingProvider
	bootstrapServers  []string
	numPartitions     int
	replicationFactor int
}

// TestMessage represents a test message payload
type TestMessage struct {
	ID      string    `json:"id"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

func init() {
	// Set logrus to debug level and show file/line numbers
	logrus.SetLevel(logrus.InfoLevel) // DebugLevel
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
}

// TestBasicPublishSubscribe tests basic publish and subscribe functionality
func TestBasicPublishSubscribe(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "basic")

			var receivedMsg []byte
			var wg sync.WaitGroup
			wg.Add(1)

			// Set up subscriber
			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				receivedMsg = event.Payload
				ack()
				wg.Done()
				return nil
			}

			_, cleanup := createSubscription(t, ctx, client, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Props: MessageHeaders{
					"Group": "test-Group-" + ulid.Make().String(),
				},
			})
			defer cleanup()

			msg := TestMessage{
				ID:      ulid.Make().String(),
				Content: "test message",
				Time:    time.Now(),
			}

			// Publish message
			msgID := publishTestMessage(t, client, topic, msg, nil)
			require.NotEmpty(t, msgID)

			// Wait with timeout
			waitWithTimeout(t, &wg, ctx, "waiting for message")
			payload, err := json.Marshal(msg)
			require.NoError(t, err)
			require.Equal(t, payload, receivedMsg)
		})
	}
}

// TestSendReceive tests request-response pattern
func TestSendReceive(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			started := time.Now()
			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			outTopic := createTestTopic(t, client, "out")
			inTopic := createTestTopic(t, client, "in")

			var wg sync.WaitGroup
			wg.Add(1)

			// Set up response handler with correlation ID check
			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				defer wg.Done()
				t.Logf("Received request with correlation ID: %s", event.CoRelationID())

				// Echo back received message using the same correlation ID
				props := make(MessageHeaders)
				props.SetCorrelationID(event.CoRelationID())
				replyTopic := event.ReplyTopic()
				if replyTopic == "" {
					replyTopic = inTopic // fallback to inTopic if reply topic not set
				}
				t.Logf("Publishing response to topic: %s with correlation ID: %s",
					replyTopic, event.CoRelationID())

				// Publish response
				_, err := client.Publish(ctx, replyTopic, event.Payload, props)
				if err != nil {
					t.Logf("Failed to publish response: %v", err)
					nack()
					return err
				}

				t.Logf("Published response with correlation ID: %s", event.CoRelationID())
				ack()
				return nil
			}

			// Subscribe to outgoing topic
			_, cleanup := createSubscription(t, ctx, client, SubscribeOptions{
				Topic:    outTopic,
				Callback: callback,
				Shared:   false, // Use non-shared subscription for reliability
			})
			defer cleanup()

			// Create and send request
			req := &SendReceiveRequest{
				OutTopic: outTopic,
				InTopic:  inTopic,
				Payload:  []byte("test request"),
				Timeout:  10 * time.Second,
			}

			// Allow time for subscription to be fully set up
			time.Sleep(10 * time.Millisecond)

			started = time.Now()
			resp, err := client.SendReceive(ctx, req)
			require.NoError(t, err, "elapsed %s", time.Since(started))
			require.NotNilf(t, resp.Event, "elapsed %s", time.Since(started))
			require.Equal(t, req.Payload, resp.Event.Payload)
			resp.Ack()

			waitWithTimeout(t, &wg, ctx, "waiting for message")
		})
	}
}

// TestSubscribeFilter tests message filtering
func TestSubscribeFilter(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "filter")

			var receivedCount int32
			var wg sync.WaitGroup
			expectedMessages := 1
			wg.Add(expectedMessages)

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				atomic.AddInt32(&receivedCount, 1)
				ack()
				wg.Done()
				return nil
			}

			filter := func(ctx context.Context, event *MessageEvent) bool {
				return event.Properties["pass"] == "true"
			}

			_, cleanup := createSubscription(t, ctx, client, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Filter:   filter,
				Props: MessageHeaders{
					"Group": "test-filter-Group",
				},
			})
			defer cleanup()

			// Send test messages
			publishTestMessage(t, client, topic, TestMessage{
				ID:      ulid.Make().String(),
				Content: "should pass",
				Time:    time.Now(),
			}, MessageHeaders{"pass": "true"})

			publishTestMessage(t, client, topic, TestMessage{
				ID:      ulid.Make().String(),
				Content: "should not pass",
				Time:    time.Now(),
			}, MessageHeaders{"pass": "false"})

			waitWithTimeout(t, &wg, ctx, "waiting for filtered message")
			require.Equal(t, int32(expectedMessages), atomic.LoadInt32(&receivedCount))
		})
	}
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, cfg, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			t.Run("InvalidTopic", func(t *testing.T) {
				_, err := client.Publish(ctx, "", []byte("test"), nil)
				require.Error(t, err)
			})

			t.Run("MessageTooLarge", func(t *testing.T) {
				topic := createTestTopic(t, client, "large")
				payload := make([]byte, cfg.Queue.MaxMessageSize+1)
				_, err := client.Publish(ctx, topic, payload, nil)
				require.Error(t, err)
			})

			t.Run("ContextCancelled", func(t *testing.T) {
				topic := createTestTopic(t, client, "cancel")
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := client.Publish(ctx, topic, []byte("test"), nil)
				require.Error(t, err)
			})
		})
	}
}

// TestTimeout tests timeout scenarios
func TestTimeout(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			t.Run("SendReceiveTimeout", func(t *testing.T) {
				outTopic := createTestTopic(t, client, "timeout_out")
				inTopic := createTestTopic(t, client, "timeout_in")

				req := &SendReceiveRequest{
					OutTopic: outTopic,
					InTopic:  inTopic,
					Payload:  []byte("test"),
					Timeout:  100 * time.Millisecond,
				}

				_, err := client.SendReceive(ctx, req)
				require.Error(t, err)
			})
		})
	}
}

// TestSharedSubscription tests shared subscription functionality
func TestSharedSubscription(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "shared")
			messageCount := 10

			// Use atomic counter for total messages received
			var totalReceived int32
			var receivedMessages sync.Map

			// Create subscribers
			var cleanups []func()
			for i := 1; i <= 2; i++ {
				subID := fmt.Sprintf("sub%d", i)
				callback := func(id string) Callback {
					return func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
						// Increment total before processing
						current := atomic.AddInt32(&totalReceived, 1)
						if current <= int32(messageCount) {
							// Only process if we haven't exceeded expected count
							value, _ := receivedMessages.LoadOrStore(id, 0)
							receivedMessages.Store(id, value.(int)+1)
							ack()
						} else {
							// If we've received too many messages, nack them
							nack()
						}
						return nil
					}
				}

				_, cleanup := createSubscription(t, ctx, client, SubscribeOptions{
					Topic:    topic,
					Callback: callback(subID),
					Shared:   true,
					Props: MessageHeaders{
						"Group": "shared-Group",
					},
				})
				cleanups = append(cleanups, cleanup)
			}
			defer func() {
				for _, cleanup := range cleanups {
					cleanup()
				}
			}()

			// Wait for subscriptions to be ready
			time.Sleep(2 * time.Second)

			// Publish messages
			t.Logf("Publishing %d messages", messageCount)
			for i := 0; i < messageCount; i++ {
				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: fmt.Sprintf("msg%d", i),
					Time:    time.Now(),
				}
				publishTestMessage(t, client, topic, msg, nil)
				time.Sleep(5 * time.Millisecond) // Small delay between messages
			}

			// Wait with timeout for all messages to be processed
			deadline := time.After(10 * time.Second)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-deadline:
					t.Fatal("Test timed out waiting for messages")
					return
				case <-ticker.C:
					received := atomic.LoadInt32(&totalReceived)
					if received >= int32(messageCount) {
						// Verify message distribution
						var total int
						var subscriberCount int
						receivedMessages.Range(func(key, value interface{}) bool {
							count := value.(int)
							total += count
							subscriberCount++
							t.Logf("Subscriber %v received %d messages", key, count)
							return true
						})

						require.Equal(t, messageCount, total, "Total messages received should match sent")
						require.Equal(t, 2, subscriberCount, "Messages should be distributed across both subscribers")
						return
					}
				}
			}
		})
	}
}

// TestRedelivery tests message redelivery on nack
func TestRedelivery(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "redelivery")
			var attempts int32 // Use atomic for thread safety
			var wg sync.WaitGroup
			wg.Add(1)

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				currentAttempt := atomic.AddInt32(&attempts, 1)
				if currentAttempt == 1 {
					nack() // First attempt - nack the message
				} else {
					ack() // Second attempt - ack the message
					wg.Done()
				}
				return nil
			}

			props := make(MessageHeaders)
			props[groupKey] = "redelivery-Group"

			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Group:    props[groupKey],
				Props:    props,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Publish message
			_, err = client.Publish(ctx, topic, []byte("test"), nil)
			require.NoError(t, err)

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				require.Equal(t, int32(2), atomic.LoadInt32(&attempts),
					"Expected exactly 2 delivery attempts")
			case <-ctx.Done():
				t.Fatalf("Test timed out after %v. Attempts made: %d",
					10*time.Second, atomic.LoadInt32(&attempts))
			}
		})
	}
}

// TestConcurrentOperations tests concurrent publish and subscribe operations
func TestConcurrentOperations(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "concurrent")
			messageCount := 100
			subscriberCount := 5
			// Since each message goes to all subscribers (except same group)
			expectedTotal := messageCount * (subscriberCount - 1)

			receivedCount := int32(0)
			var wg sync.WaitGroup
			wg.Add(expectedTotal) // Adjust the wait count for multiple subscribers

			var subIds []string
			// Register multiple subscribers
			for i := 0; i < subscriberCount; i++ {
				groupID := fmt.Sprintf("concurrent-Group-%d", i)
				callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
					// Check if message is from same group
					if event.Properties[groupKey] == groupID {
						// Skip messages from same group
						return nil
					}
					atomic.AddInt32(&receivedCount, 1)
					ack()
					wg.Done()
					return nil
				}

				props := make(MessageHeaders)
				props[groupKey] = groupID

				subID, err := client.Subscribe(ctx, SubscribeOptions{
					Topic:    topic,
					Callback: callback,
					Group:    groupID,
					Props:    props,
					Shared:   false,
				})
				require.NoError(t, err)
				subIds = append(subIds, subID)
			}
			defer func() {
				for _, subID := range subIds {
					_ = client.UnSubscribe(ctx, topic, subID)
				}
			}()

			// Publish messages concurrently
			var publishWg sync.WaitGroup
			publishWg.Add(messageCount)
			for i := 0; i < messageCount; i++ {
				go func(i int) {
					defer publishWg.Done()
					groupID := fmt.Sprintf("concurrent-Group-%d", i%subscriberCount)
					props := make(MessageHeaders)
					props[groupKey] = groupID

					_, err := client.Publish(ctx, topic, []byte(fmt.Sprintf("msg%d", i)), props)
					require.NoError(t, err)
				}(i)
			}

			publishWg.Wait()
			wg.Wait()

			// Each message should be received by (subscriberCount - 1) subscribers
			require.Equal(t, int32(expectedTotal), atomic.LoadInt32(&receivedCount),
				"Expected each message to be received by all subscribers except same group")
		})
	}
}

// TestMetrics tests queue metrics collection
func TestMetrics(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "metrics")
			messageCount := 10
			var wg sync.WaitGroup
			wg.Add(messageCount)

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				ack()
				wg.Done()
				return nil
			}

			// Create subscriber
			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Publish messages
			for i := 0; i < messageCount; i++ {
				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: fmt.Sprintf("message-%d", i),
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				_, err = client.Publish(ctx, topic, payload, nil)
				require.NoError(t, err)
			}

			// Wait for message processing
			wg.Wait()

			// Check metrics
			metrics, err := client.GetMetrics(ctx, topic)
			require.NoError(t, err)
			require.Equal(t, topic, metrics.Topic)
			require.Equal(t, int64(messageCount), metrics.MessagesConsumed)
		})
	}
}

// TestMultipleSubscriberGroups tests multiple subscriber groups receiving messages
func TestMultipleSubscriberGroups(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "Group")

			messageCount := 5
			groupCount := 3
			// Each message should be delivered to all subscribers since they're non-shared
			expectedTotal := messageCount * groupCount

			var wg sync.WaitGroup
			wg.Add(expectedTotal)
			t.Logf("Added %d items to WaitGroup (%d messages * %d groups)",
				expectedTotal, messageCount, groupCount)

			// Track received messages per Group
			receivedMessages := make(map[string][]string)
			var mu sync.Mutex

			// Create subscribers for each Group
			var subIds []string
			for i := 0; i < groupCount; i++ {
				groupID := fmt.Sprintf("Group-%d", i)
				t.Logf("Creating subscriber for Group: %s", groupID)

				callback := func(groupID string) Callback {
					return func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
						mu.Lock()
						receivedMessages[groupID] = append(receivedMessages[groupID], string(event.ID))
						t.Logf("Received message for Group %s: %s",
							groupID, event.ID)
						wg.Done()
						mu.Unlock()
						ack()
						return nil
					}
				}(groupID)

				props := make(MessageHeaders)
				props[groupKey] = groupID

				subID, err := client.Subscribe(ctx, SubscribeOptions{
					Topic:    topic,
					Shared:   false,
					Callback: callback,
					Group:    groupID,
					Props:    props,
				})
				require.NoError(t, err)
				t.Logf("Successfully created subscriber %s for Group %s", subID, groupID)
				subIds = append(subIds, subID)
			}

			// Allow subscriptions to fully set up
			time.Sleep(100 * time.Millisecond)

			// Publish messages
			var publishedMsgIds []string
			for i := 0; i < messageCount; i++ {
				props := make(MessageHeaders)
				msgID, err := client.Publish(ctx, topic, []byte(fmt.Sprintf("message-%d", i)), props)
				require.NoError(t, err)
				publishedMsgIds = append(publishedMsgIds, string(msgID))
				t.Logf("Published message %d/%d with ID %s", i+1, messageCount, msgID)
			}

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				t.Log("All messages processed successfully")
			case <-ctx.Done():
				mu.Lock()
				t.Log("Test timed out. Final state:")
				for group, msgs := range receivedMessages {
					t.Logf("Group %s received messages: %v", group, msgs)
				}
				t.Logf("Published message IDs: %v", publishedMsgIds)
				mu.Unlock()
				t.Fatal("Test timed out waiting for messages")
			}

			// Verify results
			mu.Lock()
			defer mu.Unlock()

			for i := 0; i < groupCount; i++ {
				groupID := fmt.Sprintf("Group-%d", i)
				received := len(receivedMessages[groupID])
				// Each group should receive all messages since they're non-shared
				require.Equal(t, messageCount, received,
					"Group %s received %d messages, expected %d",
					groupID, received, messageCount)
			}
		})
	}
}

// TestDeadLetterQueue tests DLQ functionality
func TestDeadLetterQueue(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "dlq")
			dlqTopic := topic + "-dlq"

			var processingErrors int32
			var dlqReceived int32
			var wg sync.WaitGroup
			wg.Add(1)

			// Setup main consumer that fails processing
			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				atomic.AddInt32(&processingErrors, 1)
				// Simulate processing error
				nack()
				return fmt.Errorf("simulated processing error")
			}

			// Setup DLQ consumer
			dlqCallback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				atomic.AddInt32(&dlqReceived, 1)
				ack()
				wg.Done()
				return nil
			}

			// Subscribe to main topic
			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Props: MessageHeaders{
					"DeadLetterQueue": dlqTopic,
					"MaxRetries":      "3",
				},
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Subscribe to DLQ topic
			dlqSubID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    dlqTopic,
				Callback: dlqCallback,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, dlqTopic, dlqSubID)
			}()

			// Publish test message
			msg := TestMessage{
				ID:      ulid.Make().String(),
				Content: "dlq-test",
				Time:    time.Now(),
			}
			payload, err := json.Marshal(msg)
			require.NoError(t, err)

			_, err = client.Publish(ctx, topic, payload, nil)
			require.NoError(t, err)

			// Wait for message to reach DLQ
			wg.Wait()

			require.Equal(t, int32(3), atomic.LoadInt32(&processingErrors)) // Max retries
			require.Equal(t, int32(1), atomic.LoadInt32(&dlqReceived))      // Message in DLQ
		})
	}
}

// TestPartitionedConsumption tests consumption from specific partitions
func TestPartitionedConsumption(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "partition")
			messageCount := 20
			partitionCount := 4

			// Track received messages per partition
			receivedMessages := make(map[int32][]string)
			receivedIds := make(map[string]bool) // Track unique message IDs
			var mu sync.Mutex

			var subIds []string
			// Create consumers for specific partitions
			for partition := 0; partition < partitionCount; partition++ {
				callback := func(partitionID int32) Callback {
					return func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
						mu.Lock()
						receivedMessages[partitionID] = append(receivedMessages[partitionID], string(event.ID))
						receivedIds[string(event.ID)] = true
						mu.Unlock()
						ack()
						return nil
					}
				}

				props := make(MessageHeaders)
				props["partition"] = fmt.Sprintf("%d", partition)

				subID, err := client.Subscribe(ctx, SubscribeOptions{
					Topic:    topic,
					Callback: callback(int32(partition)),
					Props:    props,
				})
				require.NoError(t, err)
				subIds = append(subIds, subID)
			}
			defer func() {
				for _, subID := range subIds {
					_ = client.UnSubscribe(ctx, topic, subID)
				}
			}()

			// Use a WaitGroup to ensure all messages are published
			var publishWg sync.WaitGroup
			publishWg.Add(messageCount)

			// Publish messages with partition keys
			for i := 0; i < messageCount; i++ {
				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: fmt.Sprintf("message-%d", i),
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				props := make(MessageHeaders)
				props["partition_key"] = fmt.Sprintf("key-%d", i%partitionCount)

				_, err = client.Publish(ctx, topic, payload, props)
				require.NoError(t, err)
				publishWg.Done()
			}

			// Wait for all messages to be published
			publishWg.Wait()

			// Give some time for message processing
			time.Sleep(100 * time.Millisecond)

			// Verify message distribution
			mu.Lock()
			defer mu.Unlock()

			// Each message should be received at least once
			require.Equal(t, messageCount, len(receivedIds),
				"Should receive at least one copy of each message")

			// Total received messages may be greater due to non-shared subscriptions
			totalReceived := 0
			for _, messages := range receivedMessages {
				totalReceived += len(messages)
			}
			require.True(t, totalReceived >= messageCount,
				"Total received messages should be at least equal to published messages")
			require.True(t, len(receivedMessages) > 0,
				"Messages should be received by subscribers")
		})
	}
}

// TestBatchProcessing tests batch message processing
func TestBatchProcessing(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "batch")
			batchSize := 5
			numBatches := 4
			totalMessages := batchSize * numBatches

			var processedBatches int32
			var wg sync.WaitGroup
			wg.Add(numBatches)

			// Track message batches
			var batches [][]string
			var batchMu sync.Mutex

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				var msg TestMessage
				err := json.Unmarshal(event.Payload, &msg)
				require.NoError(t, err)

				batchMu.Lock()
				if len(batches) == 0 || len(batches[len(batches)-1]) >= batchSize {
					batches = append(batches, make([]string, 0, batchSize))
				}
				currentBatch := &batches[len(batches)-1]
				*currentBatch = append(*currentBatch, msg.ID)

				if len(*currentBatch) == batchSize {
					atomic.AddInt32(&processedBatches, 1)
					wg.Done()
				}
				batchMu.Unlock()

				ack()
				return nil
			}

			// Subscribe with batch processing
			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Props: MessageHeaders{
					"batch_size": fmt.Sprintf("%d", batchSize),
				},
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Publish messages
			for i := 0; i < totalMessages; i++ {
				msg := TestMessage{
					ID:      fmt.Sprintf("msg-%d", i),
					Content: fmt.Sprintf("content-%d", i),
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				_, err = client.Publish(ctx, topic, payload, nil)
				require.NoError(t, err)
			}

			wg.Wait()

			// Verify batch processing
			require.Equal(t, int32(numBatches), atomic.LoadInt32(&processedBatches))

			batchMu.Lock()
			defer batchMu.Unlock()

			require.Equal(t, numBatches, len(batches))
			for _, batch := range batches {
				require.Equal(t, batchSize, len(batch))
			}
		})
	}
}

// TestMessageOrdering tests message ordering guarantees
func TestMessageOrdering(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "order")
			messageCount := 100

			var receivedMessages []int
			var mu sync.Mutex
			var wg sync.WaitGroup
			wg.Add(messageCount)

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				var msg TestMessage
				err := json.Unmarshal(event.Payload, &msg)
				require.NoError(t, err)

				seq, err := strconv.Atoi(msg.Content)
				require.NoError(t, err)

				mu.Lock()
				receivedMessages = append(receivedMessages, seq)
				mu.Unlock()

				ack()
				wg.Done()
				return nil
			}

			// Subscribe with ordering key
			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Props: MessageHeaders{
					"ordering_key": "test-order",
				},
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Publish ordered messages
			for i := 0; i < messageCount; i++ {
				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: fmt.Sprintf("%d", i),
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				props := make(MessageHeaders)
				props["ordering_key"] = "test-order"

				_, err = client.Publish(ctx, topic, payload, props)
				require.NoError(t, err)
			}

			wg.Wait()

			// Verify message ordering
			mu.Lock()
			defer mu.Unlock()

			require.Equal(t, messageCount, len(receivedMessages))
			for i := 1; i < len(receivedMessages); i++ {
				require.True(t, receivedMessages[i] > receivedMessages[i-1],
					"Messages should be received in order")
			}
		})
	}
}

// TestFilteredSubscription tests message filtering with multiple conditions
func TestFilteredSubscription(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "filter")
			var receivedCount int32
			var wg sync.WaitGroup
			expectedMessages := 5
			wg.Add(expectedMessages)

			// Create filtered subscription
			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				atomic.AddInt32(&receivedCount, 1)
				ack()
				wg.Done()
				return nil
			}

			filter := func(ctx context.Context, event *MessageEvent) bool {
				// Filter based on multiple properties
				priority := event.Properties["priority"]
				category := event.Properties["category"]
				return priority == "high" && category == "important"
			}

			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Filter:   filter,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Send mix of messages
			totalMessages := 10
			for i := 0; i < totalMessages; i++ {
				props := make(MessageHeaders)

				// Alternate message properties
				if i%2 == 0 {
					props["priority"] = "high"
					props["category"] = "important"
				} else {
					props["priority"] = "low"
					props["category"] = "normal"
				}

				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: fmt.Sprintf("filtered-message-%d", i),
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				_, err = client.Publish(ctx, topic, payload, props)
				require.NoError(t, err)
			}

			// Wait for filtered messages
			wg.Wait()
			require.Equal(t, int32(expectedMessages), atomic.LoadInt32(&receivedCount))
		})
	}
}

// TestRequestResponseChain tests a chain of request-response operations
func TestRequestResponseChain(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			// Create topics for the chain
			topic1 := createTestTopic(t, client, "topic1")
			topic2 := createTestTopic(t, client, "topic2")
			topic3 := createTestTopic(t, client, "topic3")
			finalTopic := createTestTopic(t, client, "final")

			var wg sync.WaitGroup
			wg.Add(1)

			subIds := make(map[string]string)
			// Setup service 1: Receives on topic1, sends to topic2
			setupService := func(inTopic, outTopic string, transform func(TestMessage) TestMessage) {
				callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
					var msg TestMessage
					err := json.Unmarshal(event.Payload, &msg)
					require.NoError(t, err)

					// Transform and forward message
					transformed := transform(msg)
					payload, err := json.Marshal(transformed)
					require.NoError(t, err)

					props := make(MessageHeaders)
					props.SetCorrelationID(event.CoRelationID())

					_, err = client.Publish(ctx, outTopic, payload, props)
					require.NoError(t, err)

					ack()
					return nil
				}

				subID, err := client.Subscribe(ctx, SubscribeOptions{
					Topic:    inTopic,
					Callback: callback,
				})
				require.NoError(t, err)
				subIds[subID] = inTopic
			}
			defer func() {
				for subID, inTopic := range subIds {
					_ = client.UnSubscribe(ctx, inTopic, subID)
				}
			}()

			// Setup chain of services
			setupService(topic1, topic2, func(m TestMessage) TestMessage {
				m.Content = m.Content + "-service1"
				return m
			})

			setupService(topic2, topic3, func(m TestMessage) TestMessage {
				m.Content = m.Content + "-service2"
				return m
			})

			setupService(topic3, finalTopic, func(m TestMessage) TestMessage {
				m.Content = m.Content + "-service3"
				return m
			})

			// Setup final receiver
			var finalMessage TestMessage
			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				err := json.Unmarshal(event.Payload, &finalMessage)
				require.NoError(t, err)
				ack()
				wg.Done()
				return nil
			}

			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    finalTopic,
				Callback: callback,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, finalTopic, subID)
			}()

			// Register the chain
			initialMsg := TestMessage{
				ID:      ulid.Make().String(),
				Content: "initial",
				Time:    time.Now(),
			}
			payload, err := json.Marshal(initialMsg)
			require.NoError(t, err)

			_, err = client.Publish(ctx, topic1, payload, nil)
			require.NoError(t, err)

			// Wait for chain completion
			wg.Wait()

			// Verify final message
			require.Equal(t, initialMsg.ID, finalMessage.ID)
			require.Equal(t, "initial-service1-service2-service3", finalMessage.Content)
		})
	}
}

// TestRedeliveryWithBackoff tests message redelivery with exponential backoff
func TestRedeliveryWithBackoff(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			// Use longer timeout
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			started := time.Now()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			topic := createTestTopic(t, client, "backoff")
			var attempts int32
			var lastAttemptTime time.Time
			var mu sync.Mutex
			var wg sync.WaitGroup
			wg.Add(1)

			callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
				currentAttempt := atomic.AddInt32(&attempts, 1)

				mu.Lock()
				if !lastAttemptTime.IsZero() {
					elapsed := time.Since(lastAttemptTime)
					t.Logf("Attempt %d after %v", currentAttempt, elapsed)
				}
				lastAttemptTime = time.Now()
				mu.Unlock()

				if currentAttempt < 3 {
					t.Logf("nack [%s] attempt %d", time.Since(started), currentAttempt)
					time.Sleep(time.Second) // Add small delay before nack
					nack()
				} else {
					t.Logf("ack [%s] attempt %d", time.Since(started), currentAttempt)
					ack()
					wg.Done()
				}
				return nil
			}

			// Create subscriber
			props := make(MessageHeaders)
			groupID := fmt.Sprintf("backoff-group-%s", ulid.Make().String())
			props[groupKey] = groupID
			props["MaxRetries"] = "3"

			subID, err := client.Subscribe(ctx, SubscribeOptions{
				Topic:    topic,
				Callback: callback,
				Shared:   true, // Use shared subscription
				Group:    groupID,
				Props:    props,
			})
			require.NoError(t, err)
			defer func() {
				_ = client.UnSubscribe(ctx, topic, subID)
			}()

			// Give subscription time to be fully set up
			time.Sleep(100 * time.Millisecond)

			// Publish message
			msg := TestMessage{
				ID:      ulid.Make().String(),
				Content: "redelivery-test",
				Time:    time.Now(),
			}
			payload, err := json.Marshal(msg)
			require.NoError(t, err)

			_, err = client.Publish(ctx, topic, payload, props)
			require.NoError(t, err)

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				final := atomic.LoadInt32(&attempts)
				require.Equal(t, int32(3), final,
					"Expected exactly 3 delivery attempts, got %d", final)
			case <-ctx.Done():
				t.Fatalf("Test timed out. Made %d attempts",
					atomic.LoadInt32(&attempts))
			}
		})
	}
}

// TestPulsarSpecificFeatures tests Pulsar-specific features
func TestPulsarSpecificFeatures(t *testing.T) {
	for _, provider := range getTestProviders() {
		t.Run(string(provider), func(t *testing.T) {
			if provider != types.PulsarMessagingProvider {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			client, _, err := createTestClient(ctx, provider)
			require.NoError(t, err)
			defer client.Close()

			t.Run("ExclusiveSubscription", func(t *testing.T) {
				topic := createTestTopic(t, client, "exclusive")

				var wg sync.WaitGroup
				wg.Add(1)

				// First subscriber (exclusive)
				callback := func(ctx context.Context, event *MessageEvent, ack, nack AckHandler) error {
					ack()
					wg.Done()
					return nil
				}

				subID1, err := client.Subscribe(ctx, SubscribeOptions{
					Topic:    topic,
					Callback: callback,
					Shared:   false, // Exclusive subscription
					Props: MessageHeaders{
						"Group": "exclusive-Group",
					},
				})
				require.NoError(t, err)
				defer func() {
					_ = client.UnSubscribe(ctx, topic, subID1)
				}()

				// Second subscriber should fail (exclusive subscription)
				_, err = client.Subscribe(ctx, SubscribeOptions{
					Topic:    topic,
					Callback: callback,
					Shared:   false,
					Props: MessageHeaders{
						"Group": "exclusive-Group",
					},
				})
				require.Error(t, err)

				// Publish test message
				msg := TestMessage{
					ID:      ulid.Make().String(),
					Content: "exclusive test",
					Time:    time.Now(),
				}
				payload, err := json.Marshal(msg)
				require.NoError(t, err)

				_, err = client.Publish(ctx, topic, payload, nil)
				require.NoError(t, err)

				waitWithTimeout(t, &wg, ctx, "Waiting for exclusive subscription")
			})
		})
	}
}

// Additional helper functions

// Helper function for waiting with timeout
func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, ctx context.Context, message string) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Add separate timeout for wait
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case <-done:
		return
	case <-ctx.Done():
		t.Fatalf("%s: %v", message, ctx.Err())
	}
}

func toDurationSecs(secs int) *time.Duration {
	duration := time.Duration(secs) * time.Second
	return &duration
}

func createTestClient(ctx context.Context, provider types.MessagingProvider) (client Client,
	config *types.CommonConfig, err error) {
	testConfig := getTestConfig(provider)
	config = &types.CommonConfig{
		Debug: true,
		Queue: &types.QueueConfig{
			Provider:   testConfig.provider,
			Endpoints:  testConfig.bootstrapServers,
			RetryMax:   3,
			RetryDelay: toDurationSecs(1),
			Kafka: &types.KafkaConfig{
				ChannelBuffer: 10,
				CommitTimeout: 5 * time.Second,
				Group:         "test-default-Group",
			},
			MaxMessageSize:    1024 * 1024,
			MaxConnections:    100,
			ConnectionTimeout: toDurationSecs(10),
			OperationTimeout:  toDurationSecs(30),
			CommitTimeout:     toDurationSecs(2),
		},
		Redis: &types.RedisConfig{
			Host:       getEnvOrDefault("TEST_REDIS_HOST", "localhost"),
			Port:       getEnvIntOrDefault("TEST_REDIS_PORT", 6379),
			Password:   getEnvOrDefault("TEST_REDIS_PASSWORD", ""),
			PoolSize:   10,
			MaxPopWait: 5 * time.Second,
			TTLMinutes: 1,
		},
	}

	client, err = CreateClient(ctx, config)
	return
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getTestConfig(provider types.MessagingProvider) TestConfig {
	var servers string
	switch provider {
	case types.KafkaMessagingProvider:
		servers = os.Getenv("TEST_KAFKA_SERVERS")
		if servers == "" {
			servers = "localhost:9092"
		}
	case types.RedisMessagingProvider:
		host := getEnvOrDefault("TEST_REDIS_HOST", "localhost")
		port := getEnvIntOrDefault("TEST_REDIS_PORT", 6379)
		servers = fmt.Sprintf("%s:%d", host, port)
	case types.PulsarMessagingProvider:
		servers = os.Getenv("TEST_PULSAR_SERVERS")
		if servers == "" {
			servers = "pulsar://localhost:6650"
		}
	}

	return TestConfig{
		provider:          provider,
		bootstrapServers:  []string{servers},
		numPartitions:     1,
		replicationFactor: 1,
	}
}

func createTestTopic(t *testing.T, client Client, name string) string {
	topic := fmt.Sprintf("%s-%s", name, ulid.Make().String())

	err := client.CreateTopicIfNotExists(context.Background(), topic, &TopicConfig{
		NumPartitions:     1, // cfg.numPartitions,
		ReplicationFactor: 1, // cfg.replicationFactor,
		RetentionTime:     1 * time.Hour,
		Configs: map[string]string{
			"cleanup.policy":      "delete",
			"retention.ms":        "3600000", // 1 hour
			"min.insync.replicas": "1",
		},
	})
	require.NoError(t, err)
	// Wait for topic creation to complete
	time.Sleep(10 * time.Millisecond)
	return topic
}

func createSubscription(t *testing.T, ctx context.Context,
	client Client, opts SubscribeOptions) (string, func()) {
	if opts.Props == nil {
		opts.Props = make(MessageHeaders)
	}
	if opts.Group == "" {
		opts.Group = "test-Group-" + ulid.Make().String()
	}

	subID, err := client.Subscribe(ctx, opts)
	require.NoError(t, err)

	// Wait for subscription to be ready
	time.Sleep(1 * time.Millisecond)

	cleanup := func() {
		err := client.UnSubscribe(context.Background(), opts.Topic, subID)
		require.NoError(t, err)
	}

	return subID, cleanup
}

func publishTestMessage(t *testing.T, client Client,
	topic string, msg TestMessage, props MessageHeaders) []byte {
	payload, err := json.Marshal(msg)
	require.NoError(t, err)

	msgID, err := client.Publish(context.Background(), topic, payload, props)
	require.NoError(t, err)

	// Allow time for message processing
	time.Sleep(1 * time.Millisecond)

	return msgID
}

func cleanupSubscriptions(t *testing.T, client Client, topic string, subIDs []string) {
	for _, subID := range subIDs {
		err := client.UnSubscribe(context.Background(), topic, subID)
		require.NoError(t, err)
	}
}
