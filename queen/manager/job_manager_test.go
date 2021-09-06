package manager

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

func Test_ShouldNotThrowErrorWhenSavingCronJobDefinitionAgain(t *testing.T) {
	var qc = common.NewQueryContext("user-id", "test-org", "")
	// GIVEN: a job definition with cron trigger is created
	job := newTestJobDefinition("test-job")
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
		common.NewQueryContext("", "", ""),
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

// Creating a test job
func newTestJobDefinition(name string) *types.JobDefinition {
	job := types.NewJobDefinition("io.formicary.test." + name)
	job.UserID = "user-id"
	job.OrganizationID = "test-org"
	_, _ = job.AddVariable("jk1", "jv1")
	for i := 1; i < 10; i++ {
		task := types.NewTaskDefinition(fmt.Sprintf("task%d", i), common.Shell)
		if i < 9 {
			task.OnExitCode["completed"] = fmt.Sprintf("task%d", i+1)
		}
		prefix := fmt.Sprintf("t%d", i)
		task.BeforeScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.AfterScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.Script = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		_, _ = task.AddVariable(prefix+"k1", "v1")
		task.Method = common.Docker
		if i%2 == 1 {
			task.AlwaysRun = true
		}
		job.AddTask(task)
	}
	job.UpdateRawYaml()

	return job
}
