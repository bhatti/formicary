package resource

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// Start background routine to clean up any registration of ants that are no longer alive
// and reap any ant allocations that haven't deallocated after some timeout
func (rm *ManagerImpl) startReaperTicker(ctx context.Context) {
	rm.ticker = time.NewTicker(rm.serverCfg.Jobs.AntRegistrationAliveTimeout / 2)
	go func() {
		for {
			select {
			case <-ctx.Done():
				rm.ticker.Stop()
				return
			case <-rm.ticker.C:
				rm.reapStaleAnts(ctx)
				rm.reapStaleAllocations(ctx)
			}
		}
	}()
}

// The ants need to keep sending heart beat events to notify the server otherwise server
// treats them as dead ants
func (rm *ManagerImpl) reapStaleAnts(_ context.Context) int {
	now := time.Now()
	removeAntIDs := make([]string, 0)
	// ant-id => registration
	for _, registration := range rm.state.getRegistrations() {
		if time.Duration(now.Unix()-registration.ReceivedAt.Unix())*time.Second > rm.serverCfg.Jobs.AntRegistrationAliveTimeout {
			removeAntIDs = append(removeAntIDs, registration.AntID)
		}
	}

	for _, antID := range removeAntIDs {
		removedTags, removedMethods := rm.state.removeRegistration(antID)
		logrus.WithFields(logrus.Fields{
			"Component":      "ResourceManager",
			"RemovedTags":    removedTags,
			"RemovedMethods": removedMethods,
			"AntID":          antID,
		}).Warnf("removing stale registration of ant %s", antID)
	}

	return len(removeAntIDs)
}

// The tasks can only borrow ant resources for a limited amount of time otherwise these resources are
// automatically released.
// Note: Be careful with setting the config value otherwise it may deallocate resources for running jobs.
func (rm *ManagerImpl) reapStaleAllocations(_ context.Context) int {
	removeReservation := rm.state.reapStaleAllocations(rm.serverCfg.Jobs.AntReservationTimeout)
	// releasing ant
	for _, reservation := range removeReservation {
		if err := rm.Release(reservation); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "ResourceManager",
				"RequestID":   reservation.JobRequestID,
				"TaskType":    reservation.TaskType,
				"AntID":       reservation.AntID,
				"AllocatedAt": reservation.AllocatedAt,
				"Timeout":     rm.serverCfg.Jobs.AntReservationTimeout,
				"Error":       err,
			}).Warn("failed to deallocate resource")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component":   "ResourceManager",
				"RequestID":   reservation.JobRequestID,
				"TaskType":    reservation.TaskType,
				"AntID":       reservation.AntID,
				"AllocatedAt": reservation.AllocatedAt,
				"Timeout":     rm.serverCfg.Jobs.AntReservationTimeout,
			}).Info("forced deallocated resource after timeout")
		}
	}
	return len(removeReservation)
}
