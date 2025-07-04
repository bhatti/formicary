package tasklet

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

func (t *BaseTasklet) subscribeToIncomingRequests(ctx context.Context) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		req, err := types.UnmarshalTaskRequest(t.registration.EncryptionKey, event.Payload)
		if err != nil {
			return err
		}
		go func() {
			req.StartedAt = time.Now()
			req.CoRelationID = event.CoRelationID()
			if err := t.handleRequest(ctx, req, event.ReplyTopic()); err != nil {
				logrus.WithFields(
					logrus.Fields{
						"Component":       "BaseTasklet",
						"Tasklet":         t.ID,
						"RequestID":       req.JobRequestID,
						"JobType":         req.JobType,
						"TaskType":        req.TaskType,
						"TaskExecutionID": req.TaskExecutionID,
						"Params":          req.Variables,
						"Error":           err,
					}).Error("failed to handle request")
			}
		}()
		return nil
	}
	return t.QueueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    t.RequestTopic,
		Shared:   true,
		Callback: callback,
		Filter:   t.QueueFilter,
		Props:    make(map[string]string),
	})
}

func (t *BaseTasklet) subscribeToJobLifecycleEvent(
	ctx context.Context,
	subscriptionTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		jobExecutionLifecycleEvent, err := events.UnmarshalJobExecutionLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                  "BaseTasklet",
				"Tasklet":                    t.ID,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
				"Error":                      err,
			}).Error("failed to unmarshal jobExecutionLifecycleEvent")
			return err
		}

		// the base-tasklet will subscribe to messaging queue once for job-execution event and then
		// propagate via event bus to all request-executors that work on each request so each request doesn't
		// need to consume any queuing resources.
		t.EventBus.Publish(t.Config.GetJobExecutionLifecycleTopic(), jobExecutionLifecycleEvent)
		if jobExecutionLifecycleEvent.JobState == types.CANCELLED ||
			jobExecutionLifecycleEvent.JobState == types.PAUSED {
			if err := t.RequestRegistry.CancelJob(jobExecutionLifecycleEvent.JobRequestID); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                  "BaseTasklet",
					"Tasklet":                    t.ID,
					"ID":                         jobExecutionLifecycleEvent.ID,
					"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
					"Error":                      err}).Error("failed to cancel all tasks for job")
				return err
			}
		}
		return nil
	}
	return t.QueueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    subscriptionTopic,
		Shared:   false,
		Callback: callback,
		Filter:   t.QueueFilter,
		Props:    make(map[string]string),
	})
}

func (t *BaseTasklet) subscribeToTaskLifecycleEvent(ctx context.Context,
	subscriptionTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		taskExecutionLifecycleEvent, err := events.UnmarshalTaskExecutionLifecycleEvent(event.Payload)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                   "BaseTasklet",
				"Tasklet":                     t.ID,
				"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
				"Error":                       err}).
				Error("failed to unmarshal taskExecutionLifecycleEvent")
			return err
		}
		t.EventBus.Publish(t.Config.GetTaskExecutionLifecycleTopic(), taskExecutionLifecycleEvent)
		if taskExecutionLifecycleEvent.TaskState == types.CANCELLED ||
			taskExecutionLifecycleEvent.TaskState == types.PAUSED {
			if err = t.RequestRegistry.Cancel(taskExecutionLifecycleEvent.Key()); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":                   "BaseTasklet",
					"Tasklet":                     t.ID,
					"TaskExecutionLifecycleEvent": taskExecutionLifecycleEvent,
					"ID":                          taskExecutionLifecycleEvent.ID,
					"Error":                       err}).Error("failed to cancel task")
				return err
			}
		}
		return nil
	}
	return t.QueueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    subscriptionTopic,
		Shared:   false,
		Callback: callback,
		Filter:   t.QueueFilter,
		Props:    make(map[string]string),
	})
}

func (t *BaseTasklet) startTickerForRegistration(
	ctx context.Context) {
	// use registration as a form of heart-beat along with current load so that server can load balance
	t.registrationTicker = time.NewTicker(t.Config.RegistrationInterval)
	go func() {
		// continue sending registration while not shutdown
		for !t.Config.ShuttingDown {
			select {
			case <-ctx.Done():
				t.registrationTicker.Stop()
				return
			case <-t.done:
				t.registrationTicker.Stop()
				return
			case <-t.registrationTicker.C:
				if err := t.sendRegisterAntRequest(ctx); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":     "BaseTasklet",
						"RequestTopic":  t.RequestTopic,
						"CurrentLoad":   t.RequestRegistry.Count(),
						"TotalExecuted": t.totalExecuted,
						"Allocations":   t.RequestRegistry.GetAllocations(),
						"Tasklet":       t.ID,
						"Registration":  t.registration,
						"Error":         err}).
						Error("failed to send registration")
				} else {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(logrus.Fields{
							"Component":     "RegistrationHandler",
							"RequestTopic":  t.RequestTopic,
							"CurrentLoad":   t.RequestRegistry.Count(),
							"TotalExecuted": t.totalExecuted,
							"Allocations":   t.RequestRegistry.GetAllocations(),
							"Tasklet":       t.ID,
							"Registration":  t.registration,
						}).Debug("sent registration")
					}
				}
			}
		}

		logrus.WithFields(logrus.Fields{
			"Component":     "BaseTasklet",
			"RequestTopic":  t.RequestTopic,
			"CurrentLoad":   t.RequestRegistry.Count(),
			"TotalExecuted": t.totalExecuted,
			"Allocations":   t.RequestRegistry.GetAllocations(),
			"Tasklet":       t.ID,
			"Registration":  t.registration,
			"ShuttingDown":  t.Config.ShuttingDown}).
			Error("exiting registration loop")
	}()
}

// send ant registration periodically to let server know that the ant is alive, and it knows the load
// of the ant so that it can perform back-pressure if needed.
func (t *BaseTasklet) sendRegisterAntRequest(
	ctx context.Context) (err error) {
	t.registration.AntTopic = t.RequestTopic
	t.registration.CurrentLoad = t.RequestRegistry.Count()
	t.registration.TotalExecuted = t.totalExecuted
	t.registration.Allocations = t.RequestRegistry.GetAllocations() // get allocations based on current requests

	var b []byte
	// validate and marshal registration to be sent to server so that it can keep track of active ants
	if b, err = t.registration.Marshal(); err != nil {
		return err
	}
	if _, err = t.QueueClient.Publish(
		ctx,
		t.RegistrationTopic,
		b,
		make(map[string]string),
	); err != nil {
		return err
	}
	return
}
