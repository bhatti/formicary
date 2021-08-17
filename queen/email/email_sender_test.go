package email

import (
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/queen/config"
	"strconv"
	"testing"
)

func Test_ShouldSendEmail(t *testing.T) {
	cfg := &config.SMTPConfig{}
	cfg.FromEmail = "support@formicary.io"
	cfg.FromName = "Formicary Support"
	cfg.Username = os.Getenv("SMTP_USERNAME")
	cfg.Password = os.Getenv("SMTP_PASSWORD")
	cfg.Host = os.Getenv("SMTP_HOST")
	if cfg.Host == "" {
		cfg.Host = "smtp.gmail.com"
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	cfg.Port, _ = strconv.Atoi(port)
	cfg.JobsTemplateFile = "public/views/email/notify_job.html"
	if err := cfg.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	sender, err := New(cfg)
	require.NoError(t, err)
	err = sender.SendMessage([]string{"support@formicary.io"}, "my email", "Test email")
	require.NoError(t, err)
}
