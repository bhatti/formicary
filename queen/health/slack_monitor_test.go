// SPDX-License-Identifier: AGPL-3.0-or-later

package health

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// stubSysConfigRepo is a no-op SystemConfigRepository for testing.
type stubSysConfigRepo struct{}

var _ repository.SystemConfigRepository = &stubSysConfigRepo{}

func (s *stubSysConfigRepo) Query(_ map[string]interface{}, _, _ int, _ []string) ([]*types.SystemConfig, int64, error) {
	return nil, 0, nil
}
func (s *stubSysConfigRepo) GetByKindName(_, _ string) (*types.SystemConfig, error) { return nil, nil }
func (s *stubSysConfigRepo) Count(_ map[string]interface{}) (int64, error)           { return 0, nil }
func (s *stubSysConfigRepo) Get(_ string) (*types.SystemConfig, error)               { return nil, nil }
func (s *stubSysConfigRepo) Delete(_ string) error                                   { return nil }
func (s *stubSysConfigRepo) Save(c *types.SystemConfig) (*types.SystemConfig, error) { return c, nil }

func newTestSlackCfg(appToken, botToken string) *config.ServerConfig {
	cfg := &config.ServerConfig{}
	cfg.Slack.AppToken = appToken
	cfg.Slack.BotToken = botToken
	return cfg
}

func TestSlackConfigMonitor_Name(t *testing.T) {
	monitor := NewSlackConfigMonitor(newTestSlackCfg("", ""), &stubSysConfigRepo{}, func() bool { return false })
	assert.Equal(t, "slack", monitor.Name())
}

func TestSlackConfigMonitor_DisabledWhenNoToken(t *testing.T) {
	monitor := NewSlackConfigMonitor(newTestSlackCfg("", ""), &stubSysConfigRepo{}, func() bool { return false })
	err := monitor.PerformHealthCheck(context.Background())
	assert.NoError(t, err, "No AppToken = Slack disabled = should be healthy (not an error)")
}

func TestSlackConfigMonitor_MissingBotToken(t *testing.T) {
	monitor := NewSlackConfigMonitor(newTestSlackCfg("xapp-test-token", ""), &stubSysConfigRepo{}, func() bool { return false })
	err := monitor.PerformHealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BotToken")
}

func TestSlackConfigMonitor_NotConnected(t *testing.T) {
	monitor := NewSlackConfigMonitor(newTestSlackCfg("xapp-test-token", "xoxb-test-token"), &stubSysConfigRepo{}, func() bool { return false })
	err := monitor.PerformHealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestSlackConfigMonitor_Connected(t *testing.T) {
	monitor := NewSlackConfigMonitor(newTestSlackCfg("xapp-test-token", "xoxb-test-token"), &stubSysConfigRepo{}, func() bool { return true })
	err := monitor.PerformHealthCheck(context.Background())
	assert.NoError(t, err, "Configured and connected should be healthy")
}
