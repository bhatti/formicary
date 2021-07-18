package controller

import (
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/web"
)

func Test_InitializeSwaggerStructsForAuditController(t *testing.T) {
	_ = auditQueryParams{}
	_ = auditQueryResponseBody{}
}

func Test_ShouldQueryAudits(t *testing.T) {
	// GIVEN audit controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	auditRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	audit := types.NewAuditRecord(types.UserLogin, "login")
	_, err = auditRepository.Save(audit)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewAuditController(auditRecordRepository, webServer)

	// WHEN querying audits
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.queryAudits(ctx)

	// THEN it should valid list of audits
	require.NoError(t, err)
	all := ctx.Result.(*PaginatedResult).Records.([]*types.AuditRecord)
	require.NotEqual(t, 0, len(all))
}
