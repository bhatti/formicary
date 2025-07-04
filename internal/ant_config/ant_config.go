package ant_config

import (
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
	"plexobject.com/formicary/internal/crypto"
	"strings"
	"time"

	"plexobject.com/formicary/internal/types"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

var defaultShell = []string{
	"sh",
	"-c",
	"if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/   sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell  not found\n\texit 1\nfi\n\n",
}

// Registry -- Registry Config
type Registry struct {
	Server     string           `yaml:"registry" mapstructure:"registry"`
	Username   string           `yaml:"username" mapstructure:"username"`
	Password   string           `yaml:"password" mapstructure:"password"`
	PullPolicy types.PullPolicy `yaml:"pull_policy" mapstructure:"pull_policy"`
}

// AntConfig -- Defines the default ant config
type AntConfig struct {
	Common                 types.CommonConfig `yaml:"common" mapstructure:"common"`
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
	viper.SetDefault("default_shell", defaultShell)
	viper.SetDefault("common.user_agent", "")
	viper.SetDefault("common.proxy_url", "")
	viper.SetDefault("common.external_base_url", "")
	viper.SetDefault("common.public_dir", "")
	viper.SetDefault("common.http_port", "7777")
	viper.SetDefault("common.debug", "false")

	viper.SetDefault("common.auth.enabled", "false")
	viper.SetDefault("common.auth.session_key", "")
	viper.SetDefault("common.auth.google_client_id", "")
	viper.SetDefault("common.auth.google_client_secret", "")
	viper.SetDefault("common.auth.google_callback_host", "")
	viper.SetDefault("common.auth.github_client_id", "")
	viper.SetDefault("common.auth.github_client_secret", "")
	viper.SetDefault("common.auth.github_callback_host", "")
	viper.SetDefault("common.s3.endpoint", "")
	viper.SetDefault("common.s3.access_key_id", "")
	viper.SetDefault("common.s3.secret_access_key", "")
	viper.SetDefault("common.s3.password", "")
	viper.SetDefault("common.s3.token", "")
	viper.SetDefault("common.s3.region", "")
	viper.SetDefault("common.s3.prefix", "")
	viper.SetDefault("common.s3.bucket", "")
	viper.SetDefault("common.queue.provider", "")
	viper.SetDefault("common.redis.host", "")
	viper.SetDefault("common.redis.port", "")
	viper.SetDefault("common.redis.password", "")
	viper.SetDefault("common.pulsar.url", "")

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

	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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
	if config.Common.Debug {
		out, _ := yaml.Marshal(config)
		fmt.Printf("%s\n", out)
	}

	config.Common.ID = id
	config.antStartedAt = time.Now()
	return config, nil
}

// Validate config
func (c *AntConfig) Validate() error {
	if err := c.Common.Validate(); err != nil {
		return err
	}
	if len(c.Methods) == 0 {
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
	if c.PollInterval <= 0 {
		c.PollInterval = 3
	}
	if c.Common.EncryptionKey == "" {
		if b, err := crypto.GenerateKey(32); err == nil {
			c.Common.EncryptionKey = string(b)
		} else {
			c.Common.EncryptionKey = ulid.Make().String()
		}
	}
	if c.OutputLimit <= 0 {
		c.OutputLimit = 64 * 1024 * 1024
	}
	if len(c.DefaultShell) == 0 {
		c.DefaultShell = defaultShell
	}
	return nil
}

// NewAntRegistration constructor for ant registration
func (c *AntConfig) NewAntRegistration() *types.AntRegistration {
	return &types.AntRegistration{
		AntID:        c.Common.ID,
		MaxCapacity:  c.MaxCapacity,
		Tags:         c.Tags,
		AutoRefresh:  true,
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
