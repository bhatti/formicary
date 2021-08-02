package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldCreateAuthConfig(t *testing.T) {
	// Given auth config
	c := AuthConfig {
		CookieName: "cookie",
		JWTSecret: "secret",
		MaxAge:  time.Second,
		TokenMaxAge: time.Second,
		Secure: true,
		GoogleClientID: "id",
		GoogleClientSecret: "secret",
		GoogleCallbackHost: "host",
		GithubClientID: "id",
		GithubClientSecret: "secret",
		GithubCallbackHost: "host",
	}
	require.NotNil(t, c.SessionCookie("token", time.Now()))
	require.NotNil(t, c.ExpiredCookie("token"))
	require.NotNil(t, c.LoginStateCookie())
	require.True(t, c.HasGoogleOAuth())
	require.True(t, c.HasGithubOAuth())
	require.Equal(t, "cookie-state", c.LoginStateCookieName())
	require.NoError(t, c.Validate())
	c.Enabled = true
	require.NoError(t, c.Validate())
}

