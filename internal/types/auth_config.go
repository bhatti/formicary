package types

import (
	"github.com/twinj/uuid"
	"net/http"
	"time"
)

// AuthConfig -- Defines auth config
type AuthConfig struct {
	Enabled            bool          `yaml:"enabled" mapstructure:"enabled"`
	CookieName         string        `yaml:"cookie_name" mapstructure:"cookie_name" env:"COOKIE_NAME"`
	JWTSecret          string        `yaml:"jwt_secret" mapstructure:"jwt_secret" env:"JWT_SECRET"`
	MaxAge             time.Duration `yaml:"max_age" mapstructure:"max_age"`
	TokenMaxAge        time.Duration `yaml:"token_max_age" mapstructure:"token_max_age"`
	Secure             bool          `yaml:"secure" mapstructure:"secure"`
	GoogleClientID     string        `yaml:"google_client_id" mapstructure:"google_client_id" env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string        `yaml:"google_client_secret" mapstructure:"google_client_secret" env:"GOOGLE_CLIENT_SECRET"`
	GoogleCallbackHost string        `yaml:"google_callback_host" mapstructure:"google_callback_host" env:"GOOGLE_CALLBACK_HOST"`
	GithubClientID     string        `yaml:"github_client_id" mapstructure:"github_client_id" env:"GITHUB_CLIENT_ID"`
	GithubClientSecret string        `yaml:"github_client_secret" mapstructure:"github_client_secret" env:"GITHUB_CLIENT_SECRET"`
	GithubCallbackHost string        `yaml:"github_callback_host" mapstructure:"github_callback_host" env:"GITHUB_CALLBACK_HOST"`
}

// SessionCookie returns session cookie
func (c *AuthConfig) SessionCookie(token string, expiration time.Time) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = c.CookieName
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true
	return cookie
}

// HasGoogleOAuth if google oauth is initialized
func (c *AuthConfig) HasGoogleOAuth() bool {
	return c.GoogleCallbackHost != "" && c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

// HasGithubOAuth if github oauth is initialized
func (c *AuthConfig) HasGithubOAuth() bool {
	return c.GithubCallbackHost != "" && c.GithubClientID != "" && c.GithubClientSecret != ""
}

// ExpiredCookie returns expired cookie
func (c *AuthConfig) ExpiredCookie(name string) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = ""
	cookie.Expires = time.Unix(0, 0)
	cookie.Path = "/"
	cookie.HttpOnly = true
	return cookie
}

// LoginStateCookieName returns state cookie name
func (c *AuthConfig) LoginStateCookieName() string {
	return c.CookieName + "-state"
}

// LoginStateCookie returns state cookie
func (c *AuthConfig) LoginStateCookie() *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = c.LoginStateCookieName()
	cookie.Value = uuid.NewV4().String()
	cookie.Expires = time.Now().Add(15 * time.Minute)
	cookie.Path = "/"
	cookie.HttpOnly = true
	return cookie
}
