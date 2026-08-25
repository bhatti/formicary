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
		for !rm.isStopped() {
			select {
			case <-ctx.Done():
				rm.ticker.Stop()
				return
			case <-rm.ticker.C:
				rm.reapStaleAnts(ctx)
				rm.reapStaleAllocations(ctx)
				rm.reapStaleContainers()
			}
		}
	}()
}

// The ants need to keep sending heart beat events to notify the server otherwise server
// treats them as dead ants
func (rm *ManagerImpl) reapStaleAnts(ctx context.Context) int {
	now := time.Now()
	removeAntIDs := make([]string, 0)
	// ant-id => registration
	for _, registration := range rm.state.getRegistrations() {
		if time.Duration(now.Unix()-registration.ReceivedAt.Unix())*time.Second > rm.serverCfg.Jobs.AntRegistrationAliveTimeout {
			removeAntIDs = append(removeAntIDs, registration.AntID)
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component":    "ResourceManager",
					"Registration": registration,
					"Received":     registration.ReceivedAt,
				}).Debugf("adding stale registration of ant %s", registration.AntID)
			}
		}
		if registration.ValidRegistration != nil {
			if err := registration.ValidRegistration(ctx); err != nil {
				removeAntIDs = append(removeAntIDs, registration.AntID)
				logrus.WithFields(logrus.Fields{
					"Component":    "ResourceManager",
					"Registration": registration,
					"Received":     registration.ReceivedAt,
					"Error":        err,
				}).Warnf("adding invalid registration of ant %s", registration.AntID)
			}
		}
	}

	// Capture orgIDs before removal so we can update "no-ant" banners after.
	orgIDsAffected := make(map[string]struct{})
	for _, antID := range removeAntIDs {
		if rm.bannerBridge != nil {
			if reg := rm.state.getRegistrationByAnt(antID); reg != nil && reg.OrgID != "" {
				orgIDsAffected[reg.OrgID] = struct{}{}
			}
		}
		removedTags, removedMethods, unregistered := rm.state.removeRegistration(antID)
		logrus.WithFields(logrus.Fields{
			"Component":      "ResourceManager",
			"RemovedTags":    removedTags,
			"RemovedMethods": removedMethods,
			"Unregistered":   unregistered,
			"AntID":          antID,
		}).Warnf("removing stale registration of ant %s", antID)
	}

	// Update "no ant registered" banners for each affected org.
	if rm.bannerBridge != nil {
		for orgID := range orgIDsAffected {
			remaining := rm.RegistrationsByOrg(orgID)
			hasExternal := false
			for _, r := range remaining {
				if r.HasExternalMethods() {
					hasExternal = true
					break
				}
			}
			rm.bannerBridge.SyncNoAntForOrg(orgID, hasExternal)
		}
	}

	return len(removeAntIDs)
}

// reapStaleContainers evicts container records that have been running longer than
// the configured reservation timeout. This prevents stale container events (e.g. from
// replayed queue messages or unreported OOMKills) from accumulating on the executors page.
func (rm *ManagerImpl) reapStaleContainers() int {
	maxAge := rm.serverCfg.Jobs.AntReservationTimeout
	evicted := rm.state.evictStaleContainers(maxAge)
	if evicted > 0 {
		logrus.WithFields(logrus.Fields{
			"Component": "ResourceManager",
			"Evicted":   evicted,
			"MaxAge":    maxAge,
		}).Warn("evicted stale container records")
	}
	return evicted
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
