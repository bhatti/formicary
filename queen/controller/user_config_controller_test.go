// SPDX-License-Identifier: AGPL-3.0-or-later

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

// ──────────────────────────────────────────────────────────────────────────────
// User config CRUD
// ──────────────────────────────────────────────────────────────────────────────

func Test_ShouldQueryUserConfigs(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "my_key", "my_value", false)
	require.NoError(t, err)
	_, err = configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qc.User)
	err = ctrl.queryUserConfigs(ctx)
	require.NoError(t, err)

	all := ctx.Result.(*PaginatedResult).Records.([]*common.Config)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "secret_key", "secret_val", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	err = ctrl.getUserConfig(ctx)
	require.NoError(t, err)

	got := ctx.Result.(*common.Config)
	require.NotNil(t, got)
	// Secret must be masked in get response.
	require.Equal(t, "****", got.Value)
}

func Test_ShouldRevealUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "secret_key", "plaintext_secret", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	err = ctrl.revealUserConfig(ctx)
	require.NoError(t, err)

	got := ctx.Result.(*common.Config)
	require.NotNil(t, got)
	// Reveal must return the plaintext value.
	require.Equal(t, "plaintext_secret", got.Value)
}

func Test_ShouldRevealOrgConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewOrgConfig(qc.GetOrganizationID(), "org_secret", "org_plaintext", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	ctx.Params["org"] = qc.GetOrganizationID()
	err = ctrl.revealOrganizationConfig(ctx)
	require.NoError(t, err)

	got := ctx.Result.(*common.Config)
	require.NotNil(t, got)
	// Reveal must return the plaintext value.
	require.Equal(t, "org_plaintext", got.Value)
}

func Test_ShouldCreateUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "new_key", "new_val", false)
	require.NoError(t, err)
	b, err := json.Marshal(cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Set(web.DBUser, qc.User)
	err = ctrl.postUserConfig(ctx)
	require.NoError(t, err)

	saved := ctx.Result.(*common.Config)
	require.NotNil(t, saved)
	require.Equal(t, "new_key", saved.Name)
}

func Test_ShouldUpdateUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "upd_key", "original", false)
	require.NoError(t, err)
	initial, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	initial.Value = "updated"
	b, err := json.Marshal(initial)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = initial.ID
	err = ctrl.putUserConfig(ctx)
	require.NoError(t, err)

	saved := ctx.Result.(*common.Config)
	require.Equal(t, "updated", saved.Value)
}

func Test_ShouldDeleteUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "del_key", "del_val", false)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	err = ctrl.deleteUserConfig(ctx)
	require.NoError(t, err)
}

// ──────────────────────────────────────────────────────────────────────────────
// Secret masking — GET returns ****, PUT with **** preserves the original value
// ──────────────────────────────────────────────────────────────────────────────

func Test_ShouldMaskSecretInGetUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewUserConfig(qc.GetUserID(), "token", "super_secret", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	err = ctrl.getUserConfig(ctx)
	require.NoError(t, err)

	got := ctx.Result.(*common.Config)
	require.Equal(t, "****", got.Value, "secret value must be masked in GET response")
}

func Test_ShouldPreserveSecretOnPutWithMaskUserConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	// Save a secret config.
	cfg, err := common.NewUserConfig(qc.GetUserID(), "api_token", "original_secret", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	// Simulate the UI sending back **** for the value (masked display).
	saved.Value = "****"
	b, err := json.Marshal(saved)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	err = ctrl.putUserConfig(ctx)
	require.NoError(t, err)

	// Reveal to confirm the original secret was NOT overwritten with "****".
	reader2 := io.NopCloser(strings.NewReader(""))
	ctx2 := web.NewStubContext(&http.Request{Body: reader2, URL: &url.URL{}})
	ctx2.Set(web.DBUser, qc.User)
	ctx2.Params["id"] = saved.ID
	err = ctrl.revealUserConfig(ctx2)
	require.NoError(t, err)
	revealed := ctx2.Result.(*common.Config)
	require.Equal(t, "original_secret", revealed.Value, "PUT with **** must preserve original encrypted value")
}

func Test_ShouldPreserveSecretOnPutWithMaskOrgConfig(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	cfg, err := common.NewOrgConfig(qc.GetOrganizationID(), "db_pass", "secret_db_pass", true)
	require.NoError(t, err)
	saved, err := configRepo.Save(qc, cfg)
	require.NoError(t, err)

	saved.Value = "****"
	b, err := json.Marshal(saved)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRepo, configRepo, webServer)

	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Set(web.DBUser, qc.User)
	ctx.Params["id"] = saved.ID
	ctx.Params["org"] = qc.GetOrganizationID()
	err = ctrl.putOrganizationConfig(ctx)
	require.NoError(t, err)

	// Reveal to confirm original value preserved.
	reader2 := io.NopCloser(strings.NewReader(""))
	ctx2 := web.NewStubContext(&http.Request{Body: reader2, URL: &url.URL{}})
	ctx2.Set(web.DBUser, qc.User)
	ctx2.Params["id"] = saved.ID
	ctx2.Params["org"] = qc.GetOrganizationID()
	err = ctrl.revealOrganizationConfig(ctx2)
	require.NoError(t, err)
	revealed := ctx2.Result.(*common.Config)
	require.Equal(t, "secret_db_pass", revealed.Value, "PUT with **** must preserve original encrypted value")
}

// ──────────────────────────────────────────────────────────────────────────────
// Tenant isolation — cross-tenant access must be denied
// ──────────────────────────────────────────────────────────────────────────────

func Test_ShouldDenyCrossOrgConfigAccess(t *testing.T) {
	// GIVEN two users in different orgs
	qcA, err := repository.NewTestQC()
	require.NoError(t, err)
	qcB, err := repository.NewTestQC()
	require.NoError(t, err)

	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	// Org A saves a secret config.
	cfgA, err := common.NewOrgConfig(qcA.GetOrganizationID(), "org_a_secret", "secret_value", true)
	require.NoError(t, err)
	savedA, err := configRepo.Save(qcA, cfgA)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewOrganizationConfigController(auditRepo, configRepo, webServer)

	// WHEN user from Org B tries to GET org A's config by ID
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qcB.User)
	ctx.Params["id"] = savedA.ID
	ctx.Params["org"] = qcA.GetOrganizationID()
	err = ctrl.getOrganizationConfig(ctx)

	// THEN it must return not-found (tenant isolation enforced at repo layer)
	require.Error(t, err, "user from a different org must not access another org's config")
}

func Test_ShouldDenyCrossUserConfigAccess(t *testing.T) {
	// GIVEN two different users
	qcA, err := repository.NewTestQC()
	require.NoError(t, err)
	qcB, err := repository.NewTestQC()
	require.NoError(t, err)

	auditRepo, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	configRepo, err := repository.NewTestConfigRepository()
	require.NoError(t, err)

	// User A saves a personal config.
	cfgA, err := common.NewUserConfig(qcA.GetUserID(), "user_a_token", "private_token", true)
	require.NoError(t, err)
	savedA, err := configRepo.Save(qcA, cfgA)
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewUserConfigController(auditRepo, configRepo, webServer)

	// WHEN user B tries to GET user A's config by ID
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, URL: &url.URL{}})
	ctx.Set(web.DBUser, qcB.User)
	ctx.Params["id"] = savedA.ID
	err = ctrl.getUserConfig(ctx)

	// THEN it must return not-found
	require.Error(t, err, "a user must not access another user's config")
}
