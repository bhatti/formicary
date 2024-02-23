package types

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"plexobject.com/formicary/internal/buildversion"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const httpPrefix = "http"
const httpsPrefix = "https"

// MessagingProvider defines enum for messaging provider
type MessagingProvider string

const (
	// RedisMessagingProvider uses redis
	RedisMessagingProvider MessagingProvider = "REDIS_MESSAGING"

	// PulsarMessagingProvider uses apache pulsar
	PulsarMessagingProvider MessagingProvider = "PULSAR_MESSAGING"

	// KafkaMessagingProvider uses apache kafka
	KafkaMessagingProvider MessagingProvider = "KAFKA_MESSAGING"
)

var listeningForStackTraceDumps = false

// CommonConfig -- common config between ant and server
type CommonConfig struct {
	ID                         string             `yaml:"id" mapstructure:"id"`
	UserAgent                  string             `yaml:"user_agent" mapstructure:"user_agent" env:"USER_AGENT"`
	ProxyURL                   string             `yaml:"proxy_url" mapstructure:"proxy_url" env:"PROXY_URL"`
	ExternalBaseURL            string             `yaml:"external_base_url" mapstructure:"external_base_url"`
	BlockUserAgents            []string           `yaml:"block_user_agents" mapstructure:"block_user_agents"`
	PublicDir                  string             `yaml:"public_dir" mapstructure:"public_dir" env:"PUBLIC_DIR"`
	HTTPPort                   int                `yaml:"http_port" mapstructure:"http_port" env:"HTTP_PORT"`
	Pulsar                     PulsarConfig       `yaml:"pulsar" mapstructure:"pulsar"`
	Kafka                      KafkaConfig        `yaml:"kafka" mapstructure:"kafka"`
	S3                         S3Config           `yaml:"s3" mapstructure:"s3" env:"S3"`
	Redis                      RedisConfig        `yaml:"redis" mapstructure:"redis" env:"REDIS"`
	Auth                       AuthConfig         `yaml:"auth" mapstructure:"auth" env:"AUTH"`
	MessagingProvider          MessagingProvider  `yaml:"messaging_provider" mapstructure:"messaging_provider"`
	ContainerReaperInterval    time.Duration      `yaml:"container_reaper_interval" mapstructure:"container_reaper_interval"`
	MonitorInterval            time.Duration      `yaml:"monitor_interval" mapstructure:"monitor_interval"`
	MonitoringURLs             map[string]string  `yaml:"monitoring_urls" mapstructure:"monitoring_urls"`
	RegistrationInterval       time.Duration      `yaml:"registration_interval" mapstructure:"registration_interval"`
	DeadJobIDsEventsInterval   time.Duration      `yaml:"dead_job_ids_events_interval" mapstructure:"dead_job_ids_events_interval"`
	MaxStreamingLogMessageSize int                `yaml:"max_streaming_log_message_size" mapstructure:"max_streaming_log_message_size" json:"max_streaming_log_message_size"`
	MaxJobTimeout              time.Duration      `yaml:"max_job_timeout" mapstructure:"max_job_timeout"`
	MaxTaskTimeout             time.Duration      `yaml:"max_task_timeout" mapstructure:"max_task_timeout"`
	RateLimitPerSecond         float64            `yaml:"rate_limit_sec" mapstructure:"rate_limit_sec" json:"rate_limit_sec"`
	ShuttingDown               bool               `yaml:"-" mapstructure:"-" json:"-"`
	Development                bool               `yaml:"development" mapstructure:"development" json:"development"`
	Version                    *buildversion.Info `yaml:"-" mapstructure:"-" json:"-"`
	EncryptionKey              string             `json:"encryption_key" mapstructure:"encryption_key"`
	Debug                      bool               `yaml:"debug" env:"DEBUG"`
	blockUserAgentsMap         map[string]bool    `yaml:"-" mapstructure:"-"`
}

// PersistentTopic builds persistent topic
func PersistentTopic(provider MessagingProvider, tenant string, namespace string, name string) string {
	if provider == PulsarMessagingProvider {
		return fmt.Sprintf("persistent://%s/%s/formicary-%s", tenant, namespace, name)
	}
	return fmt.Sprintf("formicary-queue-%s", name)
}

// NonPersistentTopic builds non-persistent topic
func NonPersistentTopic(provider MessagingProvider, tenant string, namespace string, name string) string {
	if provider == PulsarMessagingProvider {
		return fmt.Sprintf("non-persistent://%s/%s/formicary-%s", tenant, namespace, name)
	}
	return fmt.Sprintf("formicary-topic-%s", name)
}

// GetExternalBaseURL url
func (c *CommonConfig) GetExternalBaseURL() string {
	if c.ExternalBaseURL != "" {
		return c.ExternalBaseURL
	} else if c.Auth.Secure {
		return fmt.Sprintf("%s://%s:%d", httpsPrefix, c.Auth.GithubCallbackHost, c.HTTPPort)
	} else {
		return fmt.Sprintf("%s://%s:%d", httpPrefix, c.Auth.GithubCallbackHost, c.HTTPPort)
	}
}

// AddSignalHandlerForStackTrace listen for signal to print stack trace
func (c *CommonConfig) AddSignalHandlerForStackTrace() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	listeningForStackTraceDumps = true

	go func() {
		for sig := range signals {
			stacktrace := make([]byte, 8192)
			length := runtime.Stack(stacktrace, true)
			logrus.WithFields(logrus.Fields{
				"Component": "CommonConfig",
				"ID":        c.ID,
				"Signal":    sig,
			}).Warnf("dumping stack trace")
			fmt.Println(string(stacktrace[:length]))
		}
	}()
	logrus.WithFields(logrus.Fields{
		"Component": "CommonConfig",
		"ID":        c.ID,
		"Signal":    syscall.SIGHUP,
	}).Infof("adding signal handler to dump stack trace")
}

// AddSignalHandlerForShutdown listen for signal to shut down cleanly
func (c *CommonConfig) AddSignalHandlerForShutdown(shutdownFunc context.CancelFunc) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGQUIT, syscall.SIGTERM)

	countSignalsReceived := 0
	go func() {
		for sig := range signals {
			c.ShuttingDown = true
			if countSignalsReceived == 0 {
				shutdownFunc()
			}
			forced := countSignalsReceived > 1
			// notify only once
			if forced {
				logrus.WithFields(logrus.Fields{
					"Component":            "CommonConfig",
					"ID":                   c.ID,
					"Signal":               sig,
					"CountSignalsReceived": countSignalsReceived,
				}).Error("forced shutdown")
				os.Exit(0)
			} else {
				logrus.WithFields(logrus.Fields{
					"Component":            "CommonConfig",
					"ID":                   c.ID,
					"Signal":               sig,
					"CountSignalsReceived": countSignalsReceived,
				}).Warn("shutting down")
			}
			countSignalsReceived++
		}
	}()
}

// GetSource source
func (c *CommonConfig) GetSource() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s@%s", c.ID, host)
}

// BlockUserAgent returns true if user-agent is blocked
func (c *CommonConfig) BlockUserAgent(agent string) bool {
	if agent == "" || len(c.blockUserAgentsMap) == 0 {
		return false
	}
	parts := strings.Split(agent, " ")
	return c.blockUserAgentsMap[parts[0]]
}

// GetRegistrationTopic - registration topic
func (c *CommonConfig) GetRegistrationTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		RegistrationTopic)
}

// GetJobExecutionLaunchTopic launch topic
func (c *CommonConfig) GetJobExecutionLaunchTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-execution-launch")
}

// GetJobSchedulerLeaderTopic leader election event
func (c *CommonConfig) GetJobSchedulerLeaderTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-scheduler-leader")
}

// GetContainerLifecycleTopic - container lifecycle topic
func (c *CommonConfig) GetContainerLifecycleTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"container-lifecycle")
}

// GetRecentlyCompletedJobsTopic topic
func (c *CommonConfig) GetRecentlyCompletedJobsTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"recently-completed-job-ids")
}

// GetJobDefinitionLifecycleTopic topic
func (c *CommonConfig) GetJobDefinitionLifecycleTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-definition-lifecycle")
}

// GetJobRequestLifecycleTopic topic
func (c *CommonConfig) GetJobRequestLifecycleTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-request-lifecycle")
}

// GetJobExecutionLifecycleTopic topic
func (c *CommonConfig) GetJobExecutionLifecycleTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-execution-lifecycle")
}

// GetJobWebhookTopic topic
func (c *CommonConfig) GetJobWebhookTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"job-webhook-lifecycle")
}

// GetTaskWebhookTopic topic
func (c *CommonConfig) GetTaskWebhookTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"task-webhook-lifecycle")
}

// GetTaskExecutionLifecycleTopic topic
func (c *CommonConfig) GetTaskExecutionLifecycleTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"task-execution-lifecycle")
}

// GetWebsocketTaskletTopic topic
func (c *CommonConfig) GetWebsocketTaskletTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"websocket-tasklet")
}

// GetExpireArtifactsTaskletTopic topic
func (c *CommonConfig) GetExpireArtifactsTaskletTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"expire-artifacts-tasklet")
}

// GetForkJobTaskletTopic topic
func (c *CommonConfig) GetForkJobTaskletTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"fork-job-tasklet")
}

// GetWaitForkJobTaskletTopic topic
func (c *CommonConfig) GetWaitForkJobTaskletTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"wait-fork-job-tasklet")
}

// GetMessagingQueue topic
func (c *CommonConfig) GetMessagingQueue(q string) string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		q)
}

// GetMessagingTaskletTopic topic
func (c *CommonConfig) GetMessagingTaskletTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"messaging-tasklet")
}

// GetLogTopic topic
func (c *CommonConfig) GetLogTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"logs")
}

// GetHealthErrorTopic topic
func (c *CommonConfig) GetHealthErrorTopic() string {
	return NonPersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"health-error")
}

// GetRequestTopic request topic for incoming requests
func (c *CommonConfig) GetRequestTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"ant-request")
}

// GetReplyTopic reply topic for incoming requests
func (c *CommonConfig) GetReplyTopic() string {
	return PersistentTopic(
		c.MessagingProvider,
		c.Pulsar.TopicTenant,
		c.Pulsar.TopicNamespace,
		"ant-reply")
}

// Validate - validates
func (c *CommonConfig) Validate(_ []string) error {
	if c.MonitorInterval == 0 {
		c.MonitorInterval = 2 * time.Second
	}
	if c.ContainerReaperInterval == 0 {
		c.ContainerReaperInterval = 1 * time.Minute
	}
	if c.Redis.TTLMinutes == 0 {
		c.Redis.TTLMinutes = 5
	}
	if c.Redis.Port == 0 {
		c.Redis.Port = 6379
	}
	if c.RegistrationInterval == 0 {
		c.RegistrationInterval = 5 * time.Second
	}
	if c.DeadJobIDsEventsInterval == 0 {
		c.DeadJobIDsEventsInterval = 1 * time.Minute
	}

	if c.MaxStreamingLogMessageSize == 0 {
		c.MaxStreamingLogMessageSize = 1024 * 1024
	}

	// Note: Following config will limit the max runtime for a task with default value of about 1 hours
	if c.MaxTaskTimeout <= 0 {
		c.MaxTaskTimeout = 1 * time.Hour
	}
	// Note: Following config will limit the max runtime for a job with default value of about 2 hours
	if c.MaxJobTimeout <= 0 {
		c.MaxJobTimeout = 2 * time.Hour
	}
	if len(c.BlockUserAgents) == 0 {
		c.BlockUserAgents = []string{"Slackbot-LinkExpanding"}
	}
	c.blockUserAgentsMap = make(map[string]bool)
	for _, agent := range c.BlockUserAgents {
		c.blockUserAgentsMap[agent] = true
	}

	if c.RateLimitPerSecond <= 0 {
		c.RateLimitPerSecond = 1
	}

	if c.PublicDir == "" {
		c.PublicDir = "./public/"
	}
	if !strings.HasSuffix(c.PublicDir, "/") {
		c.PublicDir += "/"
	}

	if c.MessagingProvider == PulsarMessagingProvider {
		if err := c.Pulsar.Validate(); err != nil {
			return err
		}
	} else if c.MessagingProvider == KafkaMessagingProvider {
		if err := c.Kafka.Validate(); err != nil {
			return err
		}
	} else {
		if err := c.Redis.Validate(); err != nil {
			return err
		}
	}

	if err := c.S3.Validate(); err != nil {
		return err
	}

	if err := c.Auth.Validate(); err != nil {
		return err
	}

	if !listeningForStackTraceDumps {
		c.AddSignalHandlerForStackTrace()
	}

	return nil
}
