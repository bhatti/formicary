package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
)

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// Start background routine to clean up any stale leases
func (gw *Gateway) startReaperTicker(ctx context.Context) {
	gw.ticker = time.NewTicker(gw.serverCfg.Jobs.OrphanRequestsTimeout / 2)
	go func() {
		for {
			select {
			case <-ctx.Done():
				gw.ticker.Stop()
				return
			case <-gw.ticker.C:
				gw.reapStaleLeases(ctx)
			}
		}
	}()
}

// release websocket connections that have not renewed their leases
func (gw *Gateway) reapStaleLeases(_ context.Context) (count int) {
	now := time.Now()
	for _, lease := range gw.registry.getAllLeases() {
		if time.Duration(now.Unix()-lease.updatedAt.Unix()) * time.Second > gw.serverCfg.Jobs.OrphanRequestsTimeout {
			_ = gw.registry.Remove(lease)
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Key":       lease.Key(),
			}).Info("removing stale lease")
			count++
		}
	}
	return
}

func (gw *Gateway) subscribeToJobDefinitionLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			jobDefinitionLifecycleEvent, err := events.UnmarshalJobDefinitionLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                   "WebsocketGateway",
					"JobDefinitionLifecycleEvent": jobDefinitionLifecycleEvent,
					"Target":                      gw.id,
					"Error":                       err}).
					Error("failed to unmarshal JobDefinitionLifecycleEvent")
				return err
			}
			// notify subscribers who are interested in changes to job-definitions
			gw.registry.Notify(
				jobDefinitionLifecycleEvent.UserID,
				"JobDefinitionLifecycleEvent",
				"", // no scope
				event.Payload,
			)
			return nil
		},
		make(map[string]string),
	)
}

func (gw *Gateway) subscribeToJobRequestLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			jobRequestLifecycleEvent, err := events.UnmarshalJobRequestLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                "WebsocketGateway",
					"JobRequestLifecycleEvent": jobRequestLifecycleEvent,
					"Target":                   gw.id,
					"Error":                    err}).
					Error("failed to unmarshal JobRequestLifecycleEvent")
				return err
			}
			// notify subscribers who are interested in changes to job-request
			gw.registry.Notify(
				jobRequestLifecycleEvent.UserID,
				"JobRequestLifecycleEvent",
				"", // no scope
				event.Payload)
			return nil
		},
		make(map[string]string),
	)
}

func (gw *Gateway) subscribeToJobExecutionLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			jobExecutionLifecycleEvent, err := events.UnmarshalJobExecutionLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                  "WebsocketGateway",
					"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
					"Target":                     gw.id,
					"Error":                      err}).
					Error("failed to unmarshal JobExecutionLifecycleEvent")
				return err
			}
			// notify subscribers who are interested in changes to start/end events of job-execution
			gw.registry.Notify(
				jobExecutionLifecycleEvent.UserID,
				"JobExecutionLifecycleEvent",
				"", // no scope
				event.Payload)
			return nil
		},
		make(map[string]string),
	)
}

func (gw *Gateway) subscribeToTaskExecutionLifecycleEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			taskExecutionLifecycleEvent, err := events.UnmarshalTaskExecutionLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                   "WebsocketGateway",
					"Target":                      gw.id,
					"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
					"Error":                       err,
				}).
					Error("failed to unmarshal TaskExecutionLifecycleEvent")
				return err
			}
			gw.registry.Notify(
				taskExecutionLifecycleEvent.UserID,
				"TaskExecutionLifecycleEvent",
				fmt.Sprintf("%d", taskExecutionLifecycleEvent.JobRequestID), // scope is request-id
				event.Payload)
			return nil
		},
		make(map[string]string),
	)
}

func (gw *Gateway) subscribeToLogEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			logEvent, err := events.UnmarshalLogEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Target":    gw.id,
					"LogEvent":  logEvent,
					"Error":     err,
				}).
					Error("failed to unmarshal LogEvent")
				return err
			}
			gw.registry.Notify(
				logEvent.UserID,
				"LogEvent",
				fmt.Sprintf("%d", logEvent.JobRequestID), // scope is request-id
				event.Payload)
			if _, err = gw.logsArchiver.Save(logEvent); err != nil {
				// ignore error
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Target":    gw.id,
					"LogEvent":  logEvent,
					"Error":     err,
				}).Warn("failed to archive LogEvent")
			}
			return nil
		},
		make(map[string]string),
	)
}

func (gw *Gateway) subscribeToHealthErrorEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		subscriptionTopic,
		false, // exclusive subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			healthEvent, err := events.UnmarshalHealthErrorEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":        "WebsocketGateway",
					"Target":           gw.id,
					"HealthErrorEvent": healthEvent,
					"Error":            err,
				}).
					Error("ErrorEvent")
				return err
			}
			gw.registry.Notify(
				"",
				"HealthErrorEvent",
				"", // no scope
				event.Payload)
			return nil
		},
		make(map[string]string),
	)
}
func (gw *Gateway) subscribeToContainersLifecycleEvents(
	ctx context.Context,
	containerTopic string) (string, error) {
	return gw.queueClient.Subscribe(
		ctx,
		containerTopic,
		false, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			containerEvent, err := events.UnmarshalContainerLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Target":    gw.id,
					"Payload":   string(event.Payload),
					"Error":     err,
				}).
					Error("failed to unmarshal registration by event gateway")
				return err
			}
			gw.registry.Notify(
				containerEvent.UserID,
				"ContainerLifecycleEvent",
				"", // no scope
				event.Payload)
			return nil
		},
		make(map[string]string),
	)
}
