// SPDX-License-Identifier: AGPL-3.0-or-later
//
// FSM-level integration tests that prove the full engine path:
//
//   SaveJobDefinition → GetJobDefinition (postProcessJob) →
//   JobExecutionStateMachine → BuildTaskRequest() →
//   TaskRequest.Variables contains job_variables →
//   resolveFanOutSource succeeds
//
// These tests replicate the EXACT path taken in production and catch the
// ReloadFromYaml bug where {{.region}}-style template vars caused postProcessJob
// to silently discard all job_variables when replacing the DB-loaded job.

package fsm

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/types"
)

// newFanOutJSM builds a full JobExecutionStateMachine for a fan-out job.
// It saves the job through the manager (so postProcessJob runs on reload),
// creates a real JobRequest+JobExecution, and wires up all reservations.
func newFanOutJSM(t *testing.T, jobYaml string) *JobExecutionStateMachine {
	t.Helper()
	cfg := config.TestServerConfig()

	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	require.NoError(t, err)
	if ch, ok := queueClient.(*queue.ClientChannel); ok {
		ch.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			_ = json.Unmarshal(inReq.Payload, &req)
			res := common.NewTaskResponse(&req)
			res.AntID = "test-ant"
			res.Host = "test"
			res.Status = common.COMPLETED
			b, _ := json.Marshal(res)
			return b, nil
		})
	}

	jobMgr, err := manager.TestJobManager(cfg)
	require.NoError(t, err)
	artifactMgr, err := manager.TestArtifactManager(cfg)
	require.NoError(t, err)
	userMgr, err := manager.TestUserManager(cfg)
	require.NoError(t, err)

	resourceManager := resource.NewStub()
	metricsRegistry := metrics.New()
	errorCodeRepo, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)

	user := common.NewUser("", ulid.Make().String()+"@formicary.io", "fanout-test", "", acl.NewRoles(""))
	user, err = userMgr.CreateUser(common.NewQueryContextFromIDs("", ""), user)
	require.NoError(t, err)
	qc := common.NewQueryContext(user, "")

	// Save through manager — postProcessJob runs on GetJobDefinition reload.
	job, err := types.NewJobDefinitionFromYaml([]byte(jobYaml))
	require.NoError(t, err)
	saved, err := jobMgr.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	// Load back through the full postProcessJob path — this is what the engine uses.
	loadedJob, err := jobMgr.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)

	req, err := types.NewJobRequestFromDefinition(loadedJob)
	require.NoError(t, err)
	jobRequestRepo, err := repository.NewTestJobRequestRepository()
	require.NoError(t, err)
	savedReq, err := jobRequestRepo.Save(qc, req)
	require.NoError(t, err)

	// Wire one reservation per task.
	reservations := make(map[string]*common.AntReservation)
	for _, task := range loadedJob.Tasks {
		resourceManager.Registry[task.TaskType+"-ant"] = &common.AntRegistration{
			AntID:       task.TaskType + "-ant",
			AntTopic:    task.TaskType + "-topic",
			MaxCapacity: 100,
			Tags:        []string{},
			Methods:     []common.TaskMethod{task.Method},
			Allocations: make(map[string]*common.AntAllocation),
		}
		reservations[task.TaskType] = &common.AntReservation{
			AntID:    task.TaskType + "-ant",
			AntTopic: task.TaskType + "-topic",
		}
	}

	jsm := NewJobExecutionStateMachine(
		cfg,
		queueClient,
		jobMgr,
		artifactMgr,
		userMgr,
		resourceManager,
		errorCodeRepo,
		metricsRegistry,
		savedReq.ToInfo(),
		reservations,
	)
	dbErr, launchErr := jsm.CreateJobExecution(context.Background())
	require.NoError(t, dbErr)
	require.NoError(t, launchErr)
	require.NoError(t, jsm.PrepareLaunch(jsm.JobExecution.ID))
	return jsm
}

// Test_FanOut_BuildTaskRequest_JobVariablesInVariables is the definitive test.
// Proves that after SaveJobDefinition → GetJobDefinition (postProcessJob) →
// BuildTaskRequest(), the TaskRequest.Variables map contains job_variables.
// This is the exact variable path that FanOutTasklet.Execute uses.
func Test_FanOut_BuildTaskRequest_JobVariablesInVariables(t *testing.T) {
	jobYaml := `job_type: io.formicary.test.fsm-fanout-buildreq
job_variables:
  regions: '["us-east-1","us-west-2","eu-west-1"]'
tasks:
  - task_type: setup
    method: SHELL
    script:
      - echo "setup"
    on_completed: deploy
  - task_type: deploy
    method: SHELL
    fan_out:
      source: regions
      item_var: region
      max_parallel: 2
      fail_fast: false
    script:
      - echo "Deploying to {{.region}}"
    on_completed: done
  - task_type: done
    method: SHELL
    script:
      - echo "done count={{.FanOutItemCount}}"
`
	jsm := newFanOutJSM(t, jobYaml)

	tsm, err := NewTaskExecutionStateMachine(jsm, "deploy")
	require.NoError(t, err)
	taskReq, err := tsm.BuildTaskRequest()
	require.NoError(t, err)

	// The critical assertion: regions must be here for FanOutTasklet.
	v, ok := taskReq.Variables["regions"]
	require.True(t, ok,
		"job_variables 'regions' must be in TaskRequest.Variables after full DB round-trip + BuildTaskRequest()")
	require.Equal(t, `["us-east-1","us-west-2","eu-west-1"]`, v.Value)

	require.NotNil(t, taskReq.ExecutorOpts.FanOut)
	require.Equal(t, "regions", taskReq.ExecutorOpts.FanOut.Source)
	require.Equal(t, "region", taskReq.ExecutorOpts.FanOut.ItemVar)
}

// Test_FanOut_BuildTaskRequest_MultipleJobVariables tests all job_variables survive.
func Test_FanOut_BuildTaskRequest_MultipleJobVariables(t *testing.T) {
	jobYaml := `job_type: io.formicary.test.fsm-fanout-multivars
job_variables:
  datasets: '["sales_2024","inventory_2024","orders_2024"]'
  batch_size: "500"
  env: "production"
tasks:
  - task_type: setup
    method: SHELL
    script:
      - echo "setup batch={{.batch_size}}"
    on_completed: process
  - task_type: process
    method: SHELL
    fan_out:
      source: datasets
      item_var: dataset
      max_parallel: 2
    script:
      - echo "Processing {{.dataset}} batch={{.batch_size}}"
    on_completed: done
  - task_type: done
    method: SHELL
    script:
      - echo "done env={{.env}}"
`
	jsm := newFanOutJSM(t, jobYaml)

	tsm, err := NewTaskExecutionStateMachine(jsm, "process")
	require.NoError(t, err)
	taskReq, err := tsm.BuildTaskRequest()
	require.NoError(t, err)

	_, ok := taskReq.Variables["datasets"]
	require.True(t, ok, "datasets must be in TaskRequest.Variables")
	_, ok = taskReq.Variables["batch_size"]
	require.True(t, ok, "batch_size must be in TaskRequest.Variables")
	_, ok = taskReq.Variables["env"]
	require.True(t, ok, "env must be in TaskRequest.Variables")
	require.NotNil(t, taskReq.ExecutorOpts.FanOut)
	require.Equal(t, "datasets", taskReq.ExecutorOpts.FanOut.Source)
}

// Test_FanOut_BuildTaskRequest_FanOutDeployFixture loads fixtures/fan_out_job.yaml
// and proves the full engine path works for the exact job that failed in production.
func Test_FanOut_BuildTaskRequest_FanOutDeployFixture(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)

	jsm := newFanOutJSM(t, string(b))

	tsm, err := NewTaskExecutionStateMachine(jsm, "deploy")
	require.NoError(t, err)
	taskReq, err := tsm.BuildTaskRequest()
	require.NoError(t, err)

	v, ok := taskReq.Variables["regions"]
	require.True(t, ok,
		"fixtures/fan_out_job.yaml: regions must be in TaskRequest.Variables after full engine path")
	require.NotNil(t, v.Value)
	require.NotNil(t, taskReq.ExecutorOpts.FanOut)
	require.Equal(t, "regions", taskReq.ExecutorOpts.FanOut.Source)
}

// Test_FanOut_BuildTaskRequest_FanOutTaskFixture loads fixtures/fan_out_task_job.yaml.
func Test_FanOut_BuildTaskRequest_FanOutTaskFixture(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)

	jsm := newFanOutJSM(t, string(b))

	tsm, err := NewTaskExecutionStateMachine(jsm, "process")
	require.NoError(t, err)
	taskReq, err := tsm.BuildTaskRequest()
	require.NoError(t, err)

	v, ok := taskReq.Variables["items"]
	require.True(t, ok,
		"fixtures/fan_out_task_job.yaml: items must be in TaskRequest.Variables")
	require.NotNil(t, v.Value)
	require.NotNil(t, taskReq.ExecutorOpts.FanOut)
	require.Equal(t, "items", taskReq.ExecutorOpts.FanOut.Source)
}

// Test_FanOut_BuildTaskRequest_FanOutForkJobFixture loads fixtures/fan_out_job_fork_job.yaml.
func Test_FanOut_BuildTaskRequest_FanOutForkJobFixture(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)

	jsm := newFanOutJSM(t, string(b))

	tsm, err := NewTaskExecutionStateMachine(jsm, "process")
	require.NoError(t, err)
	taskReq, err := tsm.BuildTaskRequest()
	require.NoError(t, err)

	v, ok := taskReq.Variables["datasets"]
	require.True(t, ok,
		"fixtures/fan_out_job_fork_job.yaml: datasets must be in TaskRequest.Variables")
	require.NotNil(t, v.Value)
	require.NotNil(t, taskReq.ExecutorOpts.FanOut)
	require.Equal(t, "datasets", taskReq.ExecutorOpts.FanOut.Source)
	require.True(t, taskReq.ExecutorOpts.FanOut.IsJobFanOut())
}
