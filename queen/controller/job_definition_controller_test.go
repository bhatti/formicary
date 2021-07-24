package controller

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
)

func newTestJobManager(serverCfg *config.ServerConfig, t *testing.T) *manager.JobManager {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	queueClient, _ := queue.NewStubClient(&serverCfg.CommonConfig)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	jobRequestRepository, err := repository.NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecutionRepository, err := repository.NewTestJobExecutionRepository()
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	artifactService, err := artifacts.NewStub(nil)
	require.NoError(t, err)
	artifactManager, err := manager.NewArtifactManager(
		serverCfg,
		artifactRepository,
		artifactService)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	metricsRegistry := metrics.New()

	resourceManager := resource.New(serverCfg, queueClient)
	jobManager, err := manager.NewJobManager(
		serverCfg,
		auditRecordRepository,
		jobDefinitionRepository,
		jobRequestRepository,
		jobExecutionRepository,
		userRepository,
		orgRepository,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		metricsRegistry,
		queueClient,
	)
	if err != nil {
		t.Fatalf("failed to create job manager %v", err)
	}
	job, err := jobManager.SaveJobDefinition(qc, newTestJobDefinition("my-job"))
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	req, err := types.NewJobRequestFromDefinition(job)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	_, err = jobManager.SaveJobRequest(common.NewQueryContext(req.UserID, req.OrganizationID, ""), req)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	return jobManager
}

func Test_ShouldQueryJobDefinitions(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	_ = jobDefinitionQueryResponseBody{}
	_ = jobDefinitionQueryParams{}
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err := ctrl.queryJobDefinitions(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	jobs := ctx.Result.(*PaginatedResult).Records.([]*types.JobDefinition)
	if len(jobs) == 0 {
		t.Fatalf("no jobDefinitions found")
	}
}

func Test_ShouldGetJobDefinitionsYAML(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionTypeParams{}
	_ = jobDefinitionBody{}
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["type"] = "my-job"
	err := ctrl.getYamlJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	yaml := ctx.Result.(string)
	if yaml == "" {
		t.Fatalf("no jobDefinition found")
	}
}

func Test_ShouldStatsJobDefinitions(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = emptyJobDefinitionParams{}
	_ = jobDefinitionStatsResponseBody{}
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err := ctrl.statsJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	jobStats := ctx.Result.([]*stats.JobStats)
	if jobStats == nil {
		t.Fatalf("no job stats found")
	}
}

func Test_ShouldUploadAndGetJobDefinitionWithYAML(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionBody{}
	_ = jobDefinitionIDParams{}
	newJob := newTestJobDefinition("job")
	b, err := json.Marshal(newJob)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"yaml"}}})
	err = ctrl.postJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	savedJob := ctx.Result.(*types.JobDefinition)
	if savedJob.ID == "" {
		t.Fatalf("jobDefinition-id is empty")
	}

	ctx.Params["id"] = savedJob.ID
	err = ctrl.getJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
}

func Test_ShouldUploadAndGetJobDefinition(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionBody{}
	_ = jobDefinitionIDParams{}
	newJob := newTestJobDefinition("job")
	b, err := json.Marshal(newJob)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	savedJob := ctx.Result.(*types.JobDefinition)
	if savedJob.ID == "" {
		t.Fatalf("jobDefinition-id is empty")
	}

	ctx.Params["id"] = savedJob.ID
	err = ctrl.getJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
}

func Test_ShouldUploadAndPauseJobDefinition(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionIDParams{}
	newJob := newTestJobDefinition("job")
	b, err := json.Marshal(newJob)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	savedJob := ctx.Result.(*types.JobDefinition)
	if savedJob.ID == "" {
		t.Fatalf("jobDefinition-id is empty")
	}

	ctx.Params["id"] = savedJob.ID
	err = ctrl.pauseJobDefinition(ctx)
	queryOut := emptyResponseBody{}
	if err != nil {
		t.Fatalf("unexpected error %s %v", err, queryOut)
	}
}

func Test_ShouldUploadAndUnpauseJobDefinition(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionIDParams{}
	_ = emptyResponseBody{}
	newJob := newTestJobDefinition("job")
	b, err := json.Marshal(newJob)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	savedJob := ctx.Result.(*types.JobDefinition)
	if savedJob.ID == "" {
		t.Fatalf("jobDefinition-id is empty")
	}

	ctx.Params["id"] = savedJob.ID
	err = ctrl.unpauseJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
}

func Test_ShouldUpdateConcurrency(t *testing.T) {
	mgr := newTestJobManager(newTestConfig(), t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionIDParams{}
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionConcurrencyParams{}
	newJob := newTestJobDefinition("job")
	b, err := json.Marshal(newJob)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postJobDefinition(ctx)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	savedJob := ctx.Result.(*types.JobDefinition)
	if savedJob.ID == "" {
		t.Fatalf("jobDefinition-id is empty")
	}

	ctx.Params["id"] = savedJob.ID
	ctx.Params["concurrency"] = "3"
	err = ctrl.updateConcurrencyJobDefinition(ctx)
	queryOut := emptyResponseBody{}
	if err != nil {
		t.Fatalf("unexpected error %s %v", err, queryOut)
	}
}

func newTestJobDefinition(name string) *types.JobDefinition {
	job := types.NewJobDefinition(name)
	job.UserID = "test-user"
	job.OrganizationID = "test-org"
	job.MaxConcurrency = rand.Int()+1
	task1 := types.NewTaskDefinition("task1", common.Shell)
	task1.Method = common.Docker
	job.AddTask(task1)
	_, _ = job.AddConfig("name", "value", false)
	job.UpdateRawYaml()
	return job
}
