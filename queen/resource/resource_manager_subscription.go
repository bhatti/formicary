package resource

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
)

func (rm *ManagerImpl) subscribeToRegistration(
	ctx context.Context,
	registrationTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		registration, err := common.UnmarshalAntRegistration(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":         "ResourceManager",
				"Registration":      registration,
				"RegistrationTopic": registrationTopic,
				"Payload":           string(event.Payload),
				"Target":            rm.id,
				"Error":             err}).Error("failed to unmarshal registration by resource manager")
			return err
		}
		// OrgID is set server-side from the ant's JWT by the WebSocket layer (never by the ant).
		// For non-WebSocket transports (Redis, Pulsar, Kafka) OrgID stays "" when auth is
		// disabled — consistent with the no-op guarantee.
		if orgID := event.Properties[queue.AntOrgIDHeader]; orgID != "" {
			registration.OrgID = orgID
			// Append org-id to AntID so multi-user deployments never collide even when
			// two users run an ant with the same hostname/pod name.
			// Format: "<ant-id>@<org-id>" — readable in the dashboard, stable across restarts.
			if !containsOrgSuffix(registration.AntID, orgID) {
				registration.AntID = registration.AntID + "@" + orgID
			}
		}
		if err := rm.Register(ctx, registration); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":    "ResourceManager",
				"Registration": registration,
				"Target":       rm.id,
				"Error":        err}).Error("failed to Register ant")
		}
		return nil
	}
	return rm.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    registrationTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}

func (rm *ManagerImpl) subscribeToJobLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		jobExecutionLifecycleEvent, err := events.UnmarshalJobExecutionLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                  "ResourceManager",
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
				"Target":                     rm.id,
				"Error":                      err}).Error("failed to unmarshal jobExecutionLifecycleEvent")
			return err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":                  "ResourceManager",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Target":                     rm.id,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
			}).Debug("received job lifecycle event")
		}
		if jobExecutionLifecycleEvent.JobState.IsTerminal() {
			if err := rm.ReleaseJobResources(jobExecutionLifecycleEvent.JobRequestID); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                  "ResourceManager",
					"ID":                         jobExecutionLifecycleEvent.ID,
					"Target":                     rm.id,
					"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
					"Error":                      err}).Error("failed to release ant for job")
				return err
			}
		}
		return nil
	}
	return rm.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    subscriptionTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}

func (rm *ManagerImpl) subscribeToTaskLifecycleEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		taskExecutionLifecycleEvent, err := events.UnmarshalTaskExecutionLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                   "ResourceManager",
				"Target":                      rm.id,
				"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
				"Error":                       err,
			}).
				Error("failed to unmarshal taskExecutionLifecycleEvent")
			return err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":                   "ResourceManager",
				"ID":                          taskExecutionLifecycleEvent.ID,
				"Target":                      rm.id,
				"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
			}).Debug("received task lifecycle event")
		}
		// release resources if task is completed but let's wait until job completion if task failed because
		// it can be retried
		if taskExecutionLifecycleEvent.TaskState.Completed() {
			if err = rm.Release(common.NewAntReservation(
				taskExecutionLifecycleEvent.AntID,
				"",
				taskExecutionLifecycleEvent.JobRequestID,
				taskExecutionLifecycleEvent.TaskType,
				"", // enc-key
				0,
				0,
			)); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                   "ResourceManager",
					"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
					"AntID":                       taskExecutionLifecycleEvent.AntID,
					"ID":                          taskExecutionLifecycleEvent.ID,
					"Target":                      rm.id,
					"Error":                       err}).Error("failed to release ant for task")
				return err
			}
		}
		return nil
	}
	return rm.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    subscriptionTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}

// containsOrgSuffix returns true if antID already ends with "@<orgID>",
// preventing double-suffix on re-registration heartbeats.
func containsOrgSuffix(antID, orgID string) bool {
	return strings.HasSuffix(antID, "@"+orgID)
}

func (rm *ManagerImpl) subscribeToContainersLifecycleEvents(
	ctx context.Context,
	containerTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		containerEvent, err := events.UnmarshalContainerLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":      "ResourceManager",
				"ContainerTopic": containerTopic,
				"ContainerEvent": containerEvent,
				"Target":         rm.id,
				"Error":          err}).
				Error("failed to unmarshal registration by resource-manager")
			return err
		}
		rm.state.updateContainer(containerEvent)
		return nil
	}
	return rm.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    containerTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}
