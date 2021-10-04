package controller

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/types"
)

func Test_InitializeSwaggerStructsForJobResource(t *testing.T) {
	_ = jobResourceQueryParams{}
	_ = jobResourceQueryResponseBody{}
	_ = jobResourceIDParams{}
	_ = jobResourceCreateParams{}
	_ = jobResourceUpdateParams{}
	_ = jobResourceBody{}
	_ = jobResourceConfigDeleteParams{}
	_ = jobResourceConfigBody {}
}

func Test_ShouldQueryJobResources(t *testing.T) {
	// GIVEN job-resource controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	_, err = jobResourceRepository.Save(qc, types.NewJobResource("res1", 10))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN querying job-resources
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Set(web.DBUser, qc.User)
	err = ctrl.queryJobResources(ctx)

	// THEN it should not fail and return job resources
	require.NoError(t, err)
	jobs := ctx.Result.(*PaginatedResult).Records.([]*types.JobResource)
	require.NotEqual(t, 0, len(jobs))
}

func Test_ShouldGetJobResources(t *testing.T) {
	// GIVEN job-resource controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	jobRes, err := jobResourceRepository.Save(qc, types.NewJobResource("res1", 10))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN getting job-resource
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = jobRes.ID
	err = ctrl.getJobResource(ctx)

	// THEN it should not fail and return job resource
	require.NoError(t, err)
	saved := ctx.Result.(*types.JobResource)
	require.NotNil(t, saved)
}

func Test_ShouldSaveJobResource(t *testing.T) {
	// GIVEN job-resource controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	b, err := json.Marshal(types.NewJobResource("res1", 10))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN saving job-resource
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	err = ctrl.postJobResource(ctx)

	// THEN it should not fail and add job resource
	require.NoError(t, err)
	savedJob := ctx.Result.(*types.JobResource)
	require.NotNil(t, savedJob)

	// WHEN updating job-resource by id
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = savedJob.ID
	err = ctrl.putJobResource(ctx)

	// THEN it should not fail and add job resource
	require.NoError(t, err)
	savedJob = ctx.Result.(*types.JobResource)
	require.NotNil(t, savedJob)
}

func Test_ShouldPauseJobResource(t *testing.T) {
	// GIVEN job-resource controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	jobRes, err := jobResourceRepository.Save(qc, types.NewJobResource("res1", 10))
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN pausing job-resource
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = jobRes.ID
	err = ctrl.pauseJobResource(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldUnpauseJobResource(t *testing.T) {
	// GIVEN job-resource controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	jobRes, err := jobResourceRepository.Save(qc, types.NewJobResource("res1", 10))
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN unpausing job-resource
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = jobRes.ID
	err = ctrl.unpauseJobResource(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldDeleteJobResource(t *testing.T) {
	// GIVEN job resource controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobResourceRepository, err := repository.NewTestJobResourceRepository()
	require.NoError(t, err)
	jobRes, err := jobResourceRepository.Save(qc, types.NewJobResource("res1", 10))
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewJobResourceController(auditRecordRepository, jobResourceRepository, webServer)

	// WHEN deleting job-resource
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = jobRes.ID
	err = ctrl.deleteJobResource(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

