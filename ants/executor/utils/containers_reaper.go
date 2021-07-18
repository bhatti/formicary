package utils

import (
	"context"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// ContainersReaper struct
type ContainersReaper struct {
	antCfg          *config.AntConfig
	queueClient     queue.Client
	httpClient      web.HTTPClient
	metricsRegistry *metrics.Registry
	ticker          *time.Ticker
}

// NewContainersReaper constructor
func NewContainersReaper(
	antCfg *config.AntConfig,
	queueClient queue.Client,
	httpClient web.HTTPClient,
	metricsRegistry *metrics.Registry,
) *ContainersReaper {
	return &ContainersReaper{
		antCfg:          antCfg,
		queueClient:     queueClient,
		httpClient:      httpClient,
		metricsRegistry: metricsRegistry,
	}
}

// Start ticker for reaping
func (r *ContainersReaper) Start(ctx context.Context) {
	r.ticker = time.NewTicker(r.antCfg.ContainerReaperInterval)
	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.reap(ctx)
			case <-ctx.Done():
				r.ticker.Stop()
				return
			}
		}
	}()
}

// Stop ticker for reaping
func (r *ContainersReaper) Stop() {
	if r.ticker != nil {
		r.ticker.Stop()
	}
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
			if strings.Contains(container.GetName(), "formicary-") &&
				container.ElapsedSecs() > r.antCfg.MaxJobTimeout {
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
						}).Warn("reaped container")
					reaped++
					r.metricsRegistry.Incr(
						"container_reaped_total", nil)
				}
			}
		}
		if total > 0 && log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(
				log.Fields{
					"Component":       "ContainersReaper",
					"TotalContainers": total,
					"Reaped":          reaped,
					"ReapedFailed":    reapedFailed,
				}).Debug("checking stale container")
		}
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
			make(map[string]string),
			b,
			false); err != nil {
			log.WithFields(
				log.Fields{
					"Component": "ContainersReaper",
					"AntID":     r.antCfg.ID,
					"Container": container,
					"Error":     err,
				}).Warnf("failed to send lifecycle event container")
		}
	}
	return
}
