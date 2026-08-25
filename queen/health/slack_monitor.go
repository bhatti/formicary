// SPDX-License-Identifier: AGPL-3.0-or-later

package health

import (
	"context"
	"fmt"

	ihealth "plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
)

var _ ihealth.Monitorable = &SlackConfigMonitor{}

// SlackConfigMonitor checks Slack connectivity.
// Returns healthy when Slack is disabled (no AppToken configured).
// Returns an error only when Slack is configured but not connected.
type SlackConfigMonitor struct {
	cfg              *config.ServerConfig
	systemConfigRepo repository.SystemConfigRepository
	isConnected      func() bool
}

// NewSlackConfigMonitor creates a new Slack health monitor.
func NewSlackConfigMonitor(
	cfg *config.ServerConfig,
	systemConfigRepo repository.SystemConfigRepository,
	isConnected func() bool,
) *SlackConfigMonitor {
	return &SlackConfigMonitor{
		cfg:              cfg,
		systemConfigRepo: systemConfigRepo,
		isConnected:      isConnected,
	}
}

// Name implements Monitorable.
func (m *SlackConfigMonitor) Name() string { return "slack" }

// PerformHealthCheck returns nil when Slack is disabled (no token) or connected.
// Returns an error only when Slack is configured but not connected.
func (m *SlackConfigMonitor) PerformHealthCheck(_ context.Context) error {
	appToken := m.resolveAppToken()
	if appToken == "" {
		// Slack not configured — optional feature, not an error
		return nil
	}
	botToken := m.resolveBotToken()
	if botToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN missing — AppToken is set but BotToken is not; outbound Slack notifications will fail")
	}
	if !m.isConnected() {
		return fmt.Errorf("Slack Socket Mode not connected — verify AppToken validity and Slack App Event Subscriptions (app_mention)")
	}
	return nil
}

func (m *SlackConfigMonitor) resolveAppToken() string {
	if m.cfg.Slack.AppToken != "" {
		return m.cfg.Slack.AppToken
	}
	if cfg, err := m.systemConfigRepo.GetByKindName("SLACK", "AppToken"); err == nil && cfg != nil {
		return cfg.Value
	}
	return ""
}

func (m *SlackConfigMonitor) resolveBotToken() string {
	if m.cfg.Slack.BotToken != "" {
		return m.cfg.Slack.BotToken
	}
	if cfg, err := m.systemConfigRepo.GetByKindName("SLACK", "BotToken"); err == nil && cfg != nil {
		return cfg.Value
	}
	return ""
}
