// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	qtypes "plexobject.com/formicary/queen/types"
)

// stubSlackRegCodeRepo is a no-op SlackRegCodeRepository for tests.
type stubSlackRegCodeRepo struct{}

func (s *stubSlackRegCodeRepo) Create(_ *common.QueryContext, _ *common.SlackRegCode) error {
	return nil
}
func (s *stubSlackRegCodeRepo) Consume(_ string) (*common.SlackRegCode, error) { return nil, nil }
func (s *stubSlackRegCodeRepo) PurgeExpired() error                            { return nil }

// stubSystemConfigRepo is an in-memory SystemConfigRepository for tests.
type stubSystemConfigRepo struct {
	byKindName map[string]*qtypes.SystemConfig
}

func newStubSystemConfigRepo() *stubSystemConfigRepo {
	return &stubSystemConfigRepo{byKindName: make(map[string]*qtypes.SystemConfig)}
}

func (s *stubSystemConfigRepo) set(kind, name, value string) {
	s.byKindName[kind+":"+name] = &qtypes.SystemConfig{Name: name, Kind: kind, Value: value}
}

func (s *stubSystemConfigRepo) GetByKindName(kind, name string) (*qtypes.SystemConfig, error) {
	v := s.byKindName[kind+":"+name]
	return v, nil
}
func (s *stubSystemConfigRepo) Query(params map[string]interface{}, page, pageSize int, order []string) ([]*qtypes.SystemConfig, int64, error) {
	return nil, 0, nil
}
func (s *stubSystemConfigRepo) Count(params map[string]interface{}) (int64, error) { return 0, nil }
func (s *stubSystemConfigRepo) Get(id string) (*qtypes.SystemConfig, error)        { return nil, nil }
func (s *stubSystemConfigRepo) Delete(id string) error                              { return nil }
func (s *stubSystemConfigRepo) Save(ec *qtypes.SystemConfig) (*qtypes.SystemConfig, error) {
	return ec, nil
}

func Test_Should_Return_Nil_When_App_Token_Empty(t *testing.T) {
	// GIVEN a server config with no Slack AppToken
	serverCfg := config.TestServerConfig()
	serverCfg.Slack.AppToken = ""
	serverCfg.Slack.BotToken = ""

	var jobManager *manager.JobManager
	var userManager *manager.UserManager
	var configRepo repository.ConfigRepository

	// WHEN constructing SlackService
	svc, err := NewSlackService(serverCfg, jobManager, userManager, configRepo, newStubSystemConfigRepo(), &stubSlackRegCodeRepo{})

	// THEN service is nil (Slack disabled) and no error
	require.NoError(t, err)
	require.Nil(t, svc)
}

func Test_Should_Start_And_Stop_When_AppToken_Empty(t *testing.T) {
	// GIVEN a server config with no Slack AppToken — service should be nil (disabled)
	serverCfg := config.TestServerConfig()
	serverCfg.Slack.AppToken = ""
	serverCfg.Slack.BotToken = ""

	var jobManager *manager.JobManager
	var userManager *manager.UserManager
	var configRepo repository.ConfigRepository

	svc, err := NewSlackService(serverCfg, jobManager, userManager, configRepo, newStubSystemConfigRepo(), &stubSlackRegCodeRepo{})
	require.NoError(t, err)
	require.Nil(t, svc, "service must be nil when AppToken is empty")

	if svc != nil {
		t.Fatal("expected nil service but got non-nil")
	}
}

func Test_Should_Load_Admin_Routes_From_SystemConfig(t *testing.T) {
	// GIVEN a SystemConfig with a valid SlackRoutes JSON array
	adminRoutes := []config.SlackRouteConfig{
		{
			Triggers:    []string{"deploy"},
			JobType:     "my-deploy-job",
			IdVar:       "Environment",
			Description: "Deploy to an environment",
		},
	}
	b, err := json.Marshal(adminRoutes)
	require.NoError(t, err)

	sysRepo := newStubSystemConfigRepo()
	sysRepo.set("JSON", slackRoutesConfigName, string(b))

	// AND a static base route in server config
	serverCfg := config.TestServerConfig()
	serverCfg.Slack.AppToken = ""
	serverCfg.Slack.Routes = []config.SlackRouteConfig{
		{Triggers: []string{"standup"}, JobType: "ai-standup-jira"},
	}

	// Build the router directly (service constructor exits early when AppToken is empty)
	router := NewCommandRouter(serverCfg.Slack.Routes)
	cfg, _ := sysRepo.GetByKindName("JSON", slackRoutesConfigName)
	require.NotNil(t, cfg)

	var loaded []config.SlackRouteConfig
	require.NoError(t, json.Unmarshal([]byte(cfg.Value), &loaded))
	router = router.WithOrgRoutes(loaded)

	// WHEN routing the admin-added command
	result, isBuiltin, routeErr := router.Route("deploy staging")

	// THEN admin route is active and takes precedence
	require.NoError(t, routeErr)
	require.False(t, isBuiltin)
	require.Equal(t, "my-deploy-job", result.JobType)
	require.Equal(t, "staging", result.Trailing)
	require.Equal(t, "Environment", result.IdVar)

	// AND the static base route is still reachable
	result2, _, err2 := router.Route("standup")
	require.NoError(t, err2)
	require.Equal(t, "ai-standup-jira", result2.JobType)
}

func Test_Should_Ignore_Invalid_Admin_Routes_JSON(t *testing.T) {
	// GIVEN a SystemConfig with invalid JSON
	sysRepo := newStubSystemConfigRepo()
	sysRepo.set("JSON", slackRoutesConfigName, "not-valid-json")

	serverCfg := config.TestServerConfig()
	serverCfg.Slack.AppToken = ""
	serverCfg.Slack.Routes = []config.SlackRouteConfig{
		{Triggers: []string{"standup"}, JobType: "ai-standup-jira"},
	}

	// Build the router and attempt to load bad admin config
	router := NewCommandRouter(serverCfg.Slack.Routes)
	cfg, _ := sysRepo.GetByKindName("JSON", slackRoutesConfigName)
	require.NotNil(t, cfg)

	var loaded []config.SlackRouteConfig
	err := json.Unmarshal([]byte(cfg.Value), &loaded)

	// THEN unmarshal fails and static routes remain unchanged
	require.Error(t, err)
	result, _, routeErr := router.Route("standup")
	require.NoError(t, routeErr)
	require.Equal(t, "ai-standup-jira", result.JobType)
}
