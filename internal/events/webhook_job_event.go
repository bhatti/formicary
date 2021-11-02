package events

import (
	"encoding/json"
	"fmt"
	"plexobject.com/formicary/internal/types"
)

// WebhookJobEvent is used to launch execution of a job
type WebhookJobEvent struct {
	*JobExecutionLifecycleEvent
	*types.Webhook
}

// NewWebhookJobEvent constructor
func NewWebhookJobEvent(
	base *JobExecutionLifecycleEvent,
	hook *types.Webhook,
) *WebhookJobEvent {
	return &WebhookJobEvent{
		JobExecutionLifecycleEvent: base,
		Webhook:                    hook,
	}
}

// UnmarshalWebhookJobEvent unmarshal
func UnmarshalWebhookJobEvent(b []byte) (*WebhookJobEvent, error) {
	var event WebhookJobEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for launching job
func (e *WebhookJobEvent) Validate() error {
	if e.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if e.JobRequestID == 0 {
		return fmt.Errorf("requestID is not specified")
	}
	if e.JobType == "" {
		return fmt.Errorf("jobType is not specified")
	}
	if e.Webhook == nil {
		return fmt.Errorf("webhook is not specified")
	}
	return nil
}

// Marshal serializes event
func (e *WebhookJobEvent) Marshal() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}
