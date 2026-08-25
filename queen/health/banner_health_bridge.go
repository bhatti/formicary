// SPDX-License-Identifier: AGPL-3.0-or-later

package health

import (
	"fmt"

	"github.com/sirupsen/logrus"
	internalhealth "plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
)

// BannerHealthBridge writes and clears system banners in response to health monitor
// status changes and ant registration health updates. It persists banners to the DB
// so they survive queen restarts and are visible across all dashboard sessions.
type BannerHealthBridge struct {
	bannerRepo repository.BannerRepository
}

// NewBannerHealthBridge creates a bridge that writes system banners to bannerRepo.
func NewBannerHealthBridge(bannerRepo repository.BannerRepository) *BannerHealthBridge {
	return &BannerHealthBridge{bannerRepo: bannerRepo}
}

// SyncMonitorStatuses upserts or clears global system banners from queen-level health monitors.
// Register this as a health.Monitor callback via healthMonitor.OnHealthCheck(...).
func (b *BannerHealthBridge) SyncMonitorStatuses(statuses []*internalhealth.ServiceStatus) {
	for _, s := range statuses {
		key := "system:global:" + s.Monitored.Name()
		if !s.Healthy() && s.HealthError != nil {
			if err := b.bannerRepo.UpsertByKey(
				key,
				types.BannerLevelWarning,
				types.BannerScopeGlobal,
				"",
				s.Monitored.Name()+": "+s.HealthError.Error(),
			); err != nil {
				logrus.WithError(err).Warn("BannerHealthBridge: failed to upsert global banner")
			}
		} else {
			if err := b.bannerRepo.ClearByKey(key); err != nil {
				logrus.WithError(err).Warn("BannerHealthBridge: failed to clear global banner")
			}
		}
	}
}

// SyncAntHealth upserts or clears org-scoped banners based on the ant's method health.
// Call this whenever an AntRegistration heartbeat is received.
func (b *BannerHealthBridge) SyncAntHealth(ant *types.AntRegistration) {
	if !ant.HasExternalMethods() {
		return
	}
	orgID := ant.OrgID
	if orgID == "" {
		return // unscoped ants (auth disabled) — no org to target
	}
	for method, entry := range ant.MethodHealthSnapshot() {
		key := fmt.Sprintf("system:org:%s:ant:%s:%s", orgID, ant.AntID, method)
		if entry != nil && !entry.Healthy && entry.Error != "" {
			msg := fmt.Sprintf(
				"Ant worker %s: %s backend unreachable (%s). Jobs requiring this method will stay WAITING.",
				ant.AntID, method, entry.Error,
			)
			if err := b.bannerRepo.UpsertByKey(key, types.BannerLevelDanger, types.BannerScopeOrg, orgID, msg); err != nil {
				logrus.WithError(err).Warn("BannerHealthBridge: failed to upsert ant method banner")
			}
		} else {
			if err := b.bannerRepo.ClearByKey(key); err != nil {
				logrus.WithError(err).Warn("BannerHealthBridge: failed to clear ant method banner")
			}
		}
	}
}

// SyncNoAntForOrg upserts or clears the "no ant registered" banner for the given org.
func (b *BannerHealthBridge) SyncNoAntForOrg(orgID string, hasAnt bool) {
	if orgID == "" {
		return
	}
	key := "system:org:" + orgID + ":no-ant"
	if !hasAnt {
		msg := "No ant worker registered for your organization. Jobs will remain WAITING. " +
			"Run ./scripts/worker-install.sh or see the Ant Worker Setup guide."
		if err := b.bannerRepo.UpsertByKey(key, types.BannerLevelDanger, types.BannerScopeOrg, orgID, msg); err != nil {
			logrus.WithError(err).Warn("BannerHealthBridge: failed to upsert no-ant banner")
		}
	} else {
		if err := b.bannerRepo.ClearByKey(key); err != nil {
			logrus.WithError(err).Warn("BannerHealthBridge: failed to clear no-ant banner")
		}
	}
}
