package fsm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
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

// NewTestJobStateMachine test fixture
func NewTestJobStateMachine() (*JobExecutionStateMachine, error) {
	// Initializing dependent objects
	cfg := config.TestServerConfig()
	queueClient := queue.NewStubClient(&cfg.CommonConfig)
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
	queueClient.SendReceivePayloadFunc = func(
		_ queue.MessageHeaders,
		payload []byte) ([]byte, error) {
		var req common.TaskRequest
		err = json.Unmarshal(payload, &req)
		if err != nil {
			return nil, err
		}
		res := common.NewTaskResponse(&req)
		res.Status = common.COMPLETED
		return json.Marshal(res)
	}

	resourceManager.Registry["ant-1"] = &common.AntRegistration{
		AntID:       "ant-1",
		AntTopic:    "ant-1-topic",
		MaxCapacity: 100,
		Tags:        make([]string, 0),
		Methods:     []common.TaskMethod{common.Kubernetes},
		Allocations: make(map[uint64]*common.AntAllocation),
	}

	// Creating user
	user := common.NewUser("", uuid.NewV4().String()+"@formicary.io", "name", "", acl.NewRoles(""))
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
		reservations[task.TaskType] = &common.AntReservation{}
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
