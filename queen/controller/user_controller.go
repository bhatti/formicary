package controller

import (
	"encoding/json"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"time"

	"plexobject.com/formicary/internal/web"
)

// UserController structure
type UserController struct {
	userManager *manager.UserManager
	webserver   web.Server
}

// NewUserController instantiates controller for updating users
func NewUserController(
	userManager *manager.UserManager,
	webserver web.Server) *UserController {
	userCtrl := &UserController{
		userManager: userManager,
		webserver:   webserver,
	}
	webserver.GET("/api/users", userCtrl.queryUsers, acl.New(acl.User, acl.Query)).Name = "query_users"
	webserver.GET("/api/users/:id", userCtrl.getUser, acl.New(acl.User, acl.View)).Name = "get_user"
	webserver.POST("/api/users", userCtrl.postUser, acl.New(acl.User, acl.Create)).Name = "create_user"
	webserver.PUT("/api/users/:id", userCtrl.putUser, acl.New(acl.User, acl.Update)).Name = "update_user"
	webserver.PUT("/api/users/:id/notify", userCtrl.updateUserNotification, acl.New(acl.User, acl.Update)).Name = "update_user_notify"
	webserver.DELETE("/api/users/:id", userCtrl.deleteUser, acl.New(acl.User, acl.Delete)).Name = "delete_user"
	webserver.GET("/api/users/:user/tokens", userCtrl.queryUserTokens, acl.New(acl.User, acl.View)).Name = "user_tokens"
	webserver.POST("/api/users/:user/tokens", userCtrl.createUserToken, acl.New(acl.User, acl.Update)).Name = "create_user_token"
	webserver.POST("/api/users/:user/tokens/:id/delete", userCtrl.deleteUserToken, acl.New(acl.User, acl.Update)).Name = "delete_user_token"
	return userCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/users users queryUsers
// Queries users within the organization that is allowed.
// responses:
//   200: userQueryResponse
func (uc *UserController) queryUsers(c web.WebContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := uc.userManager.QueryUsers(
		qc,
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// swagger:route PUT /api/users/{id}/notify users putUserNotify
// Updates user notification.
// responses:
//   200: userResponse
// TODO add email verification if email is different than user email
func (uc *UserController) updateUserNotification(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.UpdateUserNotification(
		qc,
		c.Param("id"),
		c.FormValue("email"),
		c.FormValue("slackChannel"),
		c.FormValue("slackToken"),
		c.FormValue("when"),
	)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

// swagger:route POST /api/users users postUser
// Creates new user.
// `This requires admin access`
// responses:
//   200: userResponse
func (uc *UserController) postUser(c web.WebContext) error {
	// TODO remove this as users will be added after oauth signup
	now := time.Now()
	user := common.NewUser("", "", "", "", false)
	err := json.NewDecoder(c.Request().Body).Decode(user)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	user.OrganizationID = qc.OrganizationID
	saved, err := uc.userManager.CreateUser(qc, user)
	if err != nil {
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	return c.JSON(status, saved)
}

// swagger:route PUT /api/users/{id} users putUser
// Updates user profile.
// responses:
//   200: userResponse
func (uc *UserController) putUser(c web.WebContext) error {
	user := common.NewUser("", "", "", "", false)
	err := json.NewDecoder(c.Request().Body).Decode(user)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	user.OrganizationID = qc.OrganizationID
	user.ID = qc.UserID
	saved, err := uc.userManager.UpdateUser(qc, user)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/users/{id} users getUser
// Finds user profile by its id.
// responses:
//   200: userResponse
func (uc *UserController) getUser(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.GetUser(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

// swagger:route DELETE /api/users/{id} users deleteUser
// Deletes the user profile by its id.
// responses:
//   200: emptyResponse
func (uc *UserController) deleteUser(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.DeleteUser(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// swagger:route GET /api/users/{userId}/tokens user-tokens queryUserTokens
// Queries user-tokens for the API access.
// responses:
//   200: userTokenQueryResponse
func (uc *UserController) queryUserTokens(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	tokens, err := uc.userManager.GetUserTokens(qc, c.Param("user"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tokens)
}

// deleteUserToken - deletes user token by id
// swagger:route DELETE /api/users/{userId}/tokens/{id} user-tokens deleteUserToken
// Deletes user-token by its id so that it cannot be used for the API access.
// responses:
//   200: emptyResponse
func (uc *UserController) deleteUserToken(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.RevokeUserToken(qc, qc.UserID, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// swagger:route POST /api/users/{userId}/tokens user-tokens createUserToken
// Creates new user-token for the API access.
// responses:
//   200: userTokenResponse
func (uc *UserController) createUserToken(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	name := c.FormValue("token")
	if name == "" {
		name = c.QueryParam("token")
	}
	if name == "" {
		name = "api token"
	}
	tok, err := uc.userManager.CreateUserToken(qc, web.GetDBLoggedUserFromSession(c), name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tok)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryUsers
// The params for querying users.
type userQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Name of user
	Name string `json:"name"`
	// Username defines username
	Username string `json:"username"`
	// Email defines email
	Email string `json:"email"`
}

// Paginated results of users matching query
// swagger:response userQueryResponse
type userQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*common.User
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters userIDParams deleteUser getUser putUser
// The parameters for finding user by id
type userIDParams struct {
	// in:path
	ID string `json:"id"`
}

// swagger:parameters userNotifyParams putUserNotify
// The parameters for finding user by id
type userNotifyParams struct {
	// in:path
	ID string `json:"id"`
	// in:formData
	Email        string `json:"email"`
	SlackChannel string `json:"slackChannel"`
	When         string `json:"when"`
}

// swagger:parameters postUser putUser putUserNotify
// The request body includes user for persistence.
type userParams struct {
	// in:body
	Body *common.User
}

// User of the system who can create job-definitions, submit and execute jobs.
// swagger:response userResponse
type userResponseBody struct {
	// in:body
	Body *common.User
}

// swagger:parameters queryUserTokens createUserToken
// The params for querying or creating user tokens.
type userTokenQueryParams struct {
	// in:path
	UserID string `json:"userId"`
}

// swagger:parameters deleteUserToken
// The params for deleting user tokens.
type userTokenDeleteParams struct {
	// in:path
	UserID string `json:"userId"`
	// in:path
	ID string `json:"id"`
}

// Results of user-tokens
// swagger:response userTokenQueryResponse
type userTokenQueryResponseBody struct {
	// in:body
	Body []types.UserToken
}

// User-token for the API access.
// swagger:response userTokenResponse
type userTokenResponseBody struct {
	// in:body
	Body types.UserToken
}
