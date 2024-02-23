package notify

import (
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

func (m *mockSender) SendMessage(
	_ *common.QueryContext,
	_ *common.User,
	_ []string,
	_ string,
	_ string,
	_ map[string]interface{}) error {
	if m.err == nil {
		m.sent++
	}

	return m.err
}

func (m *mockSender) SupportsLongReport() bool {
	return true
}

func (m *mockSender) JobNotifyTemplateFile() string {
	return "../../public/views/notify/email_notify_job.html"
}

func Test_ShouldNotifyGoodJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.SMTP.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext(nil, "")
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	sender := &mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		logRepository,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	jobExec := testNewJobExecution(job, req)
	err = notifier.NotifyJob(qc, user, job, req, jobExec, common.UNKNOWN)
	require.NoError(t, err)
}

func Test_ShouldNotifyFixedJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.SMTP.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext(nil, "")
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	sender := &mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		logRepository,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)
	user, job, req := newUserJobRequest("notify-job-good", common.COMPLETED)

	err = notifier.NotifyJob(qc, user, job, req, &types.JobExecution{}, common.FAILED)
	require.NoError(t, err)
	require.Equal(t, 1, sender.sent)
}

func Test_ShouldNotifyFailedJob(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.SMTP.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext(nil, "")
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	sender := &mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		logRepository,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	user, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(qc, user, job, req, &types.JobExecution{}, common.UNKNOWN)
	require.NoError(t, err)
	require.Equal(t, 1, sender.sent)
}

func Test_ShouldNotifyFailedJobWithoutUser(t *testing.T) {
	serverCfg := config.TestServerConfig()
	if err := serverCfg.SMTP.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	qc := common.NewQueryContext(nil, "")
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	sender := &mockSender{}
	require.NoError(t, err)
	notifier, err := New(
		serverCfg,
		logRepository,
		emailVerificationRepository)
	require.NoError(t, err)
	notifier.AddSender(common.EmailChannel, sender)

	_, job, req := newUserJobRequest("notify-job-failed", common.FAILED)

	err = notifier.NotifyJob(qc, nil, job, req, &types.JobExecution{}, common.UNKNOWN)
	require.NoError(t, err)
}

func newUserJobRequest(
	name string,
	state common.RequestState) (user *common.User, job *types.JobDefinition, request *types.JobRequest) {
	qc, _ := repository.NewTestQC()
	user = qc.User
	job = repository.NewTestJobDefinition(qc.User, name)
	request, _ = types.NewJobRequestFromDefinition(job)
	request.UserID = qc.GetUserID()
	request.ID = 1001
	request.JobState = state

	qc.User.Email = "support@formicary.io"
	qc.User.Name = "Bob"
	qc.User.Notify = map[common.NotifyChannel]common.JobNotifyConfig{
		common.EmailChannel: {Recipients: []string{"support@formicary.io", "blah@formicary.io"}},
	}
	return
}

func testNewJobExecution(job *types.JobDefinition, req *types.JobRequest) *types.JobExecution {
	jobExec := types.NewJobExecution(req.ToInfo())
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", "jv2")
	for i, t := range job.Tasks {
		jobExec.AddTasks(t)
		jobExec.Tasks[i].Stdout = []string{"test"}
		_, _ = jobExec.Tasks[i].AddContext("tk1", "v1")
		_, _ = jobExec.Tasks[i].AddContext("tk2", "v2")
	}
	_ = jobExec.AfterLoad()
	return jobExec
}
