package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// ErrorEvent is used to publish console logs
type ErrorEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	// Message
	Message string `json:"message"`
}

// NewErrorEvent constructor
func NewErrorEvent(
	source string,
	userID string,
	msg string) *ErrorEvent {
	return &ErrorEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "ErrorEvent",
			CreatedAt: time.Now(),
		},
		UserID:  userID,
		Message: msg,
	}
}

// String format
func (l *ErrorEvent) String() string {
	return l.Message
}

// Validate validates event for message event
func (l *ErrorEvent) Validate() error {
	if l.Message == "" {
		return fmt.Errorf("message is not specified")
	}
	return nil
}

// UnmarshalErrorEvent unmarshal
func UnmarshalErrorEvent(b []byte) (*ErrorEvent, error) {
	var event ErrorEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Marshal serializes event
func (l *ErrorEvent) Marshal() []byte {
	if err := l.Validate(); err != nil {
		return []byte(l.Message)
	}
	b, err := json.Marshal(l)
	if err == nil {
		return b
	}
	return []byte(l.Message)
}
