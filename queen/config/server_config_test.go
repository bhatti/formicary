package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func Test_ShouldLoadConfig(t *testing.T) {
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-queen")
	viper.SetConfigType("yaml")
	os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_ID", "my-client")
	os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_SECRET", "my-secret")
	cfg, err := NewServerConfig("id")
	require.NoError(t, err)
	require.Equal(t, "id", cfg.ID)
	require.Equal(t, "my-client", cfg.Auth.GoogleClientID)
	require.Equal(t, "my-secret", cfg.Auth.GoogleClientSecret)
}

func Test_ShouldValidateTopics(t *testing.T) {
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-queen")
	viper.SetConfigType("yaml")
	c, err := NewServerConfig("id")
	require.NoError(t, err)
	c.Jobs.LaunchTopicSuffix = "-test-suffix"
	require.Equal(t, "formicary-topic-container-lifecycle", c.GetContainerLifecycleTopic())
	require.Equal(t, "formicary-topic-job-definition-lifecycle", c.GetJobDefinitionLifecycleTopic())
	require.Equal(t, "formicary-topic-job-request-lifecycle", c.GetJobRequestLifecycleTopic())
	require.Equal(t, "formicary-topic-logs", c.GetLogTopic())
	require.Equal(t, "formicary-topic-health-error", c.GetHealthErrorTopic())
	require.Equal(t, "formicary-topic-ant-registration", c.GetRegistrationTopic())
	require.Equal(t, "formicary-queue-job-execution-lifecycle", c.GetJobExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-task-execution-lifecycle", c.GetTaskExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-fork-job-tasklet", c.GetForkJobTaskletTopic())
	require.Equal(t, "formicary-queue-wait-fork-job-tasklet", c.GetWaitForkJobTaskletTopic())
	require.Equal(t, "formicary-topic-job-scheduler-leader", c.GetJobSchedulerLeaderTopic())
	require.Equal(t, "formicary-queue-job-execution-launch-test-suffix", c.GetJobExecutionLaunchTopic())
	require.Equal(t, "formicary-queue-ant-request", c.GetRequestTopic())
	require.Equal(t, "formicary-queue-ant-reply", c.GetReplyTopic())
	require.Equal(t, "formicary-queue-task-ant-registration", c.GetResponseTopicAntRegistration())
	require.Equal(t, "formicary-queue-task-reply", c.GetResponseTopicTaskReply())
}
