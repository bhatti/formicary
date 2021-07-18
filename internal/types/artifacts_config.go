package types

import (
	"time"
)

// ArtifactsWhen type alias for when artifact can be uploaded
type ArtifactsWhen string

// KeysDigest stores digest of key files
const KeysDigest = "KeysDigest"

const (
	// ArtifactsWhenOnSuccess on-success
	ArtifactsWhenOnSuccess ArtifactsWhen = "onSuccess"
	// ArtifactsWhenOnFailure on-failure
	ArtifactsWhenOnFailure ArtifactsWhen = "onFailure"
	// ArtifactsWhenAlways default
	ArtifactsWhenAlways ArtifactsWhen = "always"
)

// ArtifactsConfig defines artifacts to upload
type ArtifactsConfig struct {
	Paths         []string      `json:"paths,omitempty" yaml:"paths,omitempty"`
	ExpiresInDays time.Duration `json:"expires_in_days,omitempty" yaml:"expires_in_days,omitempty"`
	When          ArtifactsWhen `json:"when,omitempty" yaml:"when,omitempty"`
}

// NewArtifacts - constructor
func NewArtifacts() ArtifactsConfig {
	return ArtifactsConfig{
		Paths: make([]string, 0),
	}
}

// GetPathsAndExpiration finds path and expiration time based on task status
func (ac *ArtifactsConfig) GetPathsAndExpiration(succeeded bool) ([]string, *time.Time) {
	if ac.Valid(succeeded) {
		return ac.Paths, ac.Expiration()
	}
	return make([]string, 0), nil
}

// Expiration expiration time for artifacts
func (ac *ArtifactsConfig) Expiration() *time.Time {
	if ac.ExpiresInDays > 0 {
		expires := time.Now().Add(ac.ExpiresInDays * time.Hour * 24)
		return &expires
	}
	return nil
}

// Valid checks status
func (ac *ArtifactsConfig) Valid(succeeded bool) bool {
	if ac.When == "" {
		ac.When = ArtifactsWhenAlways
	}
	if succeeded {
		return ac.When == ArtifactsWhenAlways || ac.When == ArtifactsWhenOnSuccess
	}
	return ac.When == ArtifactsWhenAlways || ac.When == ArtifactsWhenOnFailure
}

// CacheConfig defines cache to upload
type CacheConfig struct {
	Paths         []string      `json:"paths,omitempty" yaml:"paths,omitempty"`
	Key           string        `json:"key,omitempty" yaml:"key,omitempty"`
	KeyPaths      []string      `json:"key_paths,omitempty" yaml:"key_paths,omitempty"`
	KeyDigest     string        `json:"key_digest,omitempty" yaml:"-"`
	NewKeyDigest  string        `json:"-" yaml:"-"`
	ExpiresInDays time.Duration `json:"expires_in_days,omitempty" yaml:"expires_in_days,omitempty"`
	matched       bool
}

// NewCacheConfig - constructor
func NewCacheConfig() CacheConfig {
	return CacheConfig{
		Paths:    make([]string, 0),
		KeyPaths: make([]string, 0),
	}
}

// Expiration expiration time for artifacts
func (ac *CacheConfig) Expiration() *time.Time {
	if ac.ExpiresInDays > 0 {
		expires := time.Now().Add(ac.ExpiresInDays * time.Hour * 24)
		return &expires
	}
	return nil
}

// Valid - checks if path/keys are specified
func (ac *CacheConfig) Valid() bool {
	return len(ac.Paths) > 0 && (len(ac.KeyPaths) > 0 || ac.Key != "")
}
