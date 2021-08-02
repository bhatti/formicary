package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldCreateCommonConfig(t *testing.T) {
	// Given auth config
	c := CommonConfig {
		ID: "id",
		HTTPPort: 8080,
		ContainerReaperInterval: time.Second,
		MonitorInterval: time.Second,
		MonitoringURLs: make(map[string]string),
		RegistrationInterval: time.Second,
		MaxStreamingLogMessageSize: 10,
		MaxJobTimeout: time.Second,
		MaxTaskTimeout: time.Second,
		RateLimitPerSecond: 1,
	}
	c.AddSignalHandlerForStackTrace()
	c.AddSignalHandlerForShutdown(nil)
	require.Equal(t, "http://:8080", c.GetExternalBaseURL())
	c.Auth.Secure = true
	require.Equal(t, "https://:8080", c.GetExternalBaseURL())
	c.ExternalBaseURL = "https://external"
	require.Equal(t, "https://external", c.GetExternalBaseURL())
	require.Contains(t, c.GetSource(), "id@")
	require.Equal(t, "formicary-pubsub-ant-registration-topic", c.GetRegistrationTopic())
	require.Equal(t, "formicary-pubsub-container-lifecycle-topic", c.GetContainerLifecycleTopic())
	require.Equal(t, "formicary-pubsub-job-definition-lifecycle-topic", c.GetJobDefinitionLifecycleTopic())
	require.Equal(t, "formicary-pubsub-job-request-lifecycle-topic", c.GetJobRequestLifecycleTopic())
	require.Equal(t, "formicary-queue-job-execution-lifecycle-topic", c.GetJobExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-task-execution-lifecycle-topic", c.GetTaskExecutionLifecycleTopic())
	require.Equal(t, "formicary-queue-fork-job-tasklet-topic", c.GetForkJobTaskletTopic())
	require.Equal(t, "formicary-queue-wait-fork-job-tasklet-topic", c.GetWaitForkJobTaskletTopic())
	require.Equal(t, "formicary-pubsub-log-topic", c.GetLogTopic())
	require.Equal(t, "formicary-pubsub-health-error-topic", c.GetHealthErrorTopic())
	require.Equal(t, "formicary-queue-ant-request", c.GetRequestTopic())
	require.Error(t, c.Validate([]string{"a"}))
	require.Contains(t, c.Validate([]string{"a"}).Error(), "redis")
	c.Redis.Host = "localhost"
	require.Error(t, c.Validate([]string{"a"}))
	require.Contains(t, c.Validate([]string{"a"}).Error(), "s3 access-key")
	c.S3.AccessKeyID = "id"
	c.S3.SecretAccessKey = "id"
	c.S3.Bucket = "bucket"
	require.NoError(t, c.Validate([]string{"a"}))
}

