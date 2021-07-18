package auth

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// Provider defines interface for authentication
type Provider interface {
	AuthHandler(state string) string
	AuthUser(expectedState string, c web.WebContext) (*common.User, error)
	AuthLoginURL() string
	AuthLoginCallbackURL() string
	AuthWebhookCallbackURL() string
	AuthWebhookCallbackHandle(c web.WebContext) error
}
