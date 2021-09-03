package notify

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/email"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"strconv"
	"testing"
)

func Test_ShouldNotifyGoodJob(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender, err := email.New(serverCfg)
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		map[common.NotifyChannel]types.Sender{common.EmailChannel: sender},
		emailVerificationRepository)
	require.NoError(t, err)

	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	err = notifier.NotifyJob(context.Background(), user, nil, job, req, common.UNKNOWN)
	require.NoError(t, err)
}

func Test_ShouldNotifyFixedJob(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender, err := email.New(serverCfg)
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		map[common.NotifyChannel]types.Sender{common.EmailChannel: sender},
		emailVerificationRepository)
	require.NoError(t, err)

	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	err = notifier.NotifyJob(context.Background(), user, nil, job, req, common.FAILED)
	require.NoError(t, err)
}

func Test_ShouldNotifyFailedJob(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender, err := email.New(serverCfg)
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		map[common.NotifyChannel]types.Sender{common.EmailChannel: sender},
		emailVerificationRepository)
	require.NoError(t, err)

	user, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(context.Background(), user, nil, job, req, common.UNKNOWN)
	require.NoError(t, err)
}

func Test_ShouldNotifyFailedJobWithoutUser(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender, err := email.New(serverCfg)
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		map[common.NotifyChannel]types.Sender{common.EmailChannel: sender},
		emailVerificationRepository)
	require.NoError(t, err)

	_, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(context.Background(), nil, nil, job, req, common.UNKNOWN)
	require.NoError(t, err)
}

// Creating a test job
func newTestJobDefinition(name string) *types.JobDefinition {
	job := types.NewJobDefinition("io.formicary.test." + name)
	job.UserID = "test-user"
	job.OrganizationID = "test-org"
	_, _ = job.AddVariable("jk1", "jv1")
	for i := 1; i < 3; i++ {
		task := types.NewTaskDefinition(fmt.Sprintf("task%d", i), common.Shell)
		if i < 2 {
			task.OnExitCode["completed"] = fmt.Sprintf("task%d", i+1)
		}
		prefix := fmt.Sprintf("t%d", i)
		task.Script = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		_, _ = task.AddVariable(prefix+"k1", "v1")
		job.AddTask(task)
	}
	job.UpdateRawYaml()

	return job
}

func newServerConfig() *config.ServerConfig {
	serverCfg := &config.ServerConfig{}
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Redis.Host = "localhost"
	serverCfg.ExternalBaseURL = "http://localhost:7070"
	serverCfg.Email.FromName = "Formicary Support"
	serverCfg.Email.FromEmail = "support@formicary.io"
	serverCfg.Email.Username = os.Getenv("SMTP_USERNAME")
	serverCfg.Email.Password = os.Getenv("SMTP_PASSWORD")
	serverCfg.Email.Host = os.Getenv("SMTP_HOST")
	if serverCfg.Email.Host == "" {
		serverCfg.Email.Host = "smtp.gmail.com"
	}
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

func newUserJobRequest(name string, state common.RequestState) (user *common.User, job *types.JobDefinition, request *types.JobRequest) {
	job = newTestJobDefinition(name)
	request, _ = types.NewJobRequestFromDefinition(job)
	request.UserID = "uid"
	request.ID = 1001
	request.JobState = state

	user = common.NewUser("gid", "username", "my name", false)
	user.ID = "uid"
	user.Name = "Bob"
	user.Email = "support@formicary.io"
	user.Notify = map[common.NotifyChannel]common.JobNotifyConfig{
		common.EmailChannel: {Recipients: []string{"support@formicary.io", "blah@mail.cc"}},
	}
	return
}
