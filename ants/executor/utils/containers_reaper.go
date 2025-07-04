package utils

import (
	"context"
	"plexobject.com/formicary/internal/ant_config"
	"strings"
	"time"

	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/web"

	log "github.com/sirupsen/logrus"
)

// ContainersReaper struct
type ContainersReaper struct {
	antCfg                              *ant_config.AntConfig
	queueClient                         queue.Client
	httpClient                          web.HTTPClient
	metricsRegistry                     *metrics.Registry
	recentlyCompletedJobIDs             *cutils.LRU
	reaperTicker                        *time.Ticker
	recentlyCompletedJobIDsSubscriberID string
	stopped                             bool
}

// NewContainersReaper constructor
func NewContainersReaper(
	antCfg *ant_config.AntConfig,
	queueClient queue.Client,
	httpClient web.HTTPClient,
	metricsRegistry *metrics.Registry,
) *ContainersReaper {
	return &ContainersReaper{
		antCfg:                  antCfg,
		recentlyCompletedJobIDs: cutils.NewLRU(10000, nil),
		queueClient:             queueClient,
		httpClient:              httpClient,
		metricsRegistry:         metricsRegistry,
	}
}

// Start reaperTicker for reaping
func (r *ContainersReaper) Start(ctx context.Context) (err error) {
	r.reaperTicker = time.NewTicker(r.antCfg.Common.ContainerReaperInterval)
	go func() {
		for !r.stopped {
			select {
			case <-r.reaperTicker.C:
				r.reap(ctx)
			case <-ctx.Done():
				r.reaperTicker.Stop()
				return
			}
		}
	}()
	if r.recentlyCompletedJobIDsSubscriberID, err = r.subscribeToRecentlyCompletedJobIDs(ctx,
		r.antCfg.Common.GetRecentlyCompletedJobsTopic()); err != nil {
		_ = r.Stop(ctx)
		return err
	}
	log.WithFields(
		log.Fields{
			"Component": "ContainersReaper",
			"Timeout":   r.antCfg.Common.ContainerReaperInterval,
			"Memory":    cutils.MemUsageMiBString(),
			"Error":     err,
		}).Infof("started reap container")
	return
}

// Stop reaperTicker for reaping
func (r *ContainersReaper) Stop(ctx context.Context) (err error) {
	if r.reaperTicker != nil {
		r.reaperTicker.Stop()
	}
	err = r.queueClient.UnSubscribe(
		ctx,
		r.antCfg.Common.GetRecentlyCompletedJobsTopic(),
		r.recentlyCompletedJobIDsSubscriberID)
	r.stopped = true
	return
}

func (r *ContainersReaper) canReap(container executor.Info) bool {
	if strings.Contains(container.GetName(), "frm-") {
		if container.ElapsedSecs() > r.antCfg.Common.MaxJobTimeout {
			return true
		}
		parts := strings.Split(container.GetName(), "-")
		if len(parts) > 3 {
			exists, ok := r.recentlyCompletedJobIDs.Get(parts[1])
			if ok {
				return exists == true
			}
		}
	}
	return false
}

// TODO fetch dead ids from /jobs/requests/dead_ids
func (r *ContainersReaper) reap(ctx context.Context) {
	containersByMethods := AllRunningContainers(ctx, r.antCfg)
	total := 0
	reaped := 0
	reapedFailed := 0
	for method, containers := range containersByMethods {
		for _, container := range containers {
			total++
			if r.canReap(container) {
				opts := types.NewExecutorOptions(container.GetName(), method)
				err := StopContainer(
					ctx,
					r.antCfg,
					r.httpClient,
					opts,
					container.GetName(),
				)
				if err != nil {
					log.WithFields(
						log.Fields{
							"Component":   "ContainersReaper",
							"Container":   container.GetName(),
							"Started":     container.GetStartedAt(),
							"ElapsedSecs": container.ElapsedSecs(),
							"Error":       err,
							"Memory":      cutils.MemUsageMiBString(),
						}).Error("failed to reap container")
					reapedFailed++
					r.metricsRegistry.Incr(
						"container_reaped_failed_total", nil)
				} else {
					_ = r.sendContainerEvent(
						ctx,
						method,
						container)
					log.WithFields(
						log.Fields{
							"Component":   "ContainersReaper",
							"Container":   container.GetName(),
							"Started":     container.GetStartedAt(),
							"ElapsedSecs": container.ElapsedSecs(),
							"Memory":      cutils.MemUsageMiBString(),
						}).Warn("reaped container")
					reaped++
					r.metricsRegistry.Incr(
						"container_reaped_total", nil)
				}
			}
		}
	}
	if total > 0 || log.IsLevelEnabled(log.DebugLevel) {
		log.WithFields(
			log.Fields{
				"Component":       "ContainersReaper",
				"TotalContainers": total,
				"Timeout":         r.antCfg.Common.MaxJobTimeout,
				"Reaped":          reaped,
				"ReapedFailed":    reapedFailed,
			}).Infof("checking stale container")
	}
}

func (r *ContainersReaper) sendContainerEvent(
	ctx context.Context,
	method types.TaskMethod,
	container executor.Info) (err error) {
	var b []byte
	userID := container.GetLabels()[types.UserID]
	if b, err = events.NewContainerLifecycleEvent(
		"ContainersReaper",
		userID,
		r.antCfg.Common.ID,
		method,
		container.GetName(),
		container.GetID(),
		types.CANCELLED,
		container.GetLabels(),
		container.GetStartedAt(),
		container.GetEndedAt()).Marshal(); err == nil {
		if _, err = r.queueClient.Publish(
			ctx,
			r.antCfg.Common.GetContainerLifecycleTopic(),
			b,
			queue.NewMessageHeaders(
				queue.DisableBatchingKey, "true",
				"ContainerID", container.GetID(),
				"UserID", userID,
			),
		); err != nil {
			log.WithFields(
				log.Fields{
					"Component": "ContainersReaper",
					"AntID":     r.antCfg.Common.ID,
					"Container": container,
					"Error":     err,
					"Memory":    cutils.MemUsageMiBString(),
				}).Warnf("failed to send lifecycle event container")
		}
	}
	return
}

func (r *ContainersReaper) subscribeToRecentlyCompletedJobIDs(
	ctx context.Context,
	containerTopic string) (string, error) {
	callback := func(ctx context.Context, event *queue.MessageEvent,
		ack queue.AckHandler, nack queue.AckHandler) error {
		defer ack()
		jobIDsEvent, err := events.UnmarshalRecentlyCompletedJobsEvent(event.Payload)
		if err != nil {
			log.WithFields(log.Fields{
				"Component": "ContainersReaper",
				"Payload":   string(event.Payload),
				"Error":     err}).Error("failed to unmarshal registration by recently completed job-ids")
			return err
		}
		for _, id := range jobIDsEvent.JobIDs {
			r.recentlyCompletedJobIDs.Add(id, true)
		}
		return nil
	}
	return r.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    containerTopic,
		Shared:   false,
		Callback: callback,
		Props:    make(map[string]string),
	})
}
