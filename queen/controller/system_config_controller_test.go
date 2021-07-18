package controller

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"strings"
	"testing"
)

func Test_InitializeSwaggerStructsForSystemConfigController(t *testing.T) {
	_ = sysConfigsQueryParams{}
	_ = sysConfigsQueryResponseBody{}
	_ = sysConfigCreateParams{}
	_ = sysConfigUpdateParams{}
	_ = sysConfigResponseBody{}
	_ = sysConfigIDParams{}
}

func Test_ShouldQuerySystemConfigs(t *testing.T) {
	// GIVEN system config controller
	sysConfigRepository, err := repository.NewTestSystemConfigRepository()
	require.NoError(t, err)
	_, err = sysConfigRepository.Save(types.NewSystemConfig("scope", "kind", "name", "value"))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewSystemConfigController(sysConfigRepository, webServer)

	// WHEN querying system config
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.querySystemConfigs(ctx)

	// THEN it should return valid configs
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*types.SystemConfig)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldCreateAndGetSystemConfig(t *testing.T) {
	// GIVEN system config controller
	sysConfigRepository, err := repository.NewTestSystemConfigRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewSystemConfigController(sysConfigRepository, webServer)
	b, err := json.Marshal(types.NewSystemConfig("scope", "kind", "name", "value"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})

	// WHEN creating system config
	err = ctrl.postSystemConfig(ctx)

	// THEN it should return system config
	require.NoError(t, err)
	sysConfig := ctx.Result.(*types.SystemConfig)
	require.NotEqual(t, "", sysConfig.ID)

	// WHEN getting system config
	ctx.Params["id"] = sysConfig.ID
	err = ctrl.getSystemConfig(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldUpdateAndGetSystemConfig(t *testing.T) {
	// GIVEN system config controller
	sysConfigRepository, err := repository.NewTestSystemConfigRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewSystemConfigController(sysConfigRepository, webServer)

	// WHEN updating system config
	b, err := json.Marshal(types.NewSystemConfig("scope", "kind", "name", "value"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.putSystemConfig(ctx)

	// THEN it should return system config
	require.NoError(t, err)
	sysConfig := ctx.Result.(*types.SystemConfig)
	require.NotEqual(t, "", sysConfig.ID)

	// WHEN getting system config
	ctx.Params["id"] = sysConfig.ID
	err = ctrl.getSystemConfig(ctx)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldAddAndDeleteSystemConfig(t *testing.T) {
	// GIVEN system config controller
	sysConfigRepository, err := repository.NewTestSystemConfigRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewSystemConfigController(sysConfigRepository, webServer)

	// WHEN adding system config
	b, err := json.Marshal(types.NewSystemConfig("scope", "kind", "name", "value"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postSystemConfig(ctx)

	// THEN it should return system config
	require.NoError(t, err)
	sysConfig := ctx.Result.(*types.SystemConfig)
	require.NotEqual(t, "", sysConfig.ID)

	// WHEN deleting system config
	ctx.Params["id"] = sysConfig.ID
	err = ctrl.deleteSystemConfig(ctx)
	require.NoError(t, err)
}
