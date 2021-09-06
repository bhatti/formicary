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
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.S3.Bucket = "bucket"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Redis.Host = "localhost"
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
