package health

import (
	"context"
	"fmt"
	"time"
)

// ServiceStatus structure for health metrics.
type ServiceStatus struct {
	Monitored           Monitorable
	ConsecutiveFailures uint64
	ConsecutiveSuccess  uint64
	TotalFailures       uint64
	TotalSuccess        uint64
	HealthError         error
	LastCheck           time.Time
}

// NewServiceStatus constructor
func NewServiceStatus(monitored Monitorable) *ServiceStatus {
	return &ServiceStatus{
		Monitored: monitored,
	}
}

// Healthy flag
func (ss *ServiceStatus) Healthy() bool {
	return ss.HealthError == nil
}

// LastCheckString formatted date
func (ss *ServiceStatus) LastCheckString() string {
	return ss.LastCheck.Format("Jan _2, 15:04:05 MST")
}

func (ss *ServiceStatus) String() string {
	return fmt.Sprintf("Name=%s Failures=%d/%d Successes=%d/%d Error=%v",
		ss.Monitored.Name(), ss.ConsecutiveFailures, ss.ConsecutiveFailures,
		ss.ConsecutiveSuccess, ss.TotalSuccess, ss.HealthError)
}

func (ss *ServiceStatus) performHealthCheck(ctx context.Context) error {
	ss.LastCheck = time.Now()
	ss.HealthError = ss.Monitored.PerformHealthCheck(ctx)
	if ss.HealthError != nil {
		ss.ConsecutiveFailures++
		ss.ConsecutiveSuccess = 0
		ss.TotalFailures++
		return ss.HealthError
	}
	ss.ConsecutiveSuccess++
	ss.ConsecutiveFailures = 0
	ss.TotalSuccess++
	return nil
}
