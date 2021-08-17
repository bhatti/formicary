package notify

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/email"
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
	sender, err := email.New(&serverCfg.Email)
	require.NoError(t, err)
	notifier, err := New(serverCfg, map[string]types.Sender{"email": sender})
	require.NoError(t, err)

	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	msg, err := notifier.NotifyJob(user, job, req)
	require.NoError(t, err)
	require.Contains(t, msg, "Job io.formicary.test.notify-job-good - 1001 Succeeded")
	require.Contains(t, msg, "Bob")
	require.NotContains(t, msg, "Error Code")
	require.NotContains(t, msg, "Error Message")
}

func Test_ShouldNotifyFailedJob(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	sender, err := email.New(&serverCfg.Email)
	require.NoError(t, err)
	notifier, err := New(serverCfg, map[string]types.Sender{"email": sender})
	require.NoError(t, err)

	user, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	msg, err := notifier.NotifyJob(user, job, req)
	require.NoError(t, err)
	require.Contains(t, msg, "Job io.formicary.test.notify-job-failed - 1001 Failed")
	require.Contains(t, msg, "Bob")
	require.Contains(t, msg, "Error Code")
	require.Contains(t, msg, "Error Message")
}

func Test_ShouldNotifyFailedJobWithoutUser(t *testing.T) {
	serverCfg := newServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	sender, err := email.New(&serverCfg.Email)
	require.NoError(t, err)
	notifier, err := New(serverCfg, map[string]types.Sender{"email": sender})
	require.NoError(t, err)

	_, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	msg, err := notifier.NotifyJob(nil, job, req)
	require.NoError(t, err)
	require.Contains(t, msg, "Job io.formicary.test.notify-job-failed - 1001 Failed")
	require.Contains(t, msg, "Error Code")
	require.Contains(t, msg, "Error Message")
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
	serverCfg.Email.JobsTemplateFile = "../../public/views/email/notify_job.html"
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
	user.Notify = map[string]common.JobNotifyConfig{
		"email": {Recipients: []string{"support@formicary.io"}},
	}
	return
}
