// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// stopFn is a function that stops a running trigger subscription or poller.
type stopFn func(ctx context.Context)

// triggerKey uniquely identifies a trigger within the system.
type triggerKey struct {
	jobDefID    string
	triggerName string
}

// Manager orchestrates all event-driven triggers. It is leader-aware:
// S3 pollers and queue subscribers are only active on the scheduler leader.
// Webhook routes are registered on all instances.
type Manager struct {
	serverCfg        *config.ServerConfig
	queueClient      queue.Client
	jobManager       *manager.JobManager
	triggerStateRepo repository.TriggerStateRepository
	evaluator        *Evaluator
	submitter        *Submitter
	webhookHandler   *WebhookHandler

	// active holds stop functions for running pollers/subscribers.
	active map[triggerKey]stopFn
	mu     sync.Mutex

	// leader state: updated by JobSchedulerLeader subscription.
	isLeader                     bool
	lastLeaderEventAt            time.Time
	leaderSubscriptionID         string
	defLifecycleSubscriptionID   string

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a TriggerManager.
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	jobManager *manager.JobManager,
	triggerStateRepo repository.TriggerStateRepository,
	evaluator *Evaluator,
	submitter *Submitter,
	webhookHandler *WebhookHandler,
) *Manager {
	return &Manager{
		serverCfg:        serverCfg,
		queueClient:      queueClient,
		jobManager:       jobManager,
		triggerStateRepo: triggerStateRepo,
		evaluator:        evaluator,
		submitter:        submitter,
		webhookHandler:   webhookHandler,
		active:           make(map[triggerKey]stopFn),
		stopCh:           make(chan struct{}),
	}
}

// Start subscribes to leader events and job definition lifecycle events,
// then loads all existing job definitions with triggers.
func (m *Manager) Start(ctx context.Context) error {
	if m.serverCfg.Jobs.DisableTriggers {
		logrus.WithField("Component", "TriggerManager").Info("triggers disabled by config")
		return nil
	}

	var err error
	// Subscribe to scheduler leader heartbeats.
	m.leaderSubscriptionID, err = m.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    m.serverCfg.Common.GetJobSchedulerLeaderTopic(),
		Shared:   false,
		Callback: m.handleLeaderEvent,
	})
	if err != nil {
		return err
	}

	// Subscribe to job definition lifecycle events (create/update/delete).
	m.defLifecycleSubscriptionID, err = m.queueClient.Subscribe(ctx, queue.SubscribeOptions{
		Topic:    m.serverCfg.Common.GetJobDefinitionLifecycleTopic(),
		Shared:   false,
		Callback: m.handleJobDefLifecycle,
	})
	if err != nil {
		return err
	}

	// Load all job definitions that have triggers and start them if we become leader.
	go m.watchLeaderAndLoad(ctx)

	logrus.WithField("Component", "TriggerManager").Info("TriggerManager started")
	return nil
}

// Stop halts all active triggers and unsubscribes.
func (m *Manager) Stop(ctx context.Context) {
	close(m.stopCh)
	m.stopAllTriggers(ctx)
	if m.leaderSubscriptionID != "" {
		_ = m.queueClient.UnSubscribe(ctx, m.serverCfg.Common.GetJobSchedulerLeaderTopic(), m.leaderSubscriptionID)
	}
	if m.defLifecycleSubscriptionID != "" {
		_ = m.queueClient.UnSubscribe(ctx, m.serverCfg.Common.GetJobDefinitionLifecycleTopic(), m.defLifecycleSubscriptionID)
	}
	m.wg.Wait()
}

// watchLeaderAndLoad waits until this instance becomes leader then loads triggers.
func (m *Manager) watchLeaderAndLoad(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	wasLeader := false
	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			nowLeader := m.isLeader && time.Since(m.lastLeaderEventAt) < m.serverCfg.Jobs.JobSchedulerLeaderInterval*3
			m.mu.Unlock()

			if nowLeader && !wasLeader {
				wasLeader = true
				m.loadAllTriggers(ctx)
			} else if !nowLeader && wasLeader {
				wasLeader = false
				m.stopAllTriggers(ctx)
			}
		}
	}
}

func (m *Manager) handleLeaderEvent(_ context.Context, event *queue.MessageEvent, ack queue.AckHandler, _ queue.AckHandler) error {
	var leaderEvent events.JobSchedulerLeaderEvent
	if err := json.Unmarshal(event.Payload, &leaderEvent); err != nil {
		return err
	}
	m.mu.Lock()
	m.isLeader = true
	m.lastLeaderEventAt = time.Now()
	m.mu.Unlock()
	if ack != nil {
		ack()
	}
	return nil
}

func (m *Manager) handleJobDefLifecycle(_ context.Context, event *queue.MessageEvent, ack queue.AckHandler, _ queue.AckHandler) error {
	var defEvent events.JobDefinitionLifecycleEvent
	if err := json.Unmarshal(event.Payload, &defEvent); err != nil {
		if ack != nil {
			ack()
		}
		return nil
	}

	ctx := context.Background()
	m.mu.Lock()
	isLeader := m.isLeader
	m.mu.Unlock()

	if defEvent.StateChange == events.DELETED || defEvent.StateChange == events.DISABLED {
		m.stopTriggersForDef(ctx, defEvent.JobDefinitionID)
	} else if defEvent.StateChange == events.UPDATED || defEvent.StateChange == events.ENABLED {
		if isLeader {
			m.stopTriggersForDef(ctx, defEvent.JobDefinitionID)
			m.startTriggersForDef(ctx, defEvent.JobDefinitionID)
		}
	}

	if ack != nil {
		ack()
	}
	return nil
}

func (m *Manager) loadAllTriggers(ctx context.Context) {
	qc := common.NewQueryContextFromIDs("", "").WithAdmin()
	page := 0
	pageSize := 100
	for {
		defs, _, err := m.jobManager.QueryJobDefinitions(qc, map[string]interface{}{}, page, pageSize, []string{"job_type"})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "TriggerManager",
				"Error":     err,
			}).Errorf("failed to load job definitions for triggers")
			return
		}
		for _, def := range defs {
			if len(def.Triggers) > 0 {
				m.startTriggersForJobDef(ctx, def)
			}
		}
		if len(defs) < pageSize {
			break
		}
		page++
	}
}

func (m *Manager) stopAllTriggers(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, stop := range m.active {
		stop(ctx)
		delete(m.active, k)
	}
}

func (m *Manager) stopTriggersForDef(ctx context.Context, jobDefID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, stop := range m.active {
		if k.jobDefID == jobDefID {
			stop(ctx)
			delete(m.active, k)
		}
	}
}

func (m *Manager) startTriggersForDef(ctx context.Context, jobDefID string) {
	qc := common.NewQueryContextFromIDs("", "").WithAdmin()
	def, err := m.jobManager.GetJobDefinition(qc, jobDefID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":       "TriggerManager",
			"JobDefinitionID": jobDefID,
			"Error":           err,
		}).Errorf("failed to load job definition for trigger start")
		return
	}
	if def == nil || def.Disabled || len(def.Triggers) == 0 {
		return
	}
	m.startTriggersForJobDef(ctx, def)
}

func (m *Manager) startTriggersForJobDef(ctx context.Context, def *types.JobDefinition) {
	for _, t := range def.Triggers {
		key := triggerKey{jobDefID: def.ID, triggerName: t.Name}
		switch t.Type {
		case "queue":
			qs, err := NewQueueSubscriber(ctx, m.queueClient, m.evaluator, m.submitter, def, t)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":   "TriggerManager",
					"JobType":     def.JobType,
					"TriggerName": t.Name,
					"Error":       err,
				}).Errorf("failed to start queue trigger")
				continue
			}
			m.mu.Lock()
			m.active[key] = func(c context.Context) { qs.Stop(c) }
			m.mu.Unlock()
		case "s3":
			if t.Mode == "notification" {
				sns, err := NewS3NotificationSubscriber(ctx, m.queueClient, m.evaluator, m.submitter, def, t)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":   "TriggerManager",
						"JobType":     def.JobType,
						"TriggerName": t.Name,
						"Error":       err,
					}).Errorf("failed to start S3 notification trigger")
					continue
				}
				m.mu.Lock()
				m.active[key] = func(c context.Context) { sns.Stop(c) }
				m.mu.Unlock()
			} else {
				poller, err := NewS3Poller(
					ctx,
					m.serverCfg.Common.S3,
					m.evaluator,
					m.submitter,
					m.triggerStateRepo,
					def,
					t,
					m.serverCfg.Jobs.TriggerPollDefaultInterval,
				)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":   "TriggerManager",
						"JobType":     def.JobType,
						"TriggerName": t.Name,
						"Error":       err,
					}).Errorf("failed to start S3 poll trigger")
					continue
				}
				m.mu.Lock()
				m.active[key] = func(_ context.Context) { poller.Stop() }
				m.mu.Unlock()
			}
		case "webhook":
			// Webhook routes are registered at startup via WebhookHandler; nothing to do here.
			continue
		}
		logrus.WithFields(logrus.Fields{
			"Component":   "TriggerManager",
			"JobType":     def.JobType,
			"TriggerName": t.Name,
			"TriggerType": t.Type,
		}).Infof("trigger started")
	}
}
