package manager

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

func Test_ShouldSaveJobDefinition(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	qc, err := repository.NewTestQC()
	b, err := os.ReadFile("../../docs/examples/io.formicary.tokens.yaml")
	require.NoError(t, err)
	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	// WHEN: the job definition is saved, which will automatically create job-request
	_, err = jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	// THEN: loading should not fail
	loaded, err := jobManager.GetJobDefinition(qc, job.ID)
	require.NoError(t, err)
	task := loaded.GetTask("analyze")
	require.NoError(t, err)
	require.True(t, task.ReportStdout)
	require.NotNil(t, loaded.ReportStdoutTask())
	task, _, err = loaded.GetDynamicTask("analyze", nil)
	require.NoError(t, err)
	require.True(t, task.ReportStdout)
	require.NotNil(t, loaded.ReportStdoutTask())
}

func Test_ShouldNotThrowErrorWhenSavingCronJobDefinitionAgain(t *testing.T) {
	// GIVEN: a job definition with cron trigger is created
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	job := repository.NewTestJobDefinition(qc.User, "test-job")
	job.CronTrigger = "0 0 * * * * *"
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepository, err := newTestJobManager(serverCfg)
	require.NoError(t, err)
	_, userKey := job.GetCronScheduleTimeAndUserKey()
	// AND: no other request exists
	jobRequestRepository.Clear() // deleting all requests

	// WHEN: the job definition is saved, which will automatically create job-request
	_, err = jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	// Verifying job-request created by above job-definition
	verifyAutomaticallyCreatedJobRequest(t, jobRequestRepository, job, userKey)

	// AND: the job definition is saved again, which will automatically create job-request
	_, err = jobManager.SaveJobDefinition(qc, job) // saving again
	// THEN: no error should occur
	require.NoError(t, err)
}

func verifyAutomaticallyCreatedJobRequest(
	t *testing.T,
	jobRequestRepository *repository.JobRequestRepositoryImpl,
	job *types.JobDefinition, expectedUserKey string) {
	all, total, err := jobRequestRepository.Query(
		common.NewQueryContext(nil, ""),
		map[string]interface{}{"job_type": job.JobType},
		0,
		10,
		make([]string, 0))
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if total != 1 {
		t.Fatalf("failed to find automatically created request %d", total)
	}
	if all[0].UserKey != expectedUserKey {
		t.Fatalf("failed to match user key %s, expecting %s", all[total].UserKey, expectedUserKey)
	}
}

func newTestJobManager(
	serverCfg *config.ServerConfig,
) (*JobManager, *repository.JobRequestRepositoryImpl, error) {
	mgr, err := TestJobManager(serverCfg)
	if err != nil {
		return nil, nil, err
	}

	jobRequestRepository, err := repository.NewTestJobRequestRepository()
	if err != nil {
		return nil, nil, err
	}
	return mgr, jobRequestRepository, err
}
