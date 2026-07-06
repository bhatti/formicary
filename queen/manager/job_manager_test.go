package manager

import (
	"context"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
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

// Cancelling a parent must cascade to its active child (cascade_cancel=true).
func Test_ShouldCancelJobCascadesToActiveChild(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Save a job definition to anchor both parent and child.
	jobDef := repository.NewTestJobDefinition(qc.User, "cascade-parent-"+ulid.Make().String())
	savedDef, err := jobManager.SaveJobDefinition(qc, jobDef)
	require.NoError(t, err)

	// Create parent request.
	parentReq, err := types.NewJobRequestFromDefinition(savedDef)
	require.NoError(t, err)
	parentReq.UserKey = ulid.Make().String()
	savedParent, err := jobRequestRepo.Save(qc, parentReq)
	require.NoError(t, err)

	// Create child request with CascadeCancel=true and ParentID set.
	childDef := repository.NewTestJobDefinition(qc.User, "cascade-child-"+ulid.Make().String())
	savedChildDef, err := jobManager.SaveJobDefinition(qc, childDef)
	require.NoError(t, err)
	childReq, err := types.NewJobRequestFromDefinition(savedChildDef)
	require.NoError(t, err)
	childReq.UserKey = ulid.Make().String()
	childReq.ParentID = savedParent.ID
	childReq.CascadeCancel = true
	savedChild, err := jobRequestRepo.Save(qc, childReq)
	require.NoError(t, err)
	require.Equal(t, common.PENDING, savedChild.JobState)

	// WHEN: parent is cancelled
	err = jobManager.CancelJobRequest(qc, savedParent.ID)
	require.NoError(t, err)

	// THEN: child must also be cancelled (BFS cascade runs synchronously in goroutine;
	// poll briefly until the state propagates).
	var childState common.RequestState
	for i := 0; i < 20; i++ {
		updated, getErr := jobRequestRepo.Get(qc, savedChild.ID)
		require.NoError(t, getErr)
		childState = updated.JobState
		if childState == common.CANCELLED {
			break
		}
		// tiny yield — the goroutine needs to run
		require.Eventually(t, func() bool {
			updated2, _ := jobRequestRepo.Get(qc, savedChild.ID)
			return updated2 != nil && updated2.JobState == common.CANCELLED
		}, 2e9, 50e6, "child job must be cascade-cancelled within 2s")
		break
	}
}

// Child without cascade_cancel=true must NOT be cancelled when parent is cancelled.
func Test_ShouldCancelJobSkipsChildWithoutCascadeFlag(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobDef := repository.NewTestJobDefinition(qc.User, "no-cascade-parent-"+ulid.Make().String())
	savedDef, err := jobManager.SaveJobDefinition(qc, jobDef)
	require.NoError(t, err)

	parentReq, err := types.NewJobRequestFromDefinition(savedDef)
	require.NoError(t, err)
	parentReq.UserKey = ulid.Make().String()
	savedParent, err := jobRequestRepo.Save(qc, parentReq)
	require.NoError(t, err)

	childDef := repository.NewTestJobDefinition(qc.User, "no-cascade-child-"+ulid.Make().String())
	savedChildDef, err := jobManager.SaveJobDefinition(qc, childDef)
	require.NoError(t, err)
	childReq, err := types.NewJobRequestFromDefinition(savedChildDef)
	require.NoError(t, err)
	childReq.UserKey = ulid.Make().String()
	childReq.ParentID = savedParent.ID
	childReq.CascadeCancel = false // explicitly no cascade
	savedChild, err := jobRequestRepo.Save(qc, childReq)
	require.NoError(t, err)

	// WHEN: parent is cancelled
	err = jobManager.CancelJobRequest(qc, savedParent.ID)
	require.NoError(t, err)

	// THEN: child should remain PENDING (not cascaded)
	updated, err := jobRequestRepo.Get(qc, savedChild.ID)
	require.NoError(t, err)
	require.Equal(t, common.PENDING, updated.JobState, "child without cascade_cancel=true must not be cancelled")
}

// CancelJobRequest on a parent with no children must not error.
func Test_ShouldCancelJobNoErrorWhenNoChildren(t *testing.T) {
	serverCfg := config.TestServerConfig()
	jobManager, jobRequestRepo, err := newTestJobManager(serverCfg)
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobDef := repository.NewTestJobDefinition(qc.User, "no-children-"+ulid.Make().String())
	savedDef, err := jobManager.SaveJobDefinition(qc, jobDef)
	require.NoError(t, err)

	req, err := types.NewJobRequestFromDefinition(savedDef)
	require.NoError(t, err)
	req.UserKey = ulid.Make().String()
	saved, err := jobRequestRepo.Save(qc, req)
	require.NoError(t, err)

	// WHEN: parent with no forked children is cancelled
	err = jobManager.CancelJobRequest(qc, saved.ID)

	// THEN: no error
	require.NoError(t, err)
}

// Test_ShouldSignalSchedulerOnTrigger verifies that TriggerJobRequest sends exactly one
// signal on the schedulerTriggerCh so the scheduler can wake up immediately.
func Test_ShouldSignalSchedulerOnTrigger(t *testing.T) {
	serverCfg := config.TestServerConfig()
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Build all repos manually so we can share the same jobRequestRepository
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobDefRepo, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	jobReqRepo, err := repository.NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecRepo, err := repository.NewTestJobExecutionRepository()
	require.NoError(t, err)
	emailVerifRepo, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	logRepo, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	artifactMgr, err := TestArtifactManager(serverCfg)
	require.NoError(t, err)
	notifier, err := notify.New(serverCfg, logRepo, emailVerifRepo)
	require.NoError(t, err)
	userMgr, err := TestUserManager(serverCfg)
	require.NoError(t, err)
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverCfg.Common)
	require.NoError(t, err)

	triggerCh := make(chan struct{}, 1)
	jobManager, err := NewJobManager(
		context.Background(),
		serverCfg,
		auditRepo,
		jobDefRepo,
		jobReqRepo,
		jobExecRepo,
		userMgr,
		resource.New(serverCfg, queueClient),
		artifactMgr,
		stats.NewJobStatsRegistry(),
		metrics.New(),
		queueClient,
		notifier,
		nil,
		triggerCh,
	)
	require.NoError(t, err)

	// Create a cron-triggered PENDING job request that Trigger() can act on.
	jobDef := repository.NewTestJobDefinition(qc.User, "trigger-signal-"+ulid.Make().String())
	jobDef.CronTrigger = "0 0 * * * * *"
	savedDef, err := jobManager.SaveJobDefinition(qc, jobDef)
	require.NoError(t, err)

	// SaveJobDefinition for a cron job creates a PENDING cron-triggered request.
	reqs, _, err := jobReqRepo.Query(qc, map[string]interface{}{
		"job_type":       savedDef.JobType,
		"job_state":      string(common.PENDING),
		"cron_triggered": true,
	}, 0, 1, nil)
	require.NoError(t, err)
	require.NotEmpty(t, reqs, "expected a cron-triggered PENDING request to exist")
	reqID := reqs[0].ID

	// Drain the channel in case SaveJobDefinition already sent a signal.
	select {
	case <-triggerCh:
	default:
	}

	// WHEN: TriggerJobRequest is called
	err = jobManager.TriggerJobRequest(qc, reqID)
	require.NoError(t, err)

	// THEN: exactly one signal must be present on the channel (non-blocking check).
	select {
	case <-triggerCh:
		// pass
	default:
		t.Fatal("expected schedulerTriggerCh to receive a signal after TriggerJobRequest")
	}
}
