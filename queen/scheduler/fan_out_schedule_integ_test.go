// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Integration test: verify that a job with a fan_out task (ai-parallel-test pattern)
// schedules successfully with a simulated local Kubernetes ant worker.
//
// The test:
//  1. Defines a minimal ai-parallel-test-style job (analyze → run-tests fan_out → done).
//  2. Registers a local simulated KUBERNETES ant with the real resource manager.
//  3. Saves the job definition and a PENDING request through the job manager.
//  4. Calls scheduleJob(), which exercises the full scheduling path:
//     CheckAntResourcesAndConcurrencyForJob → doReserveJobResources → PrepareLaunch → CreateJobExecution.
//  5. Asserts the request transitions to READY (not ERR_NO_ANT_FOR_METHOD or ERR_ANT_RESOURCES).
package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	qtypes "plexobject.com/formicary/queen/types"
)

// fanOutTestContext holds shared infrastructure for fan-out integration tests.
type fanOutTestContext struct {
	ctx        context.Context
	js         *JobScheduler
	jobManager *manager.JobManager
	qc         *common.QueryContext
}

// newFanOutTestContext builds the real resource manager + job scheduler used by both
// fan-out integration tests.  A single KUBERNETES+SHELL ant is pre-registered.
func newFanOutTestContext(t *testing.T) *fanOutTestContext {
	t.Helper()
	serverCfg := config.TestServerConfig()
	ctx := context.Background()

	errorRepo, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	queueClient, err := queue.NewClientManager().GetClient(ctx, &serverCfg.Common)
	require.NoError(t, err)
	healthMonitor, err := health.New(&serverCfg.Common, queueClient)
	require.NoError(t, err)
	jobManager, err := manager.TestJobManager(serverCfg)
	require.NoError(t, err)
	artifactManager, err := manager.TestArtifactManager(serverCfg)
	require.NoError(t, err)
	userManager, err := manager.TestUserManager(serverCfg)
	require.NoError(t, err)

	// Real resource manager so the full scheduling path runs (not the stub).
	rm := resource.New(serverCfg, queueClient)
	localAnt := &common.AntRegistration{
		AntID:       "local-k8s-ant-" + ulid.Make().String(),
		AntTopic:    "local-k8s-ant-topic",
		MaxCapacity: 10,
		Methods:     []common.TaskMethod{common.Kubernetes, common.Shell},
		Tags:        make([]string, 0),
		Allocations: make(map[string]*common.AntAllocation),
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, rm.Register(ctx, localAnt))

	js := New(serverCfg, queueClient, jobManager, artifactManager, userManager, rm,
		errorRepo, healthMonitor, metrics.New(), nil, nil, nil)

	user := common.NewUser("", ulid.Make().String()+"@formicary.io", "test", "", acl.NewRoles(""))
	user, err = userManager.CreateUser(common.NewQueryContextFromIDs("", ""), user)
	require.NoError(t, err)

	return &fanOutTestContext{
		ctx:        ctx,
		js:         js,
		jobManager: jobManager,
		qc:         common.NewQueryContext(user, ""),
	}
}

// fanOutJobYAML mirrors the structure of docs/examples/ai-parallel-test.yaml:
//   analyze (KUBERNETES) → run-tests (KUBERNETES + fan_out) → done (SHELL)
//
// The fan_out on run-tests causes the framework to rewrite it as a FAN_OUT_JOB
// internal method.  Before the fix, scheduling this job failed with:
//
//	"no ants available for method 'FAN_OUT_JOB', ants-by-methods=N, total-registered-ants=N"
//
// because FanOutTasklet registers asynchronously and may not be present yet.
var fanOutJobYAML = `
job_type: ai-parallel-test-integ
description: "Integration test for fan-out scheduling (mirrors ai-parallel-test.yaml)"
max_concurrency: 10
timeout: 3600s

tasks:
- task_type: analyze
  method: KUBERNETES
  script:
    - echo "discovering test shards"
  on_completed: run-tests
  on_failed: done

- task_type: run-tests
  method: KUBERNETES
  fan_out:
    source: TestShards
    item_var: shard
    max_parallel: "4"
    fail_fast: false
  script:
    - echo "running shard {{.shard}}"
  on_completed: done
  on_failed: done

- task_type: done
  method: SHELL
  script:
    - echo "pipeline complete"
`

// Test_ShouldScheduleJobWithFanOutUsingLocalAnt verifies end-to-end scheduling of a
// job that contains a fan_out task.  A local KUBERNETES ant is pre-registered so the
// full scheduling path (CheckAntResourcesAndConcurrencyForJob → doReserveJobResources →
// PrepareLaunch → CreateJobExecution) can run without the FAN_OUT_JOB method leaking
// into the ant reservation checks.
//
// Success criterion: scheduleJob() returns nil and the job request transitions to READY.
func Test_ShouldScheduleJobWithFanOutUsingLocalAnt(t *testing.T) {
	tc := newFanOutTestContext(t)

	// Parse and save the job definition.
	jobDef, err := qtypes.NewJobDefinitionFromYaml([]byte(fanOutJobYAML))
	require.NoError(t, err)
	jobDef.UserID = tc.qc.User.ID
	jobDef.OrganizationID = tc.qc.User.OrganizationID
	jobDef, err = tc.jobManager.SaveJobDefinition(tc.qc, jobDef)
	require.NoError(t, err)

	req, err := qtypes.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	req, err = tc.jobManager.SaveJobRequest(tc.qc, req)
	require.NoError(t, err)
	require.Equal(t, common.PENDING, req.JobState, "new request must start in PENDING state")

	// WHEN: the scheduler attempts to schedule the job.
	schedErr := tc.js.scheduleJob(tc.ctx, req.ToInfo())

	// THEN: scheduling succeeds — no "no ants available for FAN_OUT_JOB" error.
	require.NoError(t, schedErr,
		"scheduleJob must not fail for a fan_out job when a Kubernetes ant is registered; "+
			"error indicates the internal FAN_OUT_JOB method leaked into the reservation path")

	updated, err := tc.jobManager.GetJobRequest(tc.qc, req.ID)
	require.NoError(t, err)
	require.Equal(t, common.READY, updated.JobState,
		"job request must reach READY state; got %s (ErrorCode=%q)", updated.JobState, updated.ErrorCode)
	require.Empty(t, updated.ErrorCode, "scheduled job must have no error code; got %q", updated.ErrorCode)
}

// Test_ShouldScheduleExternalOnlyJobWithLocalAnt is a sanity-check baseline:
// a job with only KUBERNETES tasks (no fan_out) must also schedule cleanly.
// Guards against regressions where IsInternal() skip path breaks external-only jobs.
func Test_ShouldScheduleExternalOnlyJobWithLocalAnt(t *testing.T) {
	tc := newFanOutTestContext(t)

	externalOnlyYAML := `
job_type: ai-external-only-integ
tasks:
- task_type: build
  method: KUBERNETES
  script:
    - echo build
  on_completed: test
- task_type: test
  method: KUBERNETES
  script:
    - echo test
`
	jobDef, err := qtypes.NewJobDefinitionFromYaml([]byte(externalOnlyYAML))
	require.NoError(t, err)
	jobDef.UserID = tc.qc.User.ID
	jobDef.OrganizationID = tc.qc.User.OrganizationID
	jobDef, err = tc.jobManager.SaveJobDefinition(tc.qc, jobDef)
	require.NoError(t, err)

	req, err := qtypes.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	req, err = tc.jobManager.SaveJobRequest(tc.qc, req)
	require.NoError(t, err)

	require.NoError(t, tc.js.scheduleJob(tc.ctx, req.ToInfo()),
		"external-only job must schedule cleanly with a registered Kubernetes ant")

	updated, err := tc.jobManager.GetJobRequest(tc.qc, req.ID)
	require.NoError(t, err)
	require.Equal(t, common.READY, updated.JobState)
}
