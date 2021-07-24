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
)

func Test_InitializeSwaggerStructsForOrganizations(t *testing.T) {
	_ = orgQueryParams{}
	_ = orgQueryResponseBody{}
	_ = orgIDParams{}
	_ = orgCreateParams{}
	_ = orgUpdateParams{}
	_ = orgResponseBody{}
	_ = userInvitationResponseBody{}
}

func Test_ShouldQueryOrgs(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	organizationRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	organizationRepository.Clear()
	org := common.NewOrganization("user", "org", "name")
	// AND an existing organization
	_, err = organizationRepository.Create(qc, org)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationController(auditRecordRepository, organizationRepository, webServer)

	// WHEN querying organizations
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.queryOrganizations(ctx)

	// THEN it should not fail and return organizations
	require.NoError(t, err)
	all := ctx.Result.(*PaginatedResult).Records.([]*common.Organization)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetOrgByID(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	organizationRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	organizationRepository.Clear()
	org := common.NewOrganization("user", "org", "name")
	// AND an existing organization
	_, err = organizationRepository.Create(qc, org)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationController(auditRecordRepository, organizationRepository, webServer)

	// WHEN getting organization
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = org.ID
	err = ctrl.getOrganization(ctx)

	// THEN it should not fail nad return organization
	require.NoError(t, err)
	saved := ctx.Result.(*common.Organization)
	require.NotNil(t, saved)
}

func Test_ShouldSaveOrg(t *testing.T) {
	// GIVEN organization controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	organizationRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	organizationRepository.Clear()
	org := common.NewOrganization("user", "org", "name")
	b, err := json.Marshal(org)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()

	// WHEN saving organization
	ctrl := NewOrganizationController(auditRecordRepository, organizationRepository, webServer)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("org-id", "username", "name", false))
	err = ctrl.postOrganization(ctx)

	// THEN it should not fail nad return organization
	require.NoError(t, err)
	saved := ctx.Result.(*common.Organization)
	require.NotNil(t, saved)

	// WHEN updating organization
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser(saved.ID, "username", "name", false))
	ctx.Params["id"] = saved.ID
	err = ctrl.putOrganization(ctx)

	// THEN it should not fail nad return organization
	require.NoError(t, err)
	saved = ctx.Result.(*common.Organization)
	require.NotNil(t, saved)
}

func Test_ShouldDeleteOrg(t *testing.T) {
	var qc = common.NewQueryContext("test-user", "test-org", "")
	// GIVEN organization controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	organizationRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	organizationRepository.Clear()
	// AND existing organization
	org := common.NewOrganization("user", "org", "name")
	saved, err := organizationRepository.Create(qc, org)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationController(auditRecordRepository, organizationRepository, webServer)

	// WHEN deleting organization
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = saved.ID
	err = ctrl.deleteOrganization(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}
