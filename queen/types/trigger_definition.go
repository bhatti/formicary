// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"fmt"
	"time"
)

// TriggerAuth defines authentication for inbound webhook triggers.
type TriggerAuth struct {
	// Method is one of: hmac_sha256, bearer_token, api_key_header
	Method string `yaml:"method" json:"method"`
	// SecretConfig references a JobDefinitionConfig name holding the secret value.
	SecretConfig string `yaml:"secret_config" json:"secret_config"`
	// Header is the HTTP header carrying the signature or token.
	Header string `yaml:"header" json:"header"`
}

// TriggerRateLimit caps how many job requests a trigger can create per window.
type TriggerRateLimit struct {
	Max    int           `yaml:"max" json:"max"`
	Window time.Duration `yaml:"window" json:"window"`
}

// TriggerDefinition describes a single event trigger on a job definition.
// Triggers are transient — parsed from raw_yaml, never persisted as a separate table row.
type TriggerDefinition struct {
	// Type is required: "webhook", "s3", or "queue".
	Type string `yaml:"type" json:"type"`
	// Name is a unique identifier within the job definition.
	Name string `yaml:"name" json:"name"`

	// Webhook-specific fields.
	Path string       `yaml:"path,omitempty" json:"path,omitempty"`
	Auth *TriggerAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// S3-specific fields. Mode is "poll" (default) or "notification".
	Mode         string        `yaml:"mode,omitempty" json:"mode,omitempty"`
	Bucket       string        `yaml:"bucket,omitempty" json:"bucket,omitempty"`
	Prefix       string        `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Suffix       string        `yaml:"suffix,omitempty" json:"suffix,omitempty"`
	PollInterval time.Duration `yaml:"poll_interval,omitempty" json:"poll_interval,omitempty"`

	// Queue-specific fields.
	Topic  string `yaml:"topic,omitempty" json:"topic,omitempty"`
	Group  string `yaml:"group,omitempty" json:"group,omitempty"`
	Shared bool   `yaml:"shared,omitempty" json:"shared,omitempty"`

	// Common fields.
	// Params maps job param names to Go template expressions over event data.
	Params map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
	// Filter is a Go template expression; trigger fires only when trimmed result is "true".
	Filter string `yaml:"filter,omitempty" json:"filter,omitempty"`
	// DedupKey is a Go template expression whose result becomes JobRequest.UserKey.
	DedupKey string `yaml:"dedup_key,omitempty" json:"dedup_key,omitempty"`
	RateLimit *TriggerRateLimit `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
}

// Validate checks that required fields are present and consistent for the trigger type.
func (t *TriggerDefinition) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("trigger name is required")
	}
	switch t.Type {
	case "webhook":
		// Auth is optional; unauthenticated webhooks are valid for internal/testing use.
		// Callers that omit Auth must understand that the endpoint accepts all requests.
		if t.Auth != nil && t.Auth.Method == "" {
			return fmt.Errorf("trigger %q (webhook): auth.method is required when auth is specified", t.Name)
		}
		if t.Auth != nil && t.Auth.SecretConfig == "" {
			return fmt.Errorf("trigger %q (webhook): auth.secret_config is required when auth is specified", t.Name)
		}
	case "s3":
		if t.Bucket == "" {
			return fmt.Errorf("trigger %q (s3): bucket is required", t.Name)
		}
		if t.Mode != "" && t.Mode != "poll" && t.Mode != "notification" {
			return fmt.Errorf("trigger %q (s3): mode must be 'poll' or 'notification'", t.Name)
		}
		if t.Mode == "notification" && t.Topic == "" {
			return fmt.Errorf("trigger %q (s3): topic is required for notification mode", t.Name)
		}
	case "queue":
		if t.Topic == "" {
			return fmt.Errorf("trigger %q (queue): topic is required", t.Name)
		}
	default:
		return fmt.Errorf("trigger %q: type must be 'webhook', 's3', or 'queue', got %q", t.Name, t.Type)
	}
	if t.RateLimit != nil {
		if t.RateLimit.Max <= 0 {
			return fmt.Errorf("trigger %q: rate_limit.max must be > 0", t.Name)
		}
		if t.RateLimit.Window <= 0 {
			return fmt.Errorf("trigger %q: rate_limit.window must be > 0", t.Name)
		}
	}
	return nil
}
