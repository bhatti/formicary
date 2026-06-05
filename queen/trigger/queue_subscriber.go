// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tracing"
	"plexobject.com/formicary/queen/types"
)

// QueueSubscriber subscribes to a message queue topic and fires a trigger when messages arrive.
type QueueSubscriber struct {
	queueClient   queue.Client
	evaluator     *Evaluator
	submitter     *Submitter
	jobDef        *types.JobDefinition
	trigger       *types.TriggerDefinition
	subscriptionID string
}

// NewQueueSubscriber creates and starts a QueueSubscriber.
func NewQueueSubscriber(
	ctx context.Context,
	queueClient queue.Client,
	evaluator *Evaluator,
	submitter *Submitter,
	jobDef *types.JobDefinition,
	trigger *types.TriggerDefinition,
) (*QueueSubscriber, error) {
	qs := &QueueSubscriber{
		queueClient: queueClient,
		evaluator:   evaluator,
		submitter:   submitter,
		jobDef:      jobDef,
		trigger:     trigger,
	}
	if err := qs.start(ctx); err != nil {
		return nil, err
	}
	return qs, nil
}

// Stop unsubscribes from the queue topic.
func (qs *QueueSubscriber) Stop(ctx context.Context) {
	if qs.subscriptionID != "" {
		if err := qs.queueClient.UnSubscribe(ctx, qs.trigger.Topic, qs.subscriptionID); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "QueueSubscriber",
				"JobType":     qs.jobDef.JobType,
				"TriggerName": qs.trigger.Name,
				"Topic":       qs.trigger.Topic,
			}).Warnf("failed to unsubscribe: %v", err)
		}
		qs.subscriptionID = ""
	}
}

func (qs *QueueSubscriber) start(ctx context.Context) error {
	group := qs.trigger.Group
	if group == "" {
		group = fmt.Sprintf("formicary-trigger-%s-%s", qs.jobDef.JobType, qs.trigger.Name)
	}

	id, err := qs.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    qs.trigger.Topic,
		Shared:   qs.trigger.Shared,
		Group:    group,
		Callback: qs.handleMessage,
	})
	if err != nil {
		return fmt.Errorf("queue trigger %q: failed to subscribe to topic %q: %w", qs.trigger.Name, qs.trigger.Topic, err)
	}
	qs.subscriptionID = id

	logrus.WithFields(logrus.Fields{
		"Component":      "QueueSubscriber",
		"JobType":        qs.jobDef.JobType,
		"TriggerName":    qs.trigger.Name,
		"Topic":          qs.trigger.Topic,
		"SubscriptionID": id,
	}).Infof("subscribed to queue topic for trigger")
	return nil
}

func (qs *QueueSubscriber) handleMessage(ctx context.Context, event *queue.MessageEvent, ack queue.AckHandler, nack queue.AckHandler) error {
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.queue_message",
		trace.WithAttributes(
			attribute.String("trigger.name", qs.trigger.Name),
			attribute.String("job.type", qs.jobDef.JobType),
			attribute.String("queue.topic", qs.trigger.Topic),
		),
	)
	defer func() { span.End() }()

	var msgData interface{}
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &msgData); err != nil {
			// Non-JSON payload: expose as raw string.
			msgData = string(event.Payload)
		}
	}

	// Build template context.
	props := make(map[string]string)
	for k, v := range event.Properties {
		props[k] = v
	}
	data := map[string]interface{}{
		"Message":    msgData,
		"Properties": props,
	}

	result, err := qs.evaluator.Evaluate(ctx, &TriggerEvent{
		JobDefinition: qs.jobDef,
		Trigger:       qs.trigger,
		Data:          data,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		nack()
		return err
	}
	if !result.Passed {
		ack()
		return nil
	}

	if _, err = qs.submitter.Submit(ctx, qs.jobDef, qs.trigger.Name, result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		nack()
		return err
	}
	ack()
	return nil
}
