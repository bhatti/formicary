package config

import (
	"strings"
	"time"

	"plexobject.com/formicary/internal/events"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"plexobject.com/formicary/internal/types"
)

// TaskResponseTopicPrefix response topic
const TaskResponseTopicPrefix = "task-response-topic-"

// ServerConfig -- Defines the Server Config
type ServerConfig struct {
	types.CommonConfig            `yaml:"common" mapstructure:"common"`
	DB                            DBConfig        `yaml:"db" mapstructure:"db" env:"DB"`
	Jobs                          JobsConfig      `yaml:"jobs" mapstructure:"jobs"`
	Email                         EmailConfig     `yaml:"email" mapstructure:"email"`
	GatewaySubscriptions          map[string]bool `yaml:"gateway_subscriptions" mapstructure:"gateway_subscriptions"`
	URLPresignedExpirationMinutes time.Duration   `yaml:"url_presigned_expiration_minutes" mapstructure:"url_presigned_expiration_minutes"`
	SubscriptionQuotaEnabled      bool            `yaml:"subscription_quota_enabled" mapstructure:"subscription_quota_enabled"`
}

// EmailConfig -- Defines email config
type EmailConfig struct {
	FromEmail    string `yaml:"from_email" mapstructure:"from_email"`
	Provider     string `yaml:"provider" mapstructure:"provider"`
	APIKey       string `yaml:"api_key" mapstructure:"api_key"`
	TemplateFile string `yaml:"template_file" mapstructure:"template_file"`
}

// DBConfig -- Defines db config
type DBConfig struct {
	DataSource      string        `yaml:"data_source" mapstructure:"data_source"`
	DBType          string        `yaml:"db_type" mapstructure:"db_type"`
	EncryptionKey   string        `yaml:"encryption_key" mapstructure:"encryption_key"`
	MaxIdleConns    int           `yaml:"max_idle_connections" mapstructure:"max_idle_connections"`
	MaxOpenConns    int           `yaml:"max_open_connections" mapstructure:"max_open_connections"`
	MaxConcurrency  int           `yaml:"max_concurrency" mapstructure:"max_concurrency"`
	ConnMaxIdleTime time.Duration `yaml:"connection_max_idle_time" mapstructure:"connection_max_idle_time"`
	ConnMaxLifeTime time.Duration `yaml:"connection_max_life_time" mapstructure:"connection_max_life_time"`
}

// JobsConfig -- Defines job scheduler/tasks related config
type JobsConfig struct {
	AntReservationTimeout                time.Duration `yaml:"ant_reservation_timeout" mapstructure:"ant_reservation_timeout"`
	AntRegistrationAliveTimeout          time.Duration `yaml:"ant_registration_alive_timeout" mapstructure:"ant_registration_alive_timeout"`
	JobSchedulerLeaderInterval           time.Duration `yaml:"job_scheduler_leader_interval" mapstructure:"job_scheduler_leader_interval"`
	JobSchedulerCheckPendingJobsInterval time.Duration `yaml:"job_scheduler_check_pending_jobs_interval" mapstructure:"job_scheduler_check_pending_jobs_interval"`
	DBObjectCache                        time.Duration `yaml:"db_object_cache" mapstructure:"db_object_cache"`
	DBObjectCacheSize                    int64         `yaml:"db_object_cache_size" mapstructure:"db_object_cache_size"`
	OrphanRequestsTimeout                time.Duration `yaml:"orphan_requests_timeout" mapstructure:"orphan_requests_timeout"`
	OrphanRequestsUpdateInterval         time.Duration `yaml:"orphan_requests_update_interval" mapstructure:"orphan_requests_update_interval"`
	NotReadyJobsMaxWait                  time.Duration `yaml:"not_ready_max_wait" mapstructure:"not_ready_max_wait"`
	MaxScheduleAttempts                  int           `yaml:"max_schedule_attempts" mapstructure:"max_schedule_attempts"`
	DisableJobScheduler                  bool          `yaml:"disable_job_scheduler" mapstructure:"disable_job_scheduler"`
	MaxForkTaskletCapacity               int           `yaml:"max_fork_tasklet_capacity" mapstructure:"max_fork_tasklet_capacity"`
	MaxForkAwaitTaskletCapacity          int           `yaml:"max_fork_await_tasklet_capacity" mapstructure:"max_fork_await_tasklet_capacity"`
}

// NewServerConfig -- Initializes the default config
func NewServerConfig(id string) (*ServerConfig, error) {
	var config ServerConfig
	viper.SetDefault("log_level", "info")
	viper.SetDefault("http_port", "7000")
	viper.SetDefault("db.db_type", "")
	viper.SetDefault("db.data_source", "")

	viper.SetDefault("common.auth.enabled", "false")
	viper.SetDefault("common.auth.session_key", "")
	viper.SetDefault("common.auth.google_client_id", "")
	viper.SetDefault("common.auth.google_client_secret", "")
	viper.SetDefault("common.auth.google_callback_host", "")
	viper.SetDefault("common.auth.github_client_id", "")
	viper.SetDefault("common.auth.github_client_secret", "")
	viper.SetDefault("common.auth.github_callback_host", "")

	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.WithFields(log.Fields{
			"Component":  "ServerConfig",
			"ID":         id,
			"Error":      err,
			"UsedConfig": viper.ConfigFileUsed(),
		}).Errorf("failed to load config file")
		return nil, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"Component":  "ServerConfig",
		"ID":         id,
		"DB":         config.DB.DBType,
		"Port":       config.HTTPPort,
		"UsedConfig": viper.ConfigFileUsed(),
	}).Infof("loaded config file...")

	if err := config.Validate(); err != nil {
		return nil, err
	}
	config.ID = id
	return &config, nil
}

// Validate validates
func (c *ServerConfig) Validate() error {
	if c.Jobs.AntRegistrationAliveTimeout == 0 {
		c.Jobs.AntRegistrationAliveTimeout = 15 * time.Second
	}
	if c.Jobs.JobSchedulerLeaderInterval == 0 {
		c.Jobs.JobSchedulerLeaderInterval = 15 * time.Second
	}
	if c.Jobs.AntReservationTimeout == 0 {
		c.Jobs.AntReservationTimeout = 1 * time.Hour
	}
	if c.Jobs.JobSchedulerCheckPendingJobsInterval == 0 {
		c.Jobs.JobSchedulerCheckPendingJobsInterval = 1 * time.Second
	}
	if c.Jobs.NotReadyJobsMaxWait == 0 {
		c.Jobs.NotReadyJobsMaxWait = 30 * time.Second
	}
	if c.Jobs.DBObjectCache == 0 {
		c.Jobs.DBObjectCache = 30 * time.Second
	}
	if c.Jobs.DBObjectCacheSize == 0 {
		c.Jobs.DBObjectCacheSize = 10000
	}
	if c.Jobs.MaxScheduleAttempts == 0 {
		c.Jobs.MaxScheduleAttempts = 10000
	}
	if c.Jobs.OrphanRequestsTimeout == 0 {
		c.Jobs.OrphanRequestsTimeout = 60 * time.Second
	}
	if c.Jobs.OrphanRequestsUpdateInterval == 0 {
		c.Jobs.OrphanRequestsUpdateInterval = 15 * time.Second
	}
	if c.URLPresignedExpirationMinutes == 0 {
		c.URLPresignedExpirationMinutes = 60 * 12
	}
	if c.Jobs.MaxForkTaskletCapacity == 0 {
		c.Jobs.MaxForkTaskletCapacity = 100
	}
	if c.Jobs.MaxForkAwaitTaskletCapacity == 0 {
		c.Jobs.MaxForkAwaitTaskletCapacity = 100
	}
	if c.DB.MaxConcurrency == 0 {
		c.DB.MaxConcurrency = 100
	}
	if c.DB.MaxIdleConns == 0 {
		c.DB.MaxIdleConns = 10
	}
	if c.DB.MaxOpenConns == 0 {
		c.DB.MaxOpenConns = 20
	}
	if c.DB.MaxOpenConns == 0 {
		c.DB.MaxOpenConns = 20
	}
	if c.DB.ConnMaxIdleTime == 0 {
		c.DB.ConnMaxIdleTime = 1 * time.Hour
	}
	if c.DB.ConnMaxLifeTime == 0 {
		c.DB.ConnMaxLifeTime = 4 * time.Hour
	}
	if c.Auth.MaxAge == 0 {
		c.Auth.MaxAge = 7 * 24 * time.Hour
	}
	if c.Auth.TokenMaxAge == 0 {
		c.Auth.TokenMaxAge = 30 * 3 * 24 * time.Hour
	}
	if c.Auth.GoogleCallbackHost == "" {
		c.Auth.GoogleCallbackHost = "localhost"
	}
	if c.Auth.GithubCallbackHost == "" {
		c.Auth.GithubCallbackHost = "localhost"
	}
	if c.Auth.CookieName == "" {
		c.Auth.CookieName = "formicary-session"
	}

	if len(c.GatewaySubscriptions) == 0 {
		c.GatewaySubscriptions = map[string]bool{
			"JobExecutionLifecycleEvent":  true,
			"TaskExecutionLifecycleEvent": true,
			"LogEvent":                    true,
		}
	}
	return c.CommonConfig.Validate(make([]string, 0))
}

// NewJobSchedulerLeaderEvent constructor
func (c *ServerConfig) NewJobSchedulerLeaderEvent() events.JobSchedulerLeaderEvent {
	return events.JobSchedulerLeaderEvent{
		BaseEvent: events.BaseEvent{
			Source:    c.GetSource(),
			CreatedAt: time.Now(),
		},
	}
}

// GetResponseTopic response topic
func (c *ServerConfig) GetResponseTopic(suffix string) string {
	return types.PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		TaskResponseTopicPrefix+suffix)
}

// GetJobExecutionLaunchTopic launch topic
func (c *ServerConfig) GetJobExecutionLaunchTopic() string {
	return types.PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		types.JobExecutionLaunchTopic)
}

// GetJobSchedulerLeaderTopic leader election event
func (c *ServerConfig) GetJobSchedulerLeaderTopic() string {
	return types.NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		types.JobSchedulerLeaderTopic)
}
