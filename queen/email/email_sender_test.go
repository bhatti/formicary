package email

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/queen/config"
	"strconv"
	"testing"
)

func Test_ShouldSendEmail(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	sender, err := New(serverCfg)
	require.NoError(t, err)
	err = sender.SendMessage(
		context.Background(),
		nil,
		nil,
		[]string{"support@formicary.io"},
		"my email",
		"Test email")
	require.NoError(t, err)
}

func newServerConfig() *config.ServerConfig {
	serverCfg := &config.ServerConfig{}
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Redis.Host = "localhost"
	serverCfg.S3.Bucket = "bucket"

	serverCfg.ExternalBaseURL = "http://localhost:7070"
	serverCfg.Email.FromName = "Formicary Support"
	serverCfg.Email.FromEmail = "support@formicary.io"
	serverCfg.Email.Username = os.Getenv("SMTP_USERNAME")
	serverCfg.Email.Password = os.Getenv("SMTP_PASSWORD")
	serverCfg.Email.Host = os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	serverCfg.Email.Port, _ = strconv.Atoi(port)
	serverCfg.Notify.EmailJobsTemplateFile = "../../public/views/notify/email_notify_job.html"
	serverCfg.Notify.SlackJobsTemplateFile = "../../public/views/notify/slack_notify_job.txt"
	serverCfg.Notify.VerifyEmailTemplateFile = "../../public/views/notify/verify_email.html"
	return serverCfg
}
