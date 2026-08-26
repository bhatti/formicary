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
	appToken    string // resolved once at construction; tokens don't change at runtime
	botToken    string
	isConnected func() bool
}

// NewSlackConfigMonitor creates a new Slack health monitor.
// Tokens are resolved once from cfg then systemConfigRepo — no DB hit per health poll.
func NewSlackConfigMonitor(
	cfg *config.ServerConfig,
	systemConfigRepo repository.SystemConfigRepository,
	isConnected func() bool,
) *SlackConfigMonitor {
	appToken := cfg.Slack.AppToken
	if appToken == "" {
		if sc, err := systemConfigRepo.GetByKindName("SLACK", "AppToken"); err == nil && sc != nil {
			appToken = sc.Value
		}
	}
	botToken := cfg.Slack.BotToken
	if botToken == "" {
		if sc, err := systemConfigRepo.GetByKindName("SLACK", "BotToken"); err == nil && sc != nil {
			botToken = sc.Value
		}
	}
	return &SlackConfigMonitor{
		appToken:    appToken,
		botToken:    botToken,
		isConnected: isConnected,
	}
}

// Name implements Monitorable.
func (m *SlackConfigMonitor) Name() string { return "slack" }

// PerformHealthCheck returns nil when Slack is disabled (no token) or connected.
// Returns an error only when Slack is configured but not connected.
func (m *SlackConfigMonitor) PerformHealthCheck(_ context.Context) error {
	if m.appToken == "" {
		// Slack not configured — optional feature, not an error
		return nil
	}
	if m.botToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN missing — AppToken is set but BotToken is not; outbound Slack notifications will fail")
	}
	if !m.isConnected() {
		return fmt.Errorf("Slack Socket Mode not connected — verify AppToken validity and Slack App Event Subscriptions (app_mention)")
	}
	return nil
}
