package events

import (
	"encoding/json"
	"fmt"
	"github.com/oklog/ulid/v2"
	"time"
)

// HealthErrorEvent is used to notify health errors
type HealthErrorEvent struct {
	BaseEvent
	// Error to notify
	Error string `json:"error"`
}

// NewHealthErrorEvent constructor
func NewHealthErrorEvent(
	source string,
	err string) *HealthErrorEvent {
	return &HealthErrorEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "HealthErrorEvent",
			CreatedAt: time.Now(),
		},
		Error: err,
	}
}

// String format
func (h *HealthErrorEvent) String() string {
	return h.Error
}

// UnmarshalHealthErrorEvent unmarshal
func UnmarshalHealthErrorEvent(b []byte) (*HealthErrorEvent, error) {
	var event HealthErrorEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event
func (h *HealthErrorEvent) Validate() error {
	if h.Error == "" {
		return fmt.Errorf("error is not specified")
	}
	return nil
}

// Marshal serializes event
func (h *HealthErrorEvent) Marshal() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(h)
}
