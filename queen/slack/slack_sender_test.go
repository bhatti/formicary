package slack

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"testing"
	"time"
)

func Test_ShouldSendSlackMessage(t *testing.T) {
	sender, err := New(testServerConfig())
	require.NoError(t, err)
	org := common.NewOrganization("owner", "unit", "bundle")
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		t.Logf("skip sending slack because token is not defined")
		return
	}
	_, _ = org.AddConfig(slackToken, token, true)
	err = sender.SendMessage(
		context.Background(),
		nil,
		org,
		[]string{"#formicary"},
		"my message",
		"Test message")
	require.NoError(t, err)
}

func testServerConfig() *config.ServerConfig {
	serverCfg := &config.ServerConfig{}
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Jobs.JobSchedulerLeaderInterval = 2 * time.Second
	serverCfg.Jobs.JobSchedulerCheckPendingJobsInterval = 2 * time.Second
	serverCfg.Jobs.OrphanRequestsTimeout = 5 * time.Second
	serverCfg.Jobs.OrphanRequestsUpdateInterval = 2 * time.Second
	serverCfg.Jobs.MissingCronJobsInterval = 2 * time.Second

	serverCfg.Notify.EmailJobsTemplateFile = "../../public/views/notify/email_notify_job.html"
	serverCfg.Notify.SlackJobsTemplateFile = "../../public/views/notify/slack_notify_job.txt"
	serverCfg.Notify.VerifyEmailTemplateFile = "../../public/views/notify/verify_email.html"
	return serverCfg
}
