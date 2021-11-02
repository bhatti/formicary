package webhook

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"sync/atomic"
)

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

func (p *Processor) subscribeToJobWebhookLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	return p.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			webhookEvent, err := events.UnmarshalWebhookJobEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":       "WebhookProcessor",
					"WebhookJobEvent": webhookEvent,
					"JobsProcessed":   p.jobsProcessed,
					"Error":           err}).
					Error("failed to unmarshal WebhookJobEvent")
				return err
			}
			if body, err := json.Marshal(webhookEvent.JobExecutionLifecycleEvent); err == nil {
				var resp []byte
				if resp, _, err = p.http.PostJSON(ctx, webhookEvent.URL, webhookEvent.Headers, webhookEvent.Query, body); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":       "WebhookProcessor",
						"WebhookJobEvent": webhookEvent,
						"JobsProcessed":   p.jobsProcessed,
						"Error":           err}).
						Error("failed to handle WebhookJobEvent")
					return err
				}
				atomic.AddInt64(&p.jobsProcessed, 1)
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component":       "WebhookProcessor",
						"WebhookJobEvent": webhookEvent,
						"JobsProcessed":   p.jobsProcessed,
						"Response":        string(resp),
					}).
						Debugf("processed WebhookJobEvent")
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"Component":       "WebhookProcessor",
					"WebhookJobEvent": webhookEvent,
					"JobsProcessed":   p.jobsProcessed,
					"Error":           err}).
					Error("failed to marshal WebhookJobEvent")
				return err
			}
			return nil
		},
		make(map[string]string),
	)
}

func (p *Processor) subscribeToTaskWebhookLifecycleEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	return p.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			webhookEvent, err := events.UnmarshalWebhookTaskEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":        "WebhookProcessor",
					"WebhookTaskEvent": webhookEvent,
					"TasksProcessed":   p.tasksProcessed,
					"Error":            err}).
					Error("failed to unmarshal WebhookTaskEvent")
				return err
			}
			if body, err := json.Marshal(webhookEvent.TaskExecutionLifecycleEvent); err == nil {
				var resp []byte
				if resp, _, err = p.http.PostJSON(ctx, webhookEvent.URL, webhookEvent.Headers, webhookEvent.Query, body); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":        "WebhookProcessor",
						"WebhookTaskEvent": webhookEvent,
						"TasksProcessed":   p.tasksProcessed,
						"Error":            err}).
						Error("failed to handle WebhookTaskEvent")
					return err
				}
				atomic.AddInt64(&p.tasksProcessed, 1)
				if logrus.IsLevelEnabled(logrus.DebugLevel) {
					logrus.WithFields(logrus.Fields{
						"Component":        "WebhookProcessor",
						"WebhookTaskEvent": webhookEvent,
						"TasksProcessed":   p.tasksProcessed,
						"Response":         string(resp),
					}).
						Debugf("processed WebhookTaskEvent")
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"Component":        "WebhookProcessor",
					"WebhookTaskEvent": webhookEvent,
					"TasksProcessed":   p.tasksProcessed,
					"Error":            err}).
					Error("failed to marshal WebhookTaskEvent")
				return err
			}
			return nil
		},
		make(map[string]string),
	)
}
