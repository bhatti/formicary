package controller

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"

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
	var qc = common.NewQueryContext("test-user", "test-org", "")

	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := jobDefinitionRepository.Save(qc, newTestJobDefinition("my-job"))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN querying job config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["job"] = job.ID
	err = ctrl.queryJobConfigs(ctx)
	require.NoError(t, err)

	// THEN it should match expected number of records
	all := ctx.Result.([]*types.JobDefinitionConfig)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetJobConfig(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN job config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := jobDefinitionRepository.Save(qc, newTestJobDefinition("my-job"))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN getting job config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = job.Configs[0].ID
	err = ctrl.getJobConfig(ctx)
	require.NoError(t, err)

	// THEN it should return valid config
	saved := ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)
}

func Test_ShouldUpdateJobConfig(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	user := common.NewUser("test-org", "username", "name", false)
	user.ID = qc.UserID
	// GIVEN job config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := jobDefinitionRepository.Save(qc, newTestJobDefinition("my-job"))
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
	ctx.Set(web.DBUser, user)
	ctx.Params["job"] = job.ID
	err = ctrl.postJobConfig(ctx)

	// THEN it should return saved config
	require.NoError(t, err)
	saved := ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)

	// WHEN updating job config
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, user)
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = saved.ID
	err = ctrl.putJobConfig(ctx)
	// THEN it should return updated config
	require.NoError(t, err)
	saved = ctx.Result.(*types.JobDefinitionConfig)
	require.NotNil(t, saved)
}

func Test_ShouldDeleteJobConfig(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN job config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	job, err := jobDefinitionRepository.Save(qc, newTestJobDefinition("my-job"))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobConfigController(auditRecordRepository, jobDefinitionRepository, webServer)

	// WHEN deleting job config
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["job"] = job.ID
	ctx.Params["id"] = job.Configs[0].ID
	err = ctrl.deleteJobConfig(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}
