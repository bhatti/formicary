// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tracing"
	"plexobject.com/formicary/queen/types"
)

// S3NotificationSubscriber subscribes to an S3 event notification queue topic and
// fires a trigger for each matching object-created event.
type S3NotificationSubscriber struct {
	queueClient    queue.Client
	evaluator      *Evaluator
	submitter      *Submitter
	jobDef         *types.JobDefinition
	trigger        *types.TriggerDefinition
	subscriptionID string
}

// NewS3NotificationSubscriber creates and starts an S3NotificationSubscriber.
func NewS3NotificationSubscriber(
	ctx context.Context,
	queueClient queue.Client,
	evaluator *Evaluator,
	submitter *Submitter,
	jobDef *types.JobDefinition,
	trigger *types.TriggerDefinition,
) (*S3NotificationSubscriber, error) {
	s := &S3NotificationSubscriber{
		queueClient: queueClient,
		evaluator:   evaluator,
		submitter:   submitter,
		jobDef:      jobDef,
		trigger:     trigger,
	}
	if err := s.start(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Stop unsubscribes from the notification topic.
func (s *S3NotificationSubscriber) Stop(ctx context.Context) {
	if s.subscriptionID != "" {
		if err := s.queueClient.UnSubscribe(ctx, s.trigger.Topic, s.subscriptionID); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "S3NotificationSubscriber",
				"JobType":     s.jobDef.JobType,
				"TriggerName": s.trigger.Name,
			}).Warnf("failed to unsubscribe: %v", err)
		}
		s.subscriptionID = ""
	}
}

func (s *S3NotificationSubscriber) start(ctx context.Context) error {
	group := s.trigger.Group
	if group == "" {
		group = "formicary-trigger-s3-" + s.jobDef.JobType + "-" + s.trigger.Name
	}
	id, err := s.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    s.trigger.Topic,
		Shared:   s.trigger.Shared,
		Group:    group,
		Callback: s.handleNotification,
	})
	if err != nil {
		return err
	}
	s.subscriptionID = id
	logrus.WithFields(logrus.Fields{
		"Component":   "S3NotificationSubscriber",
		"JobType":     s.jobDef.JobType,
		"TriggerName": s.trigger.Name,
		"Topic":       s.trigger.Topic,
	}).Infof("subscribed to S3 notification topic")
	return nil
}

// s3NotificationEvent is a minimal S3 event notification envelope.
// Both AWS SNS-wrapped and direct S3 notification formats are handled.
type s3NotificationEvent struct {
	Records []struct {
		S3 struct {
			Bucket struct {
				Name string `json:"name"`
			} `json:"bucket"`
			Object struct {
				Key  string `json:"key"`
				Size int64  `json:"size"`
				ETag string `json:"eTag"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

func (s *S3NotificationSubscriber) handleNotification(ctx context.Context, event *queue.MessageEvent, ack queue.AckHandler, nack queue.AckHandler) error {
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.s3_notification",
		trace.WithAttributes(
			attribute.String("trigger.name", s.trigger.Name),
			attribute.String("job.type", s.jobDef.JobType),
		),
	)
	defer func() { span.End() }()

	var notif s3NotificationEvent
	if err := json.Unmarshal(event.Payload, &notif); err != nil {
		// Unrecognized payload format — ACK to avoid poison pill re-delivery.
		span.RecordError(err)
		logrus.WithFields(logrus.Fields{
			"Component":   "S3NotificationSubscriber",
			"JobType":     s.jobDef.JobType,
			"TriggerName": s.trigger.Name,
			"PayloadLen":  len(event.Payload),
		}).Warnf("unrecognized S3 notification payload, discarding: %v", err)
		ack()
		return nil
	}
	if len(notif.Records) == 0 {
		ack()
		return nil
	}

	// Process all records and always ACK. NACKing a partially-processed batch would cause
	// already-submitted records to be re-delivered and double-fired. Errors are logged for
	// alerting; operators should configure dedup_key on the trigger for idempotent re-delivery.
	var firstErr error
	for _, rec := range notif.Records {
		objData := map[string]interface{}{
			"Key":    rec.S3.Object.Key,
			"Bucket": rec.S3.Bucket.Name,
			"Size":   rec.S3.Object.Size,
			"ETag":   rec.S3.Object.ETag,
		}
		data := map[string]interface{}{
			"Object": objData,
		}
		result, err := s.evaluator.Evaluate(ctx, &TriggerEvent{
			JobDefinition: s.jobDef,
			Trigger:       s.trigger,
			Data:          data,
		})
		if err != nil {
			span.RecordError(err)
			logrus.WithFields(logrus.Fields{
				"Component":   "S3NotificationSubscriber",
				"JobType":     s.jobDef.JobType,
				"TriggerName": s.trigger.Name,
				"Key":         rec.S3.Object.Key,
			}).Errorf("evaluator error (record skipped): %v", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !result.Passed {
			continue
		}
		if _, err = s.submitter.Submit(ctx, s.jobDef, s.trigger.Name, result); err != nil {
			span.RecordError(err)
			logrus.WithFields(logrus.Fields{
				"Component":   "S3NotificationSubscriber",
				"JobType":     s.jobDef.JobType,
				"TriggerName": s.trigger.Name,
				"Key":         rec.S3.Object.Key,
			}).Errorf("submit error (record skipped): %v", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	ack()
	if firstErr != nil {
		span.SetStatus(codes.Error, firstErr.Error())
	}
	return nil
}
