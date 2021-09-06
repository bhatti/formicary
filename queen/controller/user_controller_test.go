package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/queen/manager"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"

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
	_ = userNotifyParams{}
}

func Test_ShouldQueryUsers(t *testing.T) {
	// GIVEN user controller
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", "test@formicary.io", false)
	_, err = userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

	// WHEN querying users
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.queryUsers(ctx)

	// THEN it should return list of users
	require.NoError(t, err)
	all := ctx.Result.(*PaginatedResult).Records.([]*common.User)
	require.NotEqual(t, 0, len(all))
}

func Test_ShouldGetUserByID(t *testing.T) {
	// GIVEN user controller
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", "test@formicary.io", false)
	_, err = userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

	// WHEN getting user by id
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = user.ID
	err = ctrl.getUser(ctx)

	// THEN it should return valid user
	require.NoError(t, err)
	saved := ctx.Result.(*common.User)
	require.NotNil(t, saved)
}

func Test_ShouldUpdateUserEmail(t *testing.T) {
	// GIVEN user controller
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("builder", "bob", "name", "test@formicary.io", false)
	b, err := json.Marshal(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

	// WHEN saving user
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("user-id", user.ID, "name", "test@formicary.io", true))
	err = ctrl.postUser(ctx)

	// THEN it should not fail and create the user
	require.NoError(t, err)
	saved := ctx.Result.(*common.User)
	require.NotNil(t, saved)

	// WHEN updating email without valid address
	ctx = web.NewStubContext(&http.Request{Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, saved)
	ctx.Params["id"] = saved.ID
	ctx.Params["email"] = "blah"
	err = ctrl.updateUserNotification(ctx)
	// THEN it should fail and update the user
	require.Error(t, err)
	require.Equal(t, "email 'blah' is not valid", err.Error())

	ctx.Params["email"] = "email1@formicary.io, email2@formicary.io"
	err = ctrl.updateUserNotification(ctx)
	// THEN it should update emails
	require.NoError(t, err)
	saved = ctx.Result.(*common.User)
	require.NotNil(t, saved)
	require.Equal(t, 2, len(saved.Notify[common.EmailChannel].Recipients))
	require.Equal(t, "email1@formicary.io", saved.Notify[common.EmailChannel].Recipients[0])
	require.Equal(t, "email2@formicary.io", saved.Notify[common.EmailChannel].Recipients[1])
}

func Test_ShouldSaveUser(t *testing.T) {
	// GIVEN user controller
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("builder", "bob", "name", "test@formicary.io", false)
	b, err := json.Marshal(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

	// WHEN saving user
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, common.NewUser("user-id", user.ID, "name", "test@formicary.io", true))
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
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", "test@formicary.io", false)
	saved, err := userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

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
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	userRepository.Clear()
	user := common.NewUser("org", "username", "name", "test@formicary.io", false)
	saved, err := userRepository.Create(user)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewUserController(manager.AssertTestUserManager(nil, t), webServer)

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
