package queue

import (
	"context"
	"fmt"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
	"runtime/debug"
	"time"
)

// validateSubscribeOptions validates subscription options
func validateSubscribeOptions(opts *SubscribeOptions) error {
	if opts == nil {
		return fmt.Errorf("subscription options cannot be nil")
	}

	// Validate topic
	if err := validateTopic(opts.Topic); err != nil {
		return fmt.Errorf("invalid topic: %w", err)
	}

	// Validate callback
	if opts.Callback == nil {
		debug.PrintStack()
		return fmt.Errorf("callback function is required")
	}

	// Validate properties
	if err := validateProperties(opts.Props); err != nil {
		return fmt.Errorf("invalid properties: %w", err)
	}

	// Validate Group if specified
	if group := opts.Group; group != "" {
		if err := validateGroupName(group); err != nil {
			return fmt.Errorf("invalid Group name: %w", err)
		}
	}

	return nil
}

// validateTopic validates topic naming conventions
func validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}

	// Kafka topic naming rules:
	// - Can't be longer than 249 characters
	// - Can only contain ASCII alphanumerics, '.', '_', '-'
	// - Can't be '.' or '..'
	if len(topic) > 249 {
		return fmt.Errorf("topic name cannot be longer than 249 characters")
	}

	if topic == "." || topic == ".." {
		return fmt.Errorf("invalid topic name: %s", topic)
	}

	for _, char := range topic {
		if !isValidTopicNameChar(char) {
			return fmt.Errorf("invalid character in topic name: %c", char)
		}
	}

	return nil
}

// validateGroupName validates consumer Group naming conventions
func validateGroupName(group string) error {
	if group == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	// Similar rules to topic names but slightly more permissive
	if len(group) > 249 {
		return fmt.Errorf("group name cannot be longer than 249 characters")
	}

	for _, char := range group {
		if !isValidGroupNameChar(char) {
			return fmt.Errorf("invalid character in Group name: %c", char)
		}
	}

	return nil
}

// validateProperties validates message properties/headers
func validateProperties(props MessageHeaders) error {
	if props == nil {
		return nil // Empty properties are allowed
	}

	for key, value := range props {
		if err := validateHeaderKey(key); err != nil {
			return fmt.Errorf("invalid header key '%s': %w", key, err)
		}
		if err := validateHeaderValue(value); err != nil {
			return fmt.Errorf("invalid header value for key '%s': %w", key, err)
		}
	}

	// Validate special properties
	if corrID := props.GetCorrelationID(); corrID != "" {
		if len(corrID) > 128 {
			return fmt.Errorf("correlation ID too long (max 128 characters)")
		}
	}

	if replyTo := props.GetReplyTopic(); replyTo != "" {
		if err := validateTopic(replyTo); err != nil {
			return fmt.Errorf("invalid reply topic: %w", err)
		}
	}

	return nil
}

// validateHeaderKey validates a message header key
func validateHeaderKey(key string) error {
	if key == "" {
		return fmt.Errorf("header key cannot be empty")
	}

	if len(key) > 256 {
		return fmt.Errorf("header key too long (max 256 characters)")
	}

	// Header keys should be valid ASCII
	for _, char := range key {
		if char > 127 {
			return fmt.Errorf("header key must contain only ASCII characters")
		}
	}

	return nil
}

// validateHeaderValue validates a message header value
func validateHeaderValue(value string) error {
	// Kafka has a default message.max.bytes setting of 1MB
	// Headers should be reasonably smaller
	if len(value) > 32*1024 { // 32KB limit for header values
		return fmt.Errorf("header value too long (max 32KB)")
	}

	return nil
}

// Helper functions for character validation
func isValidTopicNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '-'
}

func isValidGroupNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '-'
}

// Additional validation helper methods
func validateMessageSize(payload []byte, config *types.QueueConfig) error {
	if len(payload) > int(config.MaxMessageSize) {
		return fmt.Errorf("message size %d exceeds maximum allowed size %d",
			len(payload), config.MaxMessageSize)
	}
	return nil
}

func validateSendRequest(topic string, payload []byte, props MessageHeaders, config *types.QueueConfig) error {
	if err := validateTopic(topic); err != nil {
		return err
	}
	if len(payload) == 0 {
		debug.PrintStack()
		return fmt.Errorf("payload cannot be empty")
	}
	if err := validateMessageSize(payload, config); err != nil {
		return err
	}

	if err := validateProperties(props); err != nil {
		return err
	}
	return nil
}

// recoverNilMessage handles panics from nil message pointers
func recoverNilMessage(topic string, id string) {
	if r := recover(); r != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "QueueClient",
			"Topic":     topic,
			"ID":        id,
			"Recover":   r,
		}).Error("Recovered from panic")
	}
}

// getChannelBuffer returns buffer size from config or default
func getChannelBuffer(config *types.QueueConfig) int {
	if config.Kafka.ChannelBuffer > 0 {
		return config.Kafka.ChannelBuffer
	}
	return defaultQueueCapacity
}

func validateSendReceiveRequest(req *SendReceiveRequest, config *types.QueueConfig) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if err := validateTopic(req.OutTopic); err != nil {
		return fmt.Errorf("invalid out topic: %w", err)
	}
	if err := validateTopic(req.InTopic); err != nil {
		return fmt.Errorf("invalid in topic: %w", err)
	}
	return validateSendRequest(req.OutTopic, req.Payload, req.Props, config)
}

func prepareProps(req *SendReceiveRequest) MessageHeaders {
	props := req.Props
	if props == nil {
		props = make(MessageHeaders)
	}
	props.SetCorrelationID(ulid.Make().String())
	props.SetReplyTopic(req.InTopic)
	props.SetDisableBatching(true)
	return props
}

func createTimeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = getDefaultTimeout()
	}
	return context.WithTimeout(ctx, timeout)
}

func getDefaultTimeout() time.Duration {
	return 30 * time.Second // Could be made configurable
}

func getRetryDelay(attempt int) time.Duration {
	return time.Duration(attempt+1) * time.Second // Could use exponential backoff
}
