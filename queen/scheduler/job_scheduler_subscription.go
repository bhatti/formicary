package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"time"

	"plexobject.com/formicary/internal/math"
	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
)

// Start periodic background process to fire a scheduler leader event so that only leader schedules job
func (js *JobScheduler) startTickerToSendJobSchedulerLeaderEvents(ctx context.Context) *time.Ticker {
	// use registration as a form of heart-beat along with current load so that server can load balance
	ticker := time.NewTicker(js.serverCfg.Jobs.JobSchedulerLeaderInterval)
	go func() {
		for !js.isStopped() {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-js.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := js.sendJobSchedulerLeaderEvent(ctx); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "JobScheduler",
						"ID":        js.serverCfg.ID,
						"Error":     err,
					}).
						Warn("failed to send job scheduler leader event")
				}
			}
		}
	}()
	_ = js.sendJobSchedulerLeaderEvent(ctx)
	return ticker
}

func (js *JobScheduler) startTickerToSchedulePendingJobs(ctx context.Context) *time.Ticker {
	ticker := time.NewTicker(js.serverCfg.Jobs.JobSchedulerCheckPendingJobsInterval)
	go func() {
		for !js.isStopped() {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-js.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if js.busy {
					continue
				} else if err := js.schedulePendingJobs(ctx); err != nil {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(logrus.Fields{
							"Component": "JobScheduler",
							"ID":        js.serverCfg.ID,
							"Error":     err,
						}).Debug("failed to schedule pending jobs")
					}
				}
			}
		}
	}()
	return ticker
}

// startTickerToCheckMissingCronJobs schedules cron jobs that somehow failed to schedule
func (js *JobScheduler) startTickerToCheckMissingCronJobs(ctx context.Context) *time.Ticker {
	ticker := time.NewTicker(js.serverCfg.Jobs.MissingCronJobsInterval)
	go func() {
		for !js.isStopped() {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-js.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := js.scheduleCronTriggeredJobs(ctx); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "JobScheduler",
						"Error":     err,
					}).Warn("failed to schedule cron triggered jobs")
				}
			}
		}
	}()
	return ticker
}

// startTickerToCheckOrphanJobs checks any READY jobs that have not started and put it back to PENDING
func (js *JobScheduler) startTickerToCheckOrphanJobs(ctx context.Context) *time.Ticker {
	ticker := time.NewTicker(js.serverCfg.Jobs.OrphanRequestsUpdateInterval)
	go func() {
		for !js.isStopped() {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-js.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := js.scheduleOrphanJobs(ctx); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "JobScheduler",
						"Error":     err,
					}).Warn("failed to schedule orphan jobs")
				}
			}
		}
	}()
	return ticker
}

// Subscribing to job-scheduler event in failover mode
// TODO Failover mode is not working and is sending events to multiple subscribers as opposed to exclusive
func (js *JobScheduler) subscribeToJobSchedulerLeader(ctx context.Context) (string, error) {
	return js.queueClient.Subscribe(
		ctx,
		js.jobSchedulerLeaderTopic,
		false, // exclusive subscription with failover
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			var jobSchedulerLeaderEvent events.JobSchedulerLeaderEvent
			if err := json.Unmarshal(event.Payload, &jobSchedulerLeaderEvent); err != nil {
				logrus.WithFields(logrus.Fields{
					"jobSchedulerLeaderEvent": jobSchedulerLeaderEvent,
					"payload":                 string(event.Payload),
					"error":                   err,
				}).Error("failed to unmarshal job scheduler leader event")
			}

			js.lastJobSchedulerLeaderEventAt = time.Now()
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"jobSchedulerLeaderEvent": jobSchedulerLeaderEvent,
					"totalPendingJobs":        js.totalPendingJobs,
					"totalScheduledJobs":      js.totalScheduledJobs,
					"noJobsTries":             js.noJobsTries,
				}).Debug("received job scheduler leader event")
			}

			return nil
		},
		nil,
		make(map[string]string),
	)
}

func (js *JobScheduler) unsubscribeToJobSchedulerLeader(ctx context.Context) (err error) {
	return js.queueClient.UnSubscribe(
		ctx,
		js.jobSchedulerLeaderTopic,
		js.jobSchedulerLeaderSubscriptionID)
}

// Sending an event that only one of leader will receive and that leader will be come official job scheduler
// If leader dies, the failover server will take over and become job scheduler
// TODO remove this
func (js *JobScheduler) sendJobSchedulerLeaderEvent(ctx context.Context) (err error) {
	event := js.serverCfg.NewJobSchedulerLeaderEvent()

	var b []byte
	b, err = json.Marshal(event)
	if err != nil {
		logrus.WithFields(
			logrus.Fields{"jobSchedulerLeaderEvent": event, "error": err}).
			Error("failed to serialize job scheduler leader event")
	} else {
		_, err = js.queueClient.Publish(
			ctx,
			js.jobSchedulerLeaderTopic,
			b,
			queue.NewMessageHeaders(queue.DisableBatchingKey, "true"),
		)
		if err != nil {
			logrus.WithFields(
				logrus.Fields{"jobSchedulerLeaderEvent": event, "error": err}).
				Errorf("failed to send job scheduler leader event to %s",
					js.jobSchedulerLeaderTopic)
		}
	}
	return
}

// This method reschedules jobs if the server dies in middle of processing and put it back to PENDING
func (js *JobScheduler) scheduleOrphanJobs(_ context.Context) (err error) {
	total, err := js.jobManager.RequeueOrphanJobRequests(
		js.serverCfg.Jobs.OrphanRequestsTimeout)
	if err != nil {
		return err
	}
	if total > 0 {
		logrus.WithFields(logrus.Fields{
			"Component": "JobScheduler",
			"ID":        js.serverCfg.ID,
			"Total":     total,
		}).Warn("requeued orphan jobs")
	}
	return nil
}

const cronTriggeredJobChunkSize = 100

// This method scheduled job requests for job definitions that have cron trigger but for whatever reason, the request
// wasn't added because scheduled requests will be added automatically when job definition is updated or when
// previous request is completed or failed.
func (js *JobScheduler) scheduleCronTriggeredJobs(_ context.Context) (err error) {
	jobTypes, err := js.jobManager.GetCronTriggeredJobTypes()
	if err != nil {
		return fmt.Errorf("failed to load jobtypes that can be triggered via cron due to %s", err)
	}

	for i := 0; i < len(jobTypes); i += cronTriggeredJobChunkSize {
		batchedJobTypesAndTrigger := jobTypes[i:math.Min(i+cronTriggeredJobChunkSize, len(jobTypes))]
		missingJobTypes, err := js.jobManager.FindMissingCronScheduledJobsByType(batchedJobTypesAndTrigger)

		if err != nil {
			return fmt.Errorf("failed to find job types that needs to be scheduled via cron due to %s",
				err)
		}
		for _, missingJobType := range missingJobTypes {
			// by passing query context for internal use
			jobDefinition, err := js.jobManager.GetJobDefinitionByType(
				common.NewQueryContextFromIDs(missingJobType.UserID, missingJobType.OrganizationID),
				missingJobType.JobType,
				"")
			if err != nil {
				continue
			}
			request, err := types.NewJobRequestFromDefinition(jobDefinition)
			if err != nil {
				continue
			}
			request, err = js.jobManager.SaveJobRequest(
				common.NewQueryContextFromIDs(missingJobType.UserID, missingJobType.OrganizationID), request)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":             "JobScheduler",
					"ID":                    js.serverCfg.ID,
					"MissingJobType":        missingJobType.JobType,
					"MissingJobTypeUserKey": missingJobType.UserKey,
					"MissingJobTypeUserID":  missingJobType.UserID,
					"Request":               request,
					"Error":                 err,
				}).Error("failed to schedule missing job request")
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "JobScheduler",
					"ID":        js.serverCfg.ID,
					"JobType":   missingJobType,
					"Request":   request,
				}).Warn("scheduling missing job request")
			}
		}
	}
	return nil
}
