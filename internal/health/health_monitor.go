package health

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

// Monitorable defines interface for services that can be monitored for health-check.
type Monitorable interface {
	Name() string
	PerformHealthCheck(context.Context) error
}

// StatusCallback is called after each health-check cycle with the current service statuses.
type StatusCallback func(statuses []*ServiceStatus)

// Monitor structure for state of services
type Monitor struct {
	conf            *types.CommonConfig
	queueClient     queue.Client
	started         time.Time
	name            string
	overallStatus   *ServiceStatus
	registered      map[string]*ServiceStatus
	ticker          *time.Ticker
	statusCallbacks []StatusCallback
	lock            sync.RWMutex
}

// OnHealthCheck registers a callback invoked after each health-check cycle.
// Callbacks receive a snapshot of all service statuses and run synchronously
// inside the ticker goroutine — keep them fast (fire-and-forget DB writes are fine).
func (m *Monitor) OnHealthCheck(cb StatusCallback) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.statusCallbacks = append(m.statusCallbacks, cb)
}

// New instantiates monitor
func New(
	conf *types.CommonConfig,
	queueClient queue.Client) (*Monitor, error) {
	monitor := &Monitor{
		conf:        conf,
		queueClient: queueClient,
		started:     time.Now(),
		name:        "Health-Monitor",
		registered:  make(map[string]*ServiceStatus, 0),
	}
	monitor.overallStatus = NewServiceStatus(monitor)
	for name, url := range conf.MonitoringURLs {
		hpm, err := NewHostPortMonitor(name, url)
		if err != nil {
			return nil, err
		}
		monitor.Register(context.Background(), hpm)
	}
	return monitor, nil
}

// Register adds a critical service to monitor. Failures block scheduling.
func (m *Monitor) Register(_ context.Context, monitored Monitorable) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.registered[monitored.Name()] = NewServiceStatus(monitored)
}

// RegisterNonCritical adds an optional service to monitor.
// Its failures are logged and shown in health UI but do NOT affect overall HealthStatus.
func (m *Monitor) RegisterNonCritical(_ context.Context, monitored Monitorable) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.registered[monitored.Name()] = NewNonCriticalServiceStatus(monitored)
}

// GetAllStatuses returns all status
func (m *Monitor) GetAllStatuses() (overall *ServiceStatus, all []*ServiceStatus) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	overall = m.overallStatus
	all = make([]*ServiceStatus, len(m.registered))
	i := 0
	for _, e := range m.registered {
		all[i] = e
		i++
	}
	return
}

// Unregister removes service for monitoring
func (m *Monitor) Unregister(_ context.Context, monitored Monitorable) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.registered, monitored.Name())
}

// Name - name of overall monitor
func (m *Monitor) Name() string {
	return m.name
}

// HealthStatus Cached service status based on consecutive failures to avoid false positives
func (m *Monitor) HealthStatus(_ context.Context) error {
	if m.overallStatus.ConsecutiveFailures >= 3 {
		return m.overallStatus.HealthError
	}
	return nil
}

// PerformHealthCheck goes through all registered services and check their health status
func (m *Monitor) PerformHealthCheck(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	var overallError error
	for _, next := range m.registered {
		if err := next.performHealthCheck(ctx); err != nil {
			if next.Critical {
				overallError = err
			}
			logrus.WithFields(logrus.Fields{
				"Component": "HealthMonitor",
				"Status":    next,
			}).Error("failed to check health status")
		}
	}
	return overallError
}

// Start - starts background ticker for periodic health check
func (m *Monitor) Start(ctx context.Context) error {
	return m.startTicker(ctx)
}

// Stop - stops background ticker for periodic health check
func (m *Monitor) Stop(context.Context) {
	if m.ticker != nil {
		m.ticker.Stop()
	}
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (m *Monitor) startTicker(ctx context.Context) error {
	// use registration as a form of heart-beat along with current load so that server can load balance
	m.ticker = time.NewTicker(m.conf.MonitorInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				m.ticker.Stop()
				return
			case <-m.ticker.C:
				if err := m.overallStatus.performHealthCheck(ctx); err != nil {
					_ = m.fireHealthError(err)
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(logrus.Fields{
							"Component": "HealthMonitor",
							"Error":     err,
							"Status":    m.overallStatus,
						}).Debug("failed to monitor health check")
					}
				}
				// Notify all registered callbacks with the current service statuses.
				_, statuses := m.GetAllStatuses()
				m.lock.RLock()
				cbs := make([]StatusCallback, len(m.statusCallbacks))
				copy(cbs, m.statusCallbacks)
				m.lock.RUnlock()
				for _, cb := range cbs {
					cb(statuses)
				}
			}
		}
	}()
	return nil
}

// Fire event to notify health error
func (m *Monitor) fireHealthError(err error) error {
	event := events.NewHealthErrorEvent(
		m.conf.ID,
		err.Error())
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal health error event due to %w", err)
	}
	if _, err = m.queueClient.Publish(
		context.Background(),
		m.conf.GetHealthErrorTopic(),
		payload,
		queue.NewMessageHeaders(),
	); err != nil {
		return fmt.Errorf("failed to send health error event due to %w", err)
	}
	return nil
}
