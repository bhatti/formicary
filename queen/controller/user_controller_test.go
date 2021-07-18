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
	"plexobject.com/formicary/queen/types"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/web"
)

func Test_InitializeSwaggerStructsForUserController(t *testing.T) {
	_ = userQueryParams{}
	_ = userQueryResponseBody{}
	_ = userIDParams{}
	_ = userParams{}
	_ = userResponseBody{}
	_ = userTokenQueryParams{}
	_ = userTokenQueryResponseBody{}
	_ = userTokenResponseBody{}
	_ = userTokenDeleteParams{}
}

func Test_ShouldQueryUsers(t *testing.T) {
	// GIVEN user controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", false)
	user.Email = "test@formicary.io"
	_, err = userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	cfg := newTestConfig()
	ctrl := NewUserController(&cfg.CommonConfig, auditRecordRepository, userRepository, webServer)

	// WHEN querying users
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.queryUsers(ctx)

	// THEN it should valid list of users
	require.NoError(t, err)
	all := ctx.Result.(*PaginatedResult).Records.([]*common.User)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetUserByID(t *testing.T) {
	// GIVEN user controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", false)
	user.Email = "test@formicary.io"
	_, err = userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	cfg := newTestConfig()
	ctrl := NewUserController(&cfg.CommonConfig, auditRecordRepository, userRepository, webServer)

	// WHEN getting user by id
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = user.ID
	err = ctrl.getUser(ctx)

	// THEN it should valid user
	require.NoError(t, err)
	saved := ctx.Result.(*common.User)
	require.NotNil(t, saved)
}

func Test_ShouldSaveUser(t *testing.T) {
	// GIVEN user controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("builder", "bob", "name", false)
	user.Email = "test@formicary.io"
	b, err := json.Marshal(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	cfg := newTestConfig()
	ctrl := NewUserController(&cfg.CommonConfig, auditRecordRepository, userRepository, webServer)

	// WHEN saving user
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("user-id", user.ID, "name", true))
	err = ctrl.postUser(ctx)

	// THEN it should not fail and create the user
	require.NoError(t, err)
	saved := ctx.Result.(*common.User)
	require.NotNil(t, saved)

	// WHEN updating user via PUT
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, saved)
	ctx.Params["id"] = saved.ID
	err = ctrl.putUser(ctx)
	// THEN it should not fail and update the user
	require.NoError(t, err)
	saved = ctx.Result.(*common.User)
	require.NotNil(t, saved)
}

func Test_ShouldDeleteUser(t *testing.T) {
	// GIVEN user controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", false)
	user.Email = "test@formicary.io"
	saved, err := userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	cfg := newTestConfig()
	ctrl := NewUserController(&cfg.CommonConfig, auditRecordRepository, userRepository, webServer)

	// WHEN deleting user by id
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = saved.ID
	err = ctrl.deleteUser(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldAddUserToken(t *testing.T) {
	// GIVEN user controller
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", false)
	user.Email = "test@formicary.io"
	saved, err := userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	cfg := newTestConfig()
	ctrl := NewUserController(&cfg.CommonConfig, auditRecordRepository, userRepository, webServer)

	// WHEN adding user token
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = saved.ID
	ctx.Set(web.DBUser, saved)
	err = ctrl.createUserToken(ctx)
	// THEN it should not fail
	require.NoError(t, err)
	token := ctx.Result.(*types.UserToken)
	require.NotNil(t, token)

	// WHEN querying user tokens
	err = ctrl.queryUserTokens(ctx)
	ctx.Params["user"] = saved.ID
	ctx.Params["id"] = token.ID
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN querying user tokens
	err = ctrl.deleteUserToken(ctx)
	ctx.Params["user"] = saved.ID
	ctx.Params["id"] = token.ID
	// THEN it should not fail
	require.NoError(t, err)
}
