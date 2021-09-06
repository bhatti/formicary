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
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"

	"plexobject.com/formicary/internal/web"
)

func Test_InitializeSwaggerStructsForOrganizationConfig(t *testing.T) {
	_ = orgConfigQueryParams{}
	_ = orgConfigQueryResponseBody{}
	_ = orgConfigIDParams{}
	_ = orgConfigParams{}
	_ = orgConfigBody{}
	_ = orgConfigUpdateParams{}
}

func Test_ShouldQueryOrgConfigs(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	configRepository, err := repository.NewTestOrgConfigRepository()
	require.NoError(t, err)
	orgCfg, err := common.NewOrganizationConfig("org", "name", 10,
		true)
	require.NoError(t, err)
	_, err = configRepository.Save(qc, orgCfg)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRecordRepository, configRepository, webServer)

	// WHEN querying organization config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)

	err = ctrl.queryOrganizationConfigs(ctx)
	require.NoError(t, err)

	// THEN it should match expected number of records
	all := ctx.Result.(*PaginatedResult).Records.([]*common.OrganizationConfig)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetOrgConfig(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepository, err := repository.NewTestOrgConfigRepository()
	require.NoError(t, err)
	orgCfg, err := common.NewOrganizationConfig("org", "name", 10,
		true)
	require.NoError(t, err)
	_, err = configRepository.Save(qc, orgCfg)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRecordRepository, configRepository, webServer)

	// WHEN getting organization config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = orgCfg.ID
	err = ctrl.getOrganizationConfig(ctx)
	require.NoError(t, err)

	// THEN it should return valid config
	saved := ctx.Result.(*common.OrganizationConfig)
	require.NotNil(t, saved)
}

func Test_ShouldUpdateOrgConfig(t *testing.T) {
	// GIVEN organization config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepository, err := repository.NewTestOrgConfigRepository()
	require.NoError(t, err)
	orgCfg, err := common.NewOrganizationConfig("org", "name", 10,
		true)
	require.NoError(t, err)
	b, err := json.Marshal(orgCfg)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRecordRepository, configRepository, webServer)

	// WHEN saving organization config
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("org-id", "username", "name", "email@formicary.io", false))
	err = ctrl.postOrganizationConfig(ctx)

	// THEN it should return saved config
	require.NoError(t, err)
	saved := ctx.Result.(*common.OrganizationConfig)
	require.NotNil(t, saved)

	// WHEN updating organization config
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("org-id", "username", "name", "email@formicary.io", false))
	ctx.Params["id"] = saved.ID
	err = ctrl.putOrganizationConfig(ctx)
	// THEN it should return updated config
	require.NoError(t, err)
	saved = ctx.Result.(*common.OrganizationConfig)
	require.NotNil(t, saved)
}

func Test_ShouldDeleteOrgConfig(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization config controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepository, err := repository.NewTestOrgConfigRepository()
	require.NoError(t, err)
	orgCfg, err := common.NewOrganizationConfig("org", "name", 10,
		true)
	require.NoError(t, err)
	saved, err := configRepository.Save(qc, orgCfg)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRecordRepository, configRepository, webServer)

	// WHEN deleting organization config
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = saved.ID
	err = ctrl.deleteOrganizationConfig(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}
