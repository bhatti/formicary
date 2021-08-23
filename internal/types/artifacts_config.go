package types

import (
	"fmt"
	"strings"
	"time"
)

// ArtifactsWhen type alias for when artifact can be uploaded
type ArtifactsWhen string

// KeysDigest stores digest of key files
const KeysDigest = "KeysDigest"

// DefaultArtifactsExpirationDuration default expiration for artifacts
const DefaultArtifactsExpirationDuration = time.Hour * 24 * 1000

// DefaultCacheExpirationDuration default expiration for artifacts
const DefaultCacheExpirationDuration = time.Hour * 24 * 30

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
	Paths        []string      `json:"paths,omitempty" yaml:"paths,omitempty"`
	ExpiresAfter time.Duration `json:"expires_after,omitempty" yaml:"expires_after,omitempty"`
	When         ArtifactsWhen `json:"when,omitempty" yaml:"when,omitempty"`
}

// NewArtifactsConfig - constructor
func NewArtifactsConfig() ArtifactsConfig {
	return ArtifactsConfig{
		Paths: make([]string, 0),
	}
}

// GetPathsAndExpiration finds path and expiration time based on task status
func (ac *ArtifactsConfig) GetPathsAndExpiration(succeeded bool) ([]string, time.Time) {
	if ac.Valid(succeeded) {
		return ac.Paths, ac.Expiration()
	}
	return make([]string, 0), ac.Expiration()
}

// Expiration expiration time for artifacts
func (ac *ArtifactsConfig) Expiration() time.Time {
	if ac.ExpiresAfter > 0 {
		return time.Now().Add(ac.ExpiresAfter)
	}
	return time.Now().Add(DefaultArtifactsExpirationDuration)
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
	Paths        []string      `json:"paths,omitempty" yaml:"paths,omitempty"`
	Key          string        `json:"key,omitempty" yaml:"key,omitempty"`
	KeyPaths     []string      `json:"key_paths,omitempty" yaml:"key_paths,omitempty"`
	KeyDigest    string        `json:"key_digest,omitempty" yaml:"-"`
	NewKeyDigest string        `json:"-" yaml:"-"`
	ExpiresAfter time.Duration `json:"expires_after,omitempty" yaml:"expires_after,omitempty"`
	matched      bool
}

// NewCacheConfig - constructor
func NewCacheConfig() CacheConfig {
	return CacheConfig{
		Paths:    make([]string, 0),
		KeyPaths: make([]string, 0),
	}
}

// Expiration expiration time for artifacts
func (ac *CacheConfig) Expiration() time.Time {
	if ac.ExpiresAfter > 0 {
		return time.Now().Add(ac.ExpiresAfter)
	}
	return time.Now().Add(DefaultCacheExpirationDuration)
}

// Valid - checks if path/keys are specified
func (ac *CacheConfig) Valid() bool {
	return len(ac.Paths) > 0 && (len(ac.KeyPaths) > 0 || ac.Key != "")
}

func (ac *CacheConfig) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("paths: %v", ac.Paths))
	if len(ac.KeyPaths) > 0 {
		sb.WriteString(fmt.Sprintf(", keypaths: %v", ac.KeyPaths))
	} else if ac.Key != "" {
		sb.WriteString(fmt.Sprintf(", key: %s", ac.Key))
	}
	return sb.String()
}
