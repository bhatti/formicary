package config

import (
	"os"
	"plexobject.com/formicary/internal/crypto"
	"strconv"
	"time"
)

// TestServerConfig for testing
func TestServerConfig() *ServerConfig {
	serverCfg := &ServerConfig{}
	_ = serverCfg.Validate()
	serverCfg.Common.S3.AccessKeyID = "admin"
	serverCfg.Common.S3.SecretAccessKey = "password"
	serverCfg.Common.S3.Bucket = "bucket"
	serverCfg.Common.Queue.Endpoints = []string{"test"}
	serverCfg.Common.Redis.Host = "localhost"
	serverCfg.Common.ExternalBaseURL = "http://localhost:7070"
	serverCfg.SMTP.FromName = "Formicary Support"
	serverCfg.SMTP.FromEmail = "support@formicary.io"
	serverCfg.SMTP.Username = os.Getenv("SMTP_USERNAME")
	serverCfg.SMTP.Password = os.Getenv("SMTP_PASSWORD")
	serverCfg.SMTP.Host = os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	serverCfg.SMTP.Port, _ = strconv.Atoi(port)
	serverCfg.DB.MaxIdleConns = 10
	serverCfg.DB.MaxOpenConns = 20
	serverCfg.DB.MaxOpenConns = 20
	serverCfg.DB.EncryptionKey = string(crypto.SHA256Key("test-key"))
	serverCfg.DB.ConnMaxIdleTime = 1 * time.Hour
	serverCfg.DB.ConnMaxLifeTime = 4 * time.Hour
	serverCfg.Notify.EmailJobsTemplateFile = "../../public/views/notify/email_notify_job.html"
	serverCfg.Notify.SlackJobsTemplateFile = "../../public/views/notify/slack_notify_job.txt"
	serverCfg.Notify.VerifyEmailTemplateFile = "../../public/views/notify/verify_email.html"
	serverCfg.Notify.UserInvitationTemplateFile = "../../public/views/notify/user_invitation.html"

	serverCfg.Jobs.JobSchedulerLeaderInterval = 2 * time.Second
	serverCfg.Jobs.JobSchedulerCheckPendingJobsInterval = 2 * time.Second
	serverCfg.Jobs.OrphanRequestsTimeout = 5 * time.Second
	serverCfg.Jobs.OrphanRequestsUpdateInterval = 2 * time.Second
	serverCfg.Jobs.MissingCronJobsInterval = 2 * time.Second
	serverCfg.Jobs.AntRegistrationAliveTimeout = 2 * time.Second
	return serverCfg
}
