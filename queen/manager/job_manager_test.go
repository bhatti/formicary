package manager

import (
	"github.com/oklog/ulid/v2"
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

// Test_ShouldCountByJobTypeAndStateStringsValidation verifies that CountByJobTypeAndStateStrings
// rejects unknown state strings rather than silently forwarding them to the database.
func Test_ShouldCountByJobTypeAndStateStringsValidation(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	// WHEN: a valid state is provided
	_, err = jobManager.CountByJobTypeAndStateStrings("any-job", "PENDING")
	// THEN: no validation error
	require.NoError(t, err)

	// WHEN: a lowercase state is provided (should be normalised to upper-case)
	_, err = jobManager.CountByJobTypeAndStateStrings("any-job", "pending")
	require.NoError(t, err)

	// WHEN: an unknown state string is provided
	_, err = jobManager.CountByJobTypeAndStateStrings("any-job", "PENDIN") // typo
	// THEN: an error is returned so skip_if misconfiguration is surfaced
	require.Error(t, err)
	require.Contains(t, err.Error(), "PENDIN")

	// WHEN: multiple states are provided and one is invalid
	_, err = jobManager.CountByJobTypeAndStateStrings("any-job", "PENDING", "BOGUS")
	require.Error(t, err)
	require.Contains(t, err.Error(), "BOGUS")
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

func cancelJobRequest(t *testing.T, qc *common.QueryContext, jobRequestRepo *repository.JobRequestRepositoryImpl, id string) {
	t.Helper()
	err := jobRequestRepo.Cancel(qc, id)
	require.NoError(t, err)
}

func Test_ShouldRestartHardDefaultsToLatest(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Save version 0
	jobName := "restart-hard-" + ulid.Make().String()
	job := repository.NewTestJobDefinition(qc.User, jobName)
	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	oldDefID := saved.ID

	// Create a request pinned to version 0 and set it to FAILED
	request, err := types.NewJobRequestFromDefinition(saved)
	require.NoError(t, err)
	request.UserKey = ulid.Make().String()
	savedReq, err := jobRequestRepo.Save(qc, request)
	require.NoError(t, err)
	cancelJobRequest(t, qc, jobRequestRepo, savedReq.ID)

	// Save version 1 (new definition, same type)
	job2 := repository.NewTestJobDefinition(qc.User, jobName)
	saved2, err := jobManager.SaveJobDefinition(qc, job2)
	require.NoError(t, err)
	require.NotEqual(t, oldDefID, saved2.ID)

	// WHEN: hard restart with no version specified
	err = jobManager.RestartJobRequest(qc, savedReq.ID, true, "")
	require.NoError(t, err)

	// THEN: request should now point to latest definition
	reloaded, err := jobRequestRepo.Get(qc, savedReq.ID)
	require.NoError(t, err)
	require.Equal(t, saved2.ID, reloaded.GetJobDefinitionID())
}

func Test_ShouldRestartSoftKeepsPinnedVersion(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Save version 0
	jobName := "restart-soft-" + ulid.Make().String()
	job := repository.NewTestJobDefinition(qc.User, jobName)
	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	originalDefID := saved.ID

	// Create a request pinned to version 0, set to FAILED
	request, err := types.NewJobRequestFromDefinition(saved)
	require.NoError(t, err)
	request.UserKey = ulid.Make().String()
	savedReq, err := jobRequestRepo.Save(qc, request)
	require.NoError(t, err)
	cancelJobRequest(t, qc, jobRequestRepo, savedReq.ID)

	// Save version 1
	job2 := repository.NewTestJobDefinition(qc.User, jobName)
	_, err = jobManager.SaveJobDefinition(qc, job2)
	require.NoError(t, err)

	// WHEN: soft restart with no version specified
	err = jobManager.RestartJobRequest(qc, savedReq.ID, false, "")
	require.NoError(t, err)

	// THEN: request should still point to original definition
	reloaded, err := jobRequestRepo.Get(qc, savedReq.ID)
	require.NoError(t, err)
	require.Equal(t, originalDefID, reloaded.GetJobDefinitionID())
}

func Test_ShouldRestartWithSpecificDefinitionID(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Save version 0
	jobName := "restart-id-" + ulid.Make().String()
	job := repository.NewTestJobDefinition(qc.User, jobName)
	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	v0ID := saved.ID

	// Save version 1
	job2 := repository.NewTestJobDefinition(qc.User, jobName)
	saved2, err := jobManager.SaveJobDefinition(qc, job2)
	require.NoError(t, err)

	// Create a request pinned to version 1, set to FAILED
	request, err := types.NewJobRequestFromDefinition(saved2)
	require.NoError(t, err)
	request.UserKey = ulid.Make().String()
	savedReq, err := jobRequestRepo.Save(qc, request)
	require.NoError(t, err)
	cancelJobRequest(t, qc, jobRequestRepo, savedReq.ID)

	// WHEN: restart with specific old definition ID
	err = jobManager.RestartJobRequest(qc, savedReq.ID, false, v0ID)
	require.NoError(t, err)

	// THEN: request should point to version 0
	reloaded, err := jobRequestRepo.Get(qc, savedReq.ID)
	require.NoError(t, err)
	require.Equal(t, v0ID, reloaded.GetJobDefinitionID())
}
