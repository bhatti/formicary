package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
)

func Test_ShouldQueryJobDefinitions(t *testing.T) {
	mgr := manager.AssertTestJobManager(nil, t)
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionTypeParams{}
	_ = jobDefinitionBody{}
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["type"] = "io.formicary.test.my-job"
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
	mgr := manager.AssertTestJobManager(nil, t)
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionBody{}
	_ = jobDefinitionIDParams{}
	newJob := repository.NewTestJobDefinition(&common.User{}, "test-job")
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionBody{}
	_ = jobDefinitionIDParams{}
	newJob := repository.NewTestJobDefinition(&common.User{}, "test-job")
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionIDParams{}
	newJob := repository.NewTestJobDefinition(&common.User{}, "test-job")
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionIDParams{}
	_ = emptyResponseBody{}
	newJob := repository.NewTestJobDefinition(&common.User{}, "test-job")
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
	mgr := manager.AssertTestJobManager(nil, t)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	ctrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	_ = jobDefinitionIDParams{}
	_ = jobDefinitionUploadParams{}
	_ = jobDefinitionConcurrencyParams{}
	newJob := repository.NewTestJobDefinition(&common.User{}, "test-job")
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
