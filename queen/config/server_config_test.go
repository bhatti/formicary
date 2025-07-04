package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldLoadConfig(t *testing.T) {
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-queen")
	viper.SetConfigType("yaml")
	require.NoError(t, os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_ID", "my-client"))
	require.NoError(t, os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_SECRET", "my-secret"))
	require.NoError(t, os.Setenv("COMMON_S3_ACCESS_KEY_ID", "admin"))
	require.NoError(t, os.Setenv("COMMON_S3_SECRET_ACCESS_KEY", "password"))
	require.NoError(t, os.Setenv("COMMON_REDIS_HOST", "redis"))
	cfg, err := NewServerConfig("id")
	require.NoError(t, err)
	require.Equal(t, "id", cfg.Common.ID)
	require.Equal(t, "my-client", cfg.Common.Auth.GoogleClientID)
	require.Equal(t, "my-secret", cfg.Common.Auth.GoogleClientSecret)
	require.Equal(t, "admin", cfg.Common.S3.AccessKeyID)
	require.Equal(t, "password", cfg.Common.S3.SecretAccessKey)
	require.Equal(t, "redis", cfg.Common.Redis.Host)
}

func Test_ShouldValidateTopics(t *testing.T) {
	os.Setenv("COMMON_QUEUE_PROVIDER", string(types.RedisMessagingProvider))
	os.Setenv("COMMON_DEBUG", "true")
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-queen")
	viper.SetConfigType("yaml")
	c, err := NewServerConfig("id")
	require.NoError(t, err)
	c.Jobs.LaunchTopicSuffix = "-test-suffix"
	require.Equal(t, "formicary-topic-container-lifecycle", c.Common.GetContainerLifecycleTopic())
	require.Equal(t, "formicary-topic-job-definition-lifecycle", c.Common.GetJobDefinitionLifecycleTopic())
	require.Equal(t, "formicary-topic-job-request-lifecycle", c.Common.GetJobRequestLifecycleTopic())
	require.Equal(t, "formicary-topic-logs", c.Common.GetLogTopic())
	require.Equal(t, "formicary-topic-health-error", c.Common.GetHealthErrorTopic())
	require.Equal(t, "formicary-topic-ant-registration", c.Common.GetRegistrationTopic())
	require.Equal(t, "formicary-queue-job-execution-lifecycle", c.Common.GetJobExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-task-execution-lifecycle", c.Common.GetTaskExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-fork-job-tasklet", c.Common.GetForkJobTaskletTopic())
	require.Equal(t, "formicary-queue-wait-fork-job-tasklet", c.Common.GetWaitForkJobTaskletTopic())
	require.Equal(t, "formicary-topic-job-scheduler-leader", c.Common.GetJobSchedulerLeaderTopic())
	require.Equal(t, "formicary-queue-job-execution-launch-test-suffix", c.GetJobExecutionLaunchTopic())
	require.Equal(t, "formicary-queue-ant-request", c.Common.GetRequestTopic())
	require.Equal(t, "formicary-queue-ant-reply", c.Common.GetReplyTopic())
	require.Equal(t, "formicary-queue-task-ant-registration", c.GetResponseTopicAntRegistration())
	require.Equal(t, "formicary-queue-task-reply", c.GetResponseTopicTaskReply())
}
