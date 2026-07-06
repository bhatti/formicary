package fsm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	qtypes "plexobject.com/formicary/queen/types"
)

// NewTestTaskStateMachine test fixture
func NewTestTaskStateMachine() (*TaskExecutionStateMachine, error) {
	jsm, err := NewTestJobStateMachine()
	if err != nil {
		return nil, err
	}
	err = jsm.PrepareLaunch(jsm.JobExecution.ID)
	if err != nil {
		return nil, err
	}
	if len(jsm.JobDefinition.Tasks) != 9 {
		return nil, fmt.Errorf("expected 9 but found number of tasks %d", len(jsm.JobDefinition.Tasks))
	}
	return NewTaskExecutionStateMachine(
		jsm,
		jsm.JobDefinition.Tasks[0].TaskType,
	)
}

// NewTestJobStateMachineWithAntResponse creates a job state machine whose mock ant
// returns a response built by the provided callback. Use this to test non-happy-path
// flows (e.g. exit-code-3 → PAUSE_JOB) without altering the shared test fixture.
func NewTestJobStateMachineWithAntResponse(
	antResponse func(req *common.TaskRequest) *common.TaskResponse,
) (*JobExecutionStateMachine, error) {
	jsm, err := NewTestJobStateMachine()
	if err != nil {
		return nil, err
	}
	if channelClient, ok := jsm.QueueClient.(*queue.ClientChannel); ok {
		channelClient.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			if err := json.Unmarshal(inReq.Payload, &req); err != nil {
				return nil, err
			}
			return json.Marshal(antResponse(&req))
		})
	}
	return jsm, nil
}

// pauseJobYAML is a minimal two-task job. The first task maps exit code 3 to PAUSE_JOB
// so that the supervisor pauses the job (not fails it) when the ant exits with code 3.
// This is the canonical on_exit_code: PAUSE_JOB pattern used by poll-pr tasks.
var pauseJobYAML = `
job_type: io.formicary.test.pause-job
tasks:
- task_type: poll-task
  method: KUBERNETES
  script:
    - echo poll
  on_exit_code:
    3: PAUSE_JOB
  on_completed: done
- task_type: done
  method: KUBERNETES
  script:
    - echo done
`

// NewTestJobStateMachineForPause builds a job state machine from a YAML definition
// that has on_exit_code: {3: PAUSE_JOB} on the first task, and installs a mock ant
// that always returns exit code 3. This is the exact regression scenario for the
// PAUSED→FAILED bug.
func NewTestJobStateMachineForPause() (*JobExecutionStateMachine, error) {
	cfg := config.TestServerConfig()
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pulsar: %w", err)
	}
	jobManager, err := manager.TestJobManager(cfg)
	if err != nil {
		return nil, err
	}
	artifactManager, err := manager.TestArtifactManager(cfg)
	if err != nil {
		return nil, err
	}
	userManager, err := manager.TestUserManager(cfg)
	if err != nil {
		return nil, err
	}
	resourceManager := resource.NewStub()
	metricsRegistry := metrics.New()
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	if err != nil {
		return nil, err
	}

	// Mock ant: always return exit code 3 (triggers PAUSE_JOB for poll-task)
	if channelClient, ok := queueClient.(*queue.ClientChannel); ok {
		channelClient.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			if unmarshalErr := json.Unmarshal(inReq.Payload, &req); unmarshalErr != nil {
				return nil, unmarshalErr
			}
			res := common.NewTaskResponse(&req)
			res.AntID = "test-ant"
			res.Host = "test-host"
			res.Status = common.FAILED
			res.ExitCode = "3"
			return json.Marshal(res)
		})
	}

	resourceManager.Registry["ant-1"] = &common.AntRegistration{
		AntID:       "ant-1",
		AntTopic:    "ant-1-topic",
		MaxCapacity: 100,
		Tags:        make([]string, 0),
		Methods:     []common.TaskMethod{common.Kubernetes},
		Allocations: make(map[string]*common.AntAllocation),
	}

	// Create user and save job definition from YAML so on_exit_code is persisted
	user := common.NewUser("", ulid.Make().String()+"@formicary.io", "name", "", acl.NewRoles(""))
	user, err = userManager.CreateUser(common.NewQueryContextFromIDs("", ""), user)
	if err != nil {
		return nil, err
	}
	qc := common.NewQueryContext(user, "")

	jobDef, err := qtypes.NewJobDefinitionFromYaml([]byte(pauseJobYAML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pause job YAML: %w", err)
	}
	jobDef.UserID = user.ID
	jobDef.OrganizationID = user.OrganizationID
	jobDef, err = jobManager.SaveJobDefinition(qc, jobDef)
	if err != nil {
		return nil, fmt.Errorf("failed to save pause job definition: %w", err)
	}

	req, err := qtypes.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		return nil, err
	}
	req, err = jobManager.SaveJobRequest(qc, req)
	if err != nil {
		return nil, err
	}

	reservations := map[string]*common.AntReservation{
		"poll-task": {AntID: "test-ant", AntTopic: "ant-1-topic"},
		"done":      {AntID: "test-ant", AntTopic: "ant-1-topic"},
	}

	jsm := NewJobExecutionStateMachine(
		cfg,
		queueClient,
		jobManager,
		artifactManager,
		userManager,
		resourceManager,
		errorCodeRepository,
		metricsRegistry,
		req,
		reservations)
	if dbErr, createErr := jsm.CreateJobExecution(context.Background()); dbErr != nil {
		return nil, dbErr
	} else if createErr != nil {
		return nil, createErr
	}
	return jsm, nil
}

// NewTestJobStateMachine test fixture
// NewTestJobStateMachineWithOrgID creates a test state machine where the job
// request carries the given orgID, for testing org-scoped config loading.
func NewTestJobStateMachineWithOrgID(orgID string) (*JobExecutionStateMachine, error) {
	jsm, err := NewTestJobStateMachine()
	if err != nil {
		return nil, err
	}
	if req, ok := jsm.Request.(*qtypes.JobRequest); ok {
		req.OrganizationID = orgID
	}
	return jsm, nil
}

func NewTestJobStateMachine() (*JobExecutionStateMachine, error) {
	// Initializing dependent objects
	cfg := config.TestServerConfig()
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pulsar due to %w", err)
	}
	jobManager, err := manager.TestJobManager(cfg)
	if err != nil {
		return nil, err
	}
	artifactManager, err := manager.TestArtifactManager(cfg)
	if err != nil {
		return nil, err
	}
	userManager, err := manager.TestUserManager(cfg)
	if err != nil {
		return nil, err
	}
	resourceManager := resource.NewStub()
	metricsRegistry := metrics.New()
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	if err != nil {
		return nil, err
	}
	if channelClient, ok := queueClient.(*queue.ClientChannel); ok {
		channelClient.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			err = json.Unmarshal(inReq.Payload, &req)
			if err != nil {
				return nil, err
			}
			res := common.NewTaskResponse(&req)
			res.AntID = "test"
			res.Host = "test"
			res.Status = common.COMPLETED
			return json.Marshal(res)
		})
	}

	resourceManager.Registry["ant-1"] = &common.AntRegistration{
		AntID:       "ant-1",
		AntTopic:    "ant-1-topic",
		MaxCapacity: 100,
		Tags:        make([]string, 0),
		Methods:     []common.TaskMethod{common.Kubernetes},
		Allocations: make(map[string]*common.AntAllocation),
	}

	// Creating user
	user := common.NewUser("", ulid.Make().String()+"@formicary.io", "name", "", acl.NewRoles(""))
	user, err = userManager.CreateUser(common.NewQueryContextFromIDs("", ""), user)
	if err != nil {
		return nil, err
	}

	qc := common.NewQueryContext(user, "")
	jobRequest, jobExec, err := repository.NewTestJobExecution(qc, "my-test-job")
	if err != nil {
		return nil, err
	}

	jobRequest, err = jobManager.GetJobRequest(qc, jobRequest.ID)
	if err != nil {
		return nil, err
	}

	reservations := make(map[string]*common.AntReservation)
	for _, task := range jobExec.Tasks {
		reservations[task.TaskType] = &common.AntReservation{
			AntID:    "test-ant",
			AntTopic: "test-topic",
		}
	}

	jsm := NewJobExecutionStateMachine(
		cfg,
		queueClient,
		jobManager,
		artifactManager,
		userManager,
		resourceManager,
		errorCodeRepository,
		metricsRegistry,
		jobRequest,
		reservations)
	dbErr, err := jsm.CreateJobExecution(context.Background())
	if dbErr != nil {
		return nil, dbErr
	}
	if err != nil {
		return nil, err
	}

	return jsm, nil
}
