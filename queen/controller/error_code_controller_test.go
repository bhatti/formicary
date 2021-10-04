package controller

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"
)

func Test_InitializeSwaggerStructsForErrorCode(t *testing.T) {
	_ = errorCodesQueryParams{}
	_ = errorCodesQueryResponseBody{}

	_ = errorCodeParams{}
	_ = errorCodeResponseBody{}
	_ = errorCoderIDParams{}
}

func Test_ShouldQueryErrorCodes(t *testing.T) {
	// GIVEN error-code controller
	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	_, err = errorCodeRepository.Save(
		qc,
		types.NewErrorCode("myjob", "regex", "cmd", "err-code"))
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewErrorCodeController(errorCodeRepository, webServer)

	// WHEN querying error codes
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Set(web.DBUser, qc.User)
	err = ctrl.queryErrorCodes(ctx)

	// THEN it should not fail nad return error codes
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*types.ErrorCode)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldCreateAndGetErrorCode(t *testing.T) {
	// GIVEN error-code controller
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewErrorCodeController(errorCodeRepository, webServer)

	// WHEN creating error-code
	b, err := json.Marshal(types.NewErrorCode("job", "regex", "", "err-code"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postErrorCode(ctx)

	// THEN it should not fail and add error code
	require.NoError(t, err)
	errorCode := ctx.Result.(*types.ErrorCode)
	require.NotEqual(t, "", errorCode.ID)

	// WHEN getting error code
	ctx.Params["id"] = errorCode.ID
	err = ctrl.getErrorCode(ctx)
	// THEN it should not fail and add error code
	require.NoError(t, err)
	errorCode = ctx.Result.(*types.ErrorCode)
	require.NotEqual(t, "", errorCode.ID)
}

func Test_ShouldUpdateAndGetErrorCode(t *testing.T) {
	// GIVEN error-code controller
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewErrorCodeController(errorCodeRepository, webServer)

	// WHEN updating error code
	b, err := json.Marshal(types.NewErrorCode("job", "regex", "", "err-code"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.putErrorCode(ctx)

	// THEN it should not fail and update error code
	require.NoError(t, err)
	errorCode := ctx.Result.(*types.ErrorCode)
	require.NotEqual(t, "", errorCode.ID)

	// WHEN getting error code
	ctx.Params["id"] = errorCode.ID
	err = ctrl.getErrorCode(ctx)
	// THEN it should not fail and update error code
	require.NoError(t, err)
	errorCode = ctx.Result.(*types.ErrorCode)
	require.NotEqual(t, "", errorCode.ID)
}

func Test_ShouldAddAndDeleteErrorCode(t *testing.T) {
	// GIVEN error-code controller
	errorCodeRepository, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewErrorCodeController(errorCodeRepository, webServer)

	// WHEN adding error code
	b, err := json.Marshal(types.NewErrorCode("job", "regex", "", "err-code"))
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postErrorCode(ctx)
	// THEN it should not fail and add error code
	require.NoError(t, err)
	errorCode := ctx.Result.(*types.ErrorCode)
	require.NotEqual(t, "", errorCode.ID)

	// WHEN deleting error code
	ctx.Params["id"] = errorCode.ID
	err = ctrl.deleteErrorCode(ctx)
	// THEN it should not fail
	require.NoError(t, err)
}
