package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"strings"
	"time"

	"plexobject.com/formicary/internal/events"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"plexobject.com/formicary/internal/types"
)

// ServerConfig -- Defines the Server Config
type ServerConfig struct {
	Common                        types.CommonConfig `yaml:"common" mapstructure:"common"`
	DB                            DBConfig           `yaml:"db" mapstructure:"db"`
	Jobs                          JobsConfig         `yaml:"jobs" mapstructure:"jobs"`
	SMTP                          SMTPConfig         `yaml:"smtp" mapstructure:"smtp" env:"SMTP"`
	Notify                        NotifyConfig       `yaml:"notify" mapstructure:"notify"`
	GatewaySubscriptions          map[string]bool    `yaml:"gateway_subscriptions" mapstructure:"gateway_subscriptions"`
	URLPresignedExpirationMinutes time.Duration      `yaml:"url_presigned_expiration_minutes" mapstructure:"url_presigned_expiration_minutes"`
	DefaultArtifactExpiration     time.Duration      `yaml:"default_artifact_expiration" mapstructure:"default_artifact_expiration"`
	DefaultArtifactLimit          int                `yaml:"default_artifact_limit" mapstructure:"default_artifact_limit"`
	SubscriptionQuotaEnabled      bool               `yaml:"subscription_quota_enabled" mapstructure:"subscription_quota_enabled"`
}

// NotifyConfig -- Defines notification config
type NotifyConfig struct {
	EmailJobsTemplateFile      string `yaml:"email_jobs_template_file" mapstructure:"email_jobs_template_file"`
	SlackJobsTemplateFile      string `yaml:"slack_jobs_template_file" mapstructure:"slack_jobs_template_file"`
	VerifyEmailTemplateFile    string `yaml:"verify_email_template_file" mapstructure:"verify_email_template_file"`
	UserInvitationTemplateFile string `yaml:"user_invitation_template_file" mapstructure:"user_invitation_template_file"`
}

// SMTPConfig -- Defines email config
type SMTPConfig struct {
	FromEmail string `yaml:"from_email" mapstructure:"from_email" env:"FROM_EMAIL"`
	FromName  string `yaml:"from_name" mapstructure:"from_name" env:"FROM_NAME"`
	Provider  string `yaml:"provider" mapstructure:"provider" env:"PROVIDER"`
	APIKey    string `yaml:"api_key" mapstructure:"api_key" env:"API_KEY"`
	Username  string `yaml:"username" mapstructure:"username" env:"USERNAME"`
	Password  string `yaml:"password" mapstructure:"password" env:"PASSWORD"`
	Host      string `yaml:"host" mapstructure:"host" env:"HOST"`
	Port      int    `yaml:"port" mapstructure:"port" env:"PORT"`
}

// DBConfig -- Defines db config
type DBConfig struct {
	DataSource      string        `yaml:"data_source" mapstructure:"data_source" env:"DATA_SOURCE"`
	Type            string        `yaml:"type" mapstructure:"type" env:"TYPE"`
	EncryptionKey   string        `yaml:"encryption_key" mapstructure:"encryption_key" env:"ENCRYPTION_KEY"`
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
	MissingCronJobsInterval              time.Duration `yaml:"missing_cron_jobs_interval" mapstructure:"missing_cron_jobs_interval"`
	NotReadyJobsMaxWait                  time.Duration `yaml:"not_ready_max_wait" mapstructure:"not_ready_max_wait"`
	MaxScheduleAttempts                  int           `yaml:"max_schedule_attempts" mapstructure:"max_schedule_attempts"`
	DisableJobScheduler                  bool          `yaml:"disable_job_scheduler" mapstructure:"disable_job_scheduler"`
	MaxForkTaskletCapacity               int           `yaml:"max_fork_tasklet_capacity" mapstructure:"max_fork_tasklet_capacity"`
	MaxMessagingTaskletCapacity          int           `yaml:"max_messaging_tasklet_capacity" mapstructure:"max_messaging_tasklet_capacity"`
	MessagingEncryptionKey               string        `yaml:"messaging_encryption_key" mapstructure:"messaging_encryption_key"`
	ExpireArtifactsTaskletCapacity       int           `yaml:"expire_artifacts_tasklet_capacity" mapstructure:"expire_artifacts_tasklet_capacity"`
	MaxForkAwaitTaskletCapacity          int           `yaml:"max_fork_await_tasklet_capacity" mapstructure:"max_fork_await_tasklet_capacity"`
	LaunchTopicSuffix                    string        `yaml:"launch_topic_suffix" mapstructure:"launch_topic_suffix"`
}

// NewServerConfig -- Initializes the default config
func NewServerConfig(id string) (*ServerConfig, error) {
	var config ServerConfig
	viper.SetDefault("log_level", "info")
	viper.SetDefault("http_port", "7777")
	viper.SetDefault("db.type", "")
	viper.SetDefault("db.data_source", "")
	viper.SetDefault("smtp.from_email", "")
	viper.SetDefault("smtp.from_name", "")
	viper.SetDefault("smtp.provider", "")
	viper.SetDefault("smtp.api_key", "")
	viper.SetDefault("smtp.username", "")
	viper.SetDefault("smtp.password", "")
	viper.SetDefault("smtp.host", "")
	viper.SetDefault("smtp.port", "587")

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
	viper.SetDefault("common.messaging_provider", "REDIS_MESSAGING")
	viper.SetDefault("common.redis.host", "")
	viper.SetDefault("common.redis.port", "")
	viper.SetDefault("common.redis.password", "")
	viper.SetDefault("common.pulsar.url", "")

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
		"DB":         config.DB.Type,
		"Port":       config.Common.HTTPPort,
		"UsedConfig": viper.ConfigFileUsed(),
	}).Infof("loaded config file...")

	if err := config.Validate(); err != nil {
		return nil, err
	}
	config.Common.ID = id
	if config.Common.Debug {
		out, _ := yaml.Marshal(config)
		fmt.Printf("%s\n", out)
	}
	return &config, nil
}

// Validate validates notify config
func (c *JobsConfig) Validate() error {
	if c.AntRegistrationAliveTimeout == 0 {
		c.AntRegistrationAliveTimeout = 15 * time.Second
	}
	if c.JobSchedulerLeaderInterval == 0 {
		c.JobSchedulerLeaderInterval = 15 * time.Second
	}
	if c.AntReservationTimeout == 0 {
		c.AntReservationTimeout = 1 * time.Hour
	}
	if c.JobSchedulerCheckPendingJobsInterval == 0 {
		c.JobSchedulerCheckPendingJobsInterval = 1 * time.Second
	}
	if c.NotReadyJobsMaxWait == 0 {
		c.NotReadyJobsMaxWait = 30 * time.Second
	}
	if c.DBObjectCache == 0 {
		c.DBObjectCache = 30 * time.Second
	}
	if c.DBObjectCacheSize == 0 {
		c.DBObjectCacheSize = 10000
	}
	if c.MaxScheduleAttempts == 0 {
		c.MaxScheduleAttempts = 10000
	}
	if c.OrphanRequestsTimeout == 0 {
		c.OrphanRequestsTimeout = 60 * time.Second
	}
	if c.OrphanRequestsUpdateInterval == 0 {
		c.OrphanRequestsUpdateInterval = 15 * time.Second
	}
	if c.MissingCronJobsInterval == 0 {
		c.MissingCronJobsInterval = 60 * time.Second
	}
	if c.MaxForkTaskletCapacity == 0 {
		c.MaxForkTaskletCapacity = 100
	}
	if c.MaxForkAwaitTaskletCapacity == 0 {
		c.MaxForkAwaitTaskletCapacity = 100
	}
	if c.MaxMessagingTaskletCapacity == 0 {
		c.MaxMessagingTaskletCapacity = 100
	}
	if c.ExpireArtifactsTaskletCapacity == 0 {
		c.ExpireArtifactsTaskletCapacity = 100
	}
	return nil
}

// Validate validates
func (c *DBConfig) Validate() error {
	if c.MaxConcurrency == 0 {
		c.MaxConcurrency = 20
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 10
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 20
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 20
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 1 * time.Hour
	}
	if c.ConnMaxLifeTime == 0 {
		c.ConnMaxLifeTime = 4 * time.Hour
	}
	return nil
}

// Validate validates
func (c *ServerConfig) Validate() error {
	if err := c.Notify.Validate(c.Common.PublicDir); err != nil {
		return err
	}
	if err := c.Jobs.Validate(); err != nil {
		return err
	}
	if err := c.DB.Validate(); err != nil {
		return err
	}
	if err := c.Common.Auth.Validate(); err != nil {
		return err
	}
	if c.URLPresignedExpirationMinutes == 0 {
		c.URLPresignedExpirationMinutes = 60 * 12
	}
	if c.DefaultArtifactExpiration == 0 {
		c.DefaultArtifactExpiration = types.DefaultArtifactsExpirationDuration
	}
	if c.DefaultArtifactLimit == 0 {
		c.DefaultArtifactLimit = 100000
	}

	if len(c.GatewaySubscriptions) == 0 {
		c.GatewaySubscriptions = map[string]bool{
			"JobExecutionLifecycleEvent":  true,
			"TaskExecutionLifecycleEvent": true,
			"LogEvent":                    true,
		}
	}
	return c.Common.Validate(make([]string, 0))
}

// NewJobSchedulerLeaderEvent constructor
func (c *ServerConfig) NewJobSchedulerLeaderEvent() events.JobSchedulerLeaderEvent {
	return events.JobSchedulerLeaderEvent{
		BaseEvent: events.BaseEvent{
			Source:    c.Common.GetSource(),
			CreatedAt: time.Now(),
		},
	}
}

// GetResponseTopicAntRegistration response topic
func (c *ServerConfig) GetResponseTopicAntRegistration() string {
	return c.BuildResponseTopic("ant-registration")
}

// GetResponseTopicTaskReply response topic
func (c *ServerConfig) GetResponseTopicTaskReply() string {
	return c.BuildResponseTopic("reply")
}

// BuildResponseTopic response topic
func (c *ServerConfig) BuildResponseTopic(suffix string) string {
	return types.PersistentTopic(
		c.Common.MessagingProvider,
		c.Common.Pulsar.TopicTenant,
		c.Common.Pulsar.TopicNamespace,
		"task-"+suffix)
}

// GetJobExecutionLaunchTopic launch topic
func (c *ServerConfig) GetJobExecutionLaunchTopic() string {
	return types.PersistentTopic(
		c.Common.MessagingProvider,
		c.Common.Pulsar.TopicTenant,
		c.Common.Pulsar.TopicNamespace,
		"job-execution-launch"+c.Jobs.LaunchTopicSuffix)
}

// Validate validates smtp config
func (s *SMTPConfig) Validate() error {
	if s.FromEmail == "" {
		s.FromEmail = "formicary@plexobjects.com"
	}
	if s.FromName == "" {
		s.FromName = "Formicary Notifications"
	}
	if s.APIKey == "" {
		if s.Username == "" {
			s.Username = "formicary@plexobjects.com"
		}
		if s.Host == "" {
			s.Host = "localhost"
		}
		if s.Port == 0 {
			s.Port = 587
		}
		// TODO check other params
	} else {
		if s.Provider == "" {
			return types.NewValidationError(fmt.Errorf("smtp-provider not specified"))
		}
	}
	return nil
}

// Validate validates notify config
func (s *NotifyConfig) Validate(pubDir string) error {
	if s.EmailJobsTemplateFile == "" {
		s.EmailJobsTemplateFile = filepath.Join(pubDir, "views/notify/email_notify_job.html")
	}
	if s.SlackJobsTemplateFile == "" {
		s.SlackJobsTemplateFile = filepath.Join(pubDir, "views/notify/slack_notify_job.txt")
	}
	if s.VerifyEmailTemplateFile == "" {
		s.VerifyEmailTemplateFile = filepath.Join(pubDir, "views/notify/verify_email.html")
	}
	if s.UserInvitationTemplateFile == "" {
		s.UserInvitationTemplateFile = filepath.Join(pubDir, "views/notify/user_invitation.html")
	}
	return nil
}
