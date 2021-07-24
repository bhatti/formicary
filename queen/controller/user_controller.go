package controller

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/security"
	"plexobject.com/formicary/queen/types"
	"time"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

// UserController structure
type UserController struct {
	commonCfg              *common.CommonConfig
	auditRecordRepository  repository.AuditRecordRepository
	userRepository         repository.UserRepository
	subscriptionRepository repository.SubscriptionRepository
	webserver              web.Server
}

// NewUserController instantiates controller for updating users
func NewUserController(
	commonCfg *common.CommonConfig,
	auditRecordRepository repository.AuditRecordRepository,
	userRepository repository.UserRepository,
	subscriptionRepository repository.SubscriptionRepository,
	webserver web.Server) *UserController {
	userCtrl := &UserController{
		commonCfg:              commonCfg,
		auditRecordRepository:  auditRecordRepository,
		userRepository:         userRepository,
		subscriptionRepository: subscriptionRepository,
		webserver:              webserver,
	}
	webserver.GET("/api/users", userCtrl.queryUsers, acl.New(acl.User, acl.Query)).Name = "query_users"
	webserver.GET("/api/users/:id", userCtrl.getUser, acl.New(acl.User, acl.View)).Name = "get_user"
	webserver.POST("/api/users", userCtrl.postUser, acl.New(acl.User, acl.Create)).Name = "create_user"
	webserver.PUT("/api/users/:id", userCtrl.putUser, acl.New(acl.User, acl.Update)).Name = "update_user"
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
	params, order, page, pageSize, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := uc.userRepository.Query(
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

// swagger:route POST /api/users users postUser
// Creates new user.
// `This requires admin access`
// responses:
//   200: userResponse
func (uc *UserController) postUser(c web.WebContext) error {
	// TODO remove this as users will be added after oauth signup
	now := time.Now()
	user := common.NewUser("", "", "", false)
	err := json.NewDecoder(c.Request().Body).Decode(user)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	user.OrganizationID = qc.OrganizationID
	saved, err := uc.userRepository.Create(user)
	if err != nil {
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromUser(saved, types.UserUpdated, qc))

	subscription := common.NewFreemiumSubscription(saved.ID, saved.OrganizationID)
	if subscription, err = uc.subscriptionRepository.Create(subscription); err == nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
		}).Info("created Subscription")
		_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(subscription, qc))
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
			"Error":        err,
		}).Errorf("failed to create Subscription")
	}
	return c.JSON(status, saved)
}

// swagger:route PUT /api/users/{id} users putUser
// Updates user profile.
// responses:
//   200: userResponse
func (uc *UserController) putUser(c web.WebContext) error {
	user := common.NewUser("", "", "", false)
	err := json.NewDecoder(c.Request().Body).Decode(user)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	user.OrganizationID = qc.OrganizationID
	user.ID = qc.UserID
	saved, err := uc.userRepository.Update(qc, user)
	if err != nil {
		return err
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromUser(saved, types.UserUpdated, qc))
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/users/{id} users getUser
// Finds user profile by its id.
// responses:
//   200: userResponse
func (uc *UserController) getUser(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	user, err := uc.userRepository.Get(qc, c.Param("id"))
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
	err := uc.userRepository.Delete(qc, c.Param("id"))
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
	tokens, err := uc.userRepository.GetTokens(qc, c.Param("user"))
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
	err := uc.userRepository.RevokeToken(qc, qc.UserID, c.Param("id"))
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
	tok := types.NewUserToken(qc.UserID, qc.OrganizationID, name)
	strTok, expiration, err := security.BuildToken(web.GetDBLoggedUserFromSession(c), uc.commonCfg.Auth.JWTSecret, uc.commonCfg.Auth.MaxAge)
	if err == nil {
		tok.APIToken = strTok
		tok.ExpiresAt = expiration
		err = uc.userRepository.AddToken(tok)
	}
	if err != nil {
		return err
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromToken(tok, qc))
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

// swagger:parameters postUser putUser
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
