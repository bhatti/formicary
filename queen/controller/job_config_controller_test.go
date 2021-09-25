package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/queen/repository"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/types"
)

func Test_InitializeSwaggerStructsForJobConfig(t *testing.T) {
	_ = jobConfigQueryParams{}
	_ = jobConfigQueryResponseBody{}
	_ = jobConfigIDParams{}
	_ = jobConfigParams{}
	_ = jobConfigBody{}
	_ = jobConfigUpdateParams{}
}

func Test_ShouldQueryJobConfigs(t *testing.T) {
	// GIVEN job config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := repository.SaveTestJobDefinition(qc, "my-job", "")
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN querying job config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["job"] = job.ID
	err = ctrl.queryJobConfigs(ctx)
	require.NoError(t, err)

	// THEN it should match expected number of records
	all := ctx.Result.([]*types.JobDefinitionConfig)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetJobConfig(t *testing.T) {
	// GIVEN job config controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := repository.SaveTestJobDefinition(qc, "my-job", "")
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN getting job config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = job.Configs[0].ID
	err = ctrl.getJobConfig(ctx)
	require.NoError(t, err)

	// THEN it should return valid config
	saved := ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)
}

func Test_ShouldUpdateJobConfig(t *testing.T) {
	// GIVEN job config controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := repository.SaveTestJobDefinition(qc, "my-job", "")
	require.NoError(t, err)
	require.NotNil(t, job)
	jobCfg, err := job.AddConfig("k1", "v1", false)
	require.NoError(t, err)
	b, err := json.Marshal(jobCfg)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN saving job config
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["job"] = job.ID
	err = ctrl.postJobConfig(ctx)

	// THEN it should return saved config
	require.NoError(t, err)
	saved := ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)

	// WHEN updating job config
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = saved.ID
	err = ctrl.putJobConfig(ctx)
	// THEN it should return updated config
	require.NoError(t, err)
	saved = ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)
}

func Test_ShouldDeleteJobConfig(t *testing.T) {
	// GIVEN job config controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := repository.SaveTestJobDefinition(qc, "my-job", "")
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN deleting job config
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = job.Configs[0].ID
	ctx.Set(web.DBUser, qc.User)
	err = ctrl.deleteJobConfig(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}
