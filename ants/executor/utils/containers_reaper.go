package utils

import (
	"context"
	"strconv"
	"strings"
	"time"

	"plexobject.com/formicary/ants/config"
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
	antCfg                              *config.AntConfig
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
	antCfg *config.AntConfig,
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
	r.reaperTicker = time.NewTicker(r.antCfg.ContainerReaperInterval)
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
	if r.recentlyCompletedJobIDsSubscriberID, err = r.subscribeToRecentlyCompletedJobIDs(ctx, r.antCfg.GetRecentlyCompletedJobsTopic()); err != nil {
		_ = r.Stop(ctx)
		return err
	}
	log.WithFields(
		log.Fields{
			"Component": "ContainersReaper",
			"Timeout":   r.antCfg.ContainerReaperInterval,
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
		r.antCfg.GetRecentlyCompletedJobsTopic(),
		r.recentlyCompletedJobIDsSubscriberID)
	r.stopped = true
	return
}

func (r *ContainersReaper) canReap(container executor.Info) bool {
	if strings.Contains(container.GetName(), "frm-") {
		if container.ElapsedSecs() > r.antCfg.MaxJobTimeout {
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
				"Timeout":         r.antCfg.MaxJobTimeout,
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
		r.antCfg.ID,
		method,
		container.GetName(),
		container.GetID(),
		types.CANCELLED,
		container.GetLabels(),
		container.GetStartedAt(),
		container.GetEndedAt()).Marshal(); err == nil {
		if _, err = r.queueClient.Publish(
			ctx,
			r.antCfg.GetContainerLifecycleTopic(),
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
					"AntID":     r.antCfg.ID,
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
	return r.queueClient.Subscribe(
		ctx,
		containerTopic,
		false, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			jobIDsEvent, err := events.UnmarshalRecentlyCompletedJobsEvent(event.Payload)
			if err != nil {
				log.WithFields(log.Fields{
					"Component": "ContainersReaper",
					"Payload":   string(event.Payload),
					"Error":     err}).Error("failed to unmarshal registration by recently completed job-ids")
				return err
			}
			for _, id := range jobIDsEvent.JobIDs {
				r.recentlyCompletedJobIDs.Add(strconv.FormatUint(id, 10), true)
			}
			return nil
		},
		nil,
		make(map[string]string),
	)
}
