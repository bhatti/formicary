package tracing

import (
	"time"

	"plexobject.com/formicary/internal/types"
)

// Config holds OpenTelemetry tracing configuration for provider initialization.
type Config struct {
	Enabled      bool          `yaml:"enabled" mapstructure:"enabled" json:"enabled"`
	Endpoint     string        `yaml:"endpoint" mapstructure:"endpoint" json:"endpoint"`
	ServiceName  string        `yaml:"service_name" mapstructure:"service_name" json:"service_name"`
	SampleRatio  float64       `yaml:"sample_ratio" mapstructure:"sample_ratio" json:"sample_ratio"`
	BatchTimeout time.Duration `yaml:"batch_timeout" mapstructure:"batch_timeout" json:"batch_timeout"`
}

// ConfigFromCommon builds a tracing Config from the user-facing TracingConfig
// (which lives in CommonConfig) and a service name. Returns a disabled config
// if tc is nil.
func ConfigFromCommon(tc *types.TracingConfig, serviceName string) *Config {
	cfg := &Config{
		ServiceName:  serviceName,
		Endpoint:     "http://localhost:4318",
		SampleRatio:  1.0,
		BatchTimeout: 5 * time.Second,
	}
	if tc == nil {
		return cfg
	}
	cfg.Enabled = tc.Enabled
	if tc.Endpoint != "" {
		cfg.Endpoint = tc.Endpoint
	}
	if tc.SampleRatio > 0 {
		cfg.SampleRatio = tc.SampleRatio
	}
	return cfg
}
