package events

import (
	"encoding/json"
	"fmt"
	"plexobject.com/formicary/internal/types"
)

// WebhookTaskEvent is used to launch execution of a job
type WebhookTaskEvent struct {
	*TaskExecutionLifecycleEvent
	*types.Webhook
}

// NewWebhookTaskEvent constructor
func NewWebhookTaskEvent(
	base *TaskExecutionLifecycleEvent,
	hook *types.Webhook,
) *WebhookTaskEvent {
	return &WebhookTaskEvent{
		TaskExecutionLifecycleEvent: base,
		Webhook:                     hook,
	}
}

// UnmarshalWebhookTaskEvent unmarshal
func UnmarshalWebhookTaskEvent(b []byte) (*WebhookTaskEvent, error) {
	var event WebhookTaskEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for launching job
func (e *WebhookTaskEvent) Validate() error {
	if e.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if e.JobRequestID == 0 {
		return fmt.Errorf("requestID is not specified")
	}
	if e.TaskType == "" {
		return fmt.Errorf("taskType is not specified")
	}
	if e.TaskState == "" {
		return fmt.Errorf("task state is not specified")
	}
	if e.Webhook == nil {
		return fmt.Errorf("webhook is not specified")
	}
	return nil
}

// Marshal serializes event
func (e *WebhookTaskEvent) Marshal() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}
