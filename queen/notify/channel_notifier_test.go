package notify

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

type mockSender struct {
	err  error
	sent int
}

func (m mockSender) SendMessage(
	_ *common.QueryContext,
	_ *common.User,
	_ *common.Organization,
	_ []string,
	_ string,
	_ string,
	_ map[string]interface{}) error {
	if m.err == nil {
		m.sent++
	}

	return m.err
}

func (m mockSender) JobNotifyTemplateFile() string {
	return ""
}

func Test_ShouldNotifyGoodJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext("", "", "").WithAdmin()
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender := mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	err = notifier.NotifyJob(qc, user, nil, job, req, common.UNKNOWN)
	require.NoError(t, err)
}

func Test_ShouldNotifyFixedJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext("", "", "").WithAdmin()
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender := mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)
	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	err = notifier.NotifyJob(qc, user, nil, job, req, common.FAILED)
	require.NoError(t, err)
	require.Equal(t, 1, sender.sent)
}

func Test_ShouldNotifyFailedJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext("", "", "").WithAdmin()
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender := mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	user, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(qc, user, nil, job, req, common.UNKNOWN)
	require.NoError(t, err)
	require.Equal(t, 1, sender.sent)
}

func Test_ShouldNotifyFailedJobWithoutUser(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext("", "", "").WithAdmin()
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender := mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	_, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(qc, nil, nil, job, req, common.UNKNOWN)
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

func newUserJobRequest(name string, state common.RequestState) (user *common.User, job *types.JobDefinition, request *types.JobRequest) {
	job = newTestJobDefinition(name)
	request, _ = types.NewJobRequestFromDefinition(job)
	request.UserID = "uid"
	request.ID = 1001
	request.JobState = state

	user = common.NewUser("gid", "username", "my name", "support@formicary.io", false)
	user.ID = "uid"
	user.Name = "Bob"
	user.Notify = map[common.NotifyChannel]common.JobNotifyConfig{
		common.EmailChannel: {Recipients: []string{"support@formicary.io", "blah@formicary.io"}},
	}
	return
}
