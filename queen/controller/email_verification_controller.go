package controller

import (
	"encoding/json"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
)

// EmailVerificationController structure
type EmailVerificationController struct {
	userManager *manager.UserManager
	webserver   web.Server
}

// NewEmailVerificationController instantiates controller for updating emailVerifications
func NewEmailVerificationController(
	userManager *manager.UserManager,
	webserver web.Server) *EmailVerificationController {
	emailVerificationCtrl := &EmailVerificationController{
		userManager: userManager,
		webserver:   webserver,
	}
	webserver.POST("/api/users/:id/verify_email", emailVerificationCtrl.createEmailVerification, acl.New(acl.EmailVerification, acl.Create)).Name = "create_email_verification"
	webserver.PUT("/api/users/:id/verify_email/:code", emailVerificationCtrl.verifyEmailVerification, acl.New(acl.User, acl.Update)).Name = "verify_email"
	webserver.GET("/api/users/email_verifications", emailVerificationCtrl.queryEmailVerifications, acl.New(acl.EmailVerification, acl.Query)).Name = "email_verifications"
	return emailVerificationCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/users/email_verifications emailVerifications queryEmailVerifications
// Queries emailVerifications within the organization that is allowed.
// responses:
//   200: emailVerificationQueryResponse
func (uc *EmailVerificationController) queryEmailVerifications(c web.WebContext) error {
	params, order, page, pageSize, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := uc.userManager.QueryEmailVerifications(
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

// swagger:route POST /api/users/:id/verify_email emailVerifications verifyEmailVerification
// Creates new emailVerification.
// `This requires admin access`
// responses:
//   200: emailVerificationResponse
func (uc *EmailVerificationController) createEmailVerification(c web.WebContext) error {
	emailVerification := &types.EmailVerification{}
	err := json.NewDecoder(c.Request().Body).Decode(emailVerification)
	if err != nil {
		return err
	}
	qc := web.BuildQueryContext(c)
	emailVerification.UserID = qc.UserID
	saved, err := uc.userManager.CreateEmailVerifications(qc, emailVerification)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, saved)
}

// swagger:route PUT /api/users/:id/verify_email/:code emailVerifications verifyEmailVerification
// Creates new emailVerification.
// `This requires admin access`
// responses:
//   200: emailVerificationResponse
func (uc *EmailVerificationController) verifyEmailVerification(c web.WebContext) error {
	// TODO remove this as emailVerifications will be added after oauth signup
	qc := web.BuildQueryContext(c)
	rec, err := uc.userManager.VerifyEmail(qc, qc.UserID, c.Param("code"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rec)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryEmailVerifications
// The params for querying emailVerifications.
type emailVerificationQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Name of emailVerification
	Name string `json:"name"`
	// EmailCode defines email code
	EmailCode string `json:"email_code"`
	// Email defines email
	Email string `json:"email"`
}

// Paginated results of emailVerifications matching query
// swagger:response emailVerificationQueryResponse
type emailVerificationQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*types.EmailVerification
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters createEmailVerification
// The params for creating emailVerification.
type emailVerificationCreateParams struct {
	// in:body
	Body *types.EmailVerification
}

// EmailVerification is used for email verification
// swagger:response emailVerificationResponse
type emailVerificationCreateResponseBody struct {
	// in:body
	Body *types.EmailVerification
}

// swagger:parameters verifyEmailVerification
// The params to verify email
type emailVerificationVerifyParams struct {
	// in:path
	ID string `json:"id"`
	// in:path
	Code string `json:"code"`
}
