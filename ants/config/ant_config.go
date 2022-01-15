package config

import (
	"errors"
	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/crypto"
	"time"

	"plexobject.com/formicary/internal/types"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

// Registry -- Registry Config
type Registry struct {
	Server     string           `yaml:"registry" mapstructure:"registry"`
	Username   string           `yaml:"username" mapstructure:"username"`
	Password   string           `yaml:"password" mapstructure:"password"`
	PullPolicy types.PullPolicy `yaml:"pull_policy" mapstructure:"pull_policy"`
}

// AntConfig -- Defines the default ant config
type AntConfig struct {
	types.CommonConfig     `yaml:"common" mapstructure:"common"`
	Tags                   []string           `yaml:"tags" mapstructure:"tags"`
	Methods                []types.TaskMethod `yaml:"methods" mapstructure:"methods"`
	Docker                 DockerConfig       `yaml:"docker" mapstructure:"docker"`
	Kubernetes             KubernetesConfig   `yaml:"kubernetes" mapstructure:"kubernetes"`
	DefaultShell           []string           `yaml:"default_shell" mapstructure:"default_shell"`
	OutputLimit            int                `yaml:"output_limit" mapstructure:"output_limit"`
	MaxCapacity            int                `yaml:"max_capacity" mapstructure:"max_capacity"`
	TerminationGracePeriod time.Duration      `yaml:"termination_grace_period" json:"termination_grace_period" mapstructure:"termination_grace_period"`
	AwaitRunningPeriod     time.Duration      `yaml:"await_running_period" json:"await_running_period" mapstructure:"await_running_period"`
	PollInterval           int64              `yaml:"poll_interval" json:"poll_interval" mapstructure:"poll_interval"`
	PollTimeout            int64              `yaml:"poll_timeout" json:"poll_timeout" mapstructure:"poll_timeout"`

	PollIntervalBeforeShutdown time.Duration `yaml:"poll_interval_before_shutdown" json:"poll_interval_before_shutdown" mapstructure:"poll_interval_before_shutdown"`
	PollAttemptsBeforeShutdown int           `yaml:"poll_attempts_before_shutdown" json:"poll_attempts_before_shutdown" mapstructure:"poll_attempts_before_shutdown"`
	antStartedAt               time.Time
}

// NewAntConfig -- Initializes the ant config
func NewAntConfig(id string) (*AntConfig, error) {
	var config *AntConfig

	viper.SetDefault("log_level", "info")
	viper.SetDefault("default_shell", []string{
		"sh",
		"-c",
		"if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/   sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell  not found\n\texit 1\nfi\n\n",
	})
	viper.SetDefault("output_limit", 64*1024*1024)
	viper.SetDefault("termination_grace_period_seconds", "10")
	viper.SetDefault("poll_interval", "3")
	viper.SetDefault("poll_timeout", "0")
	viper.SetDefault("await_running_period_seconds", "0")

	// Docker Defaults
	viper.SetDefault("docker.host", "")
	viper.SetDefault("docker.registry.server", "index.docker.io")
	viper.SetDefault("docker.registry.username", "")
	viper.SetDefault("docker.registry.password", "")

	// Kubernetes Defaults
	viper.SetDefault("kubernetes.namespace", "default")
	viper.SetDefault("kubernetes.service_account", "default")
	viper.SetDefault("kubernetes.registry.server", "")
	viper.SetDefault("kubernetes.registry.username", "")
	viper.SetDefault("kubernetes.registry.password", "")

	if err := viper.ReadInConfig(); err == nil {
		log.WithFields(log.Fields{
			"Component":  "AntConfig",
			"ID":         id,
			"UsedConfig": viper.ConfigFileUsed(),
		}).Infof("loading config file...")
	} else {
		log.WithFields(log.Fields{
			"Component":  "AntConfig",
			"ID":         id,
			"Error":      err,
			"UsedConfig": viper.ConfigFileUsed(),
		}).Errorf("failed to load config file")
		return nil, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
		return nil, err
	}

	config.ID = id
	config.antStartedAt = time.Now()
	return config, nil
}

// Validate config
func (c *AntConfig) Validate() error {
	if c.Methods == nil || len(c.Methods) == 0 {
		return errors.New("methods is not set")
	}
	if err := c.Docker.Validate(); err != nil {
		return err
	}
	if err := c.Kubernetes.Validate(); err != nil {
		return err
	}
	if c.MaxCapacity <= 0 {
		c.MaxCapacity = 10
	}
	if c.PollIntervalBeforeShutdown == 0 {
		c.PollIntervalBeforeShutdown = 5 * time.Second
	}
	if c.PollAttemptsBeforeShutdown == 0 {
		c.PollAttemptsBeforeShutdown = 5
	}
	if c.EncryptionKey == "" {
		if b, err := crypto.GenerateKey(32); err == nil {
			c.EncryptionKey = string(b)
		} else {
			c.EncryptionKey = uuid.NewV4().String()
		}
	}
	return c.CommonConfig.Validate(c.Tags)
}

// NewAntRegistration constructor for ant registration
func (c *AntConfig) NewAntRegistration() *types.AntRegistration {
	return &types.AntRegistration{
		AntID:        c.ID,
		MaxCapacity:  c.MaxCapacity,
		Tags:         c.Tags,
		Methods:      c.Methods,
		CreatedAt:    time.Now(),
		AntStartedAt: c.antStartedAt,
	}
}

// GetPollInterval returns poll interval in secs
func (c *AntConfig) GetPollInterval() time.Duration {
	if c.PollInterval <= 0 {
		c.PollInterval = 3
	} else if c.PollInterval > 30 {
		c.PollInterval = 30
	}
	return time.Duration(c.PollInterval) * time.Second
}

// GetShutdownTimeout returns grace shutdown timeout in secs
func (c *AntConfig) GetShutdownTimeout() time.Duration {
	if c.TerminationGracePeriod > 5*time.Minute {
		c.TerminationGracePeriod = 5 * time.Minute
	}
	if c.TerminationGracePeriod == 0 {
		c.TerminationGracePeriod = 5 * time.Second
	}
	return c.TerminationGracePeriod
}

// GetAwaitRunningPeriod returns grace period to start container in secs
func (c *AntConfig) GetAwaitRunningPeriod() time.Duration {
	if c.AwaitRunningPeriod > 30*time.Minute {
		c.AwaitRunningPeriod = 30 * time.Minute
	}
	if c.AwaitRunningPeriod == 0 {
		c.AwaitRunningPeriod = 10 * time.Minute
	}
	return c.AwaitRunningPeriod
}

// GetPollTimeout returns poll timeout in secs
func (c *AntConfig) GetPollTimeout() time.Duration {
	if c.PollTimeout <= 0 {
		c.PollTimeout = 500
	} else if c.PollInterval > 10000 {
		c.PollTimeout = 10000
	}
	return time.Duration(c.PollTimeout) * time.Second
}

// GetPollAttempts returns number of poll attempts
func (c *AntConfig) GetPollAttempts() int64 {
	_ = c.GetPollTimeout()
	if c.PollInterval <= 0 {
		c.PollInterval = 3
	}

	return c.PollTimeout / c.PollInterval
}
