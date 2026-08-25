// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

func Test_Should_Return_Nil_When_User_Not_Registered(t *testing.T) {
	// GIVEN a UserRegistry backed by real test repositories
	serverCfg := config.TestServerConfig()
	userManager, err := manager.TestUserManager(serverCfg)
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	registry := NewUserRegistry(serverCfg, userManager, configRepo)

	// WHEN looking up an unknown Slack user ID
	user, token, err := registry.LookupBySlackID(context.Background(), "U_UNKNOWN_12345")

	// THEN returns nil without error
	require.NoError(t, err)
	require.Nil(t, user)
	require.Equal(t, "", token)
}

func Test_Should_Reject_Invalid_Token(t *testing.T) {
	// GIVEN a UserRegistry
	serverCfg := config.TestServerConfig()
	serverCfg.Common.Auth.JWTSecret = "test-secret"
	userManager, err := manager.TestUserManager(serverCfg)
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	registry := NewUserRegistry(serverCfg, userManager, configRepo)

	// WHEN registering with a garbage token
	_, err = registry.Register(context.Background(), "U_SLACK_123", "not-a-jwt-token", nil, "", "")

	// THEN an error is returned
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")
}
