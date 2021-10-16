package admin

import (
	"fmt"
	"net/http"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
)

// EmailVerificationAdminController structure
type EmailVerificationAdminController struct {
	userManager *manager.UserManager
	webserver   web.Server
}

// NewEmailVerificationAdminController admin dashboard for managing error-codes -- admin only
func NewEmailVerificationAdminController(
	userManager *manager.UserManager,
	webserver web.Server) *EmailVerificationAdminController {
	ctr := &EmailVerificationAdminController{
		userManager: userManager,
		webserver:   webserver,
	}
	webserver.POST("/dashboard/users/:user/create_verify_email", ctr.createEmailVerification, acl.NewPermission(acl.EmailVerification, acl.Create)).Name = "create_admin_email_verification"
	webserver.GET("/dashboard/users/verify_email/:id", ctr.showEmailVerification, acl.NewPermission(acl.User, acl.Update)).Name = "verify_admin_email"
	webserver.POST("/dashboard/users/:user/verify_email", ctr.verifyEmailVerification, acl.NewPermission(acl.User, acl.Update)).Name = "verify_admin_email"
	webserver.GET("/dashboard/users/email_verifications", ctr.queryEmailVerifications, acl.NewPermission(acl.EmailVerification, acl.Query)).Name = "email_admin_verifications"

	return ctr
}

// ********************************* HTTP Handlers ***********************************
// queryEmailVerifications - queries error-code
func (ctr *EmailVerificationAdminController) queryEmailVerifications(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	recs, total, err := ctr.userManager.QueryEmailVerifications(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/users/email_verifications?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "email_verification/index", res)
}

func (ctr *EmailVerificationAdminController) createEmailVerification(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	res := map[string]interface{}{}
	user := web.GetDBUserFromSession(c)
	email := c.FormValue("email")
	if email == "" && user != nil {
		email = user.Email
	}
	res["Email"] = email
	id := ""
	if user == nil {
		err = common.NewNotFoundError("user not found")
	} else {
		res["User"] = user
		ev := types.NewEmailVerification(email, user)
		err = ev.Validate()
		if err == nil {
			_, err = ctr.userManager.CreateEmailVerification(qc, ev)
			id = ev.ID
		}
	}
	web.RenderDBUserFromSession(c, res)
	if err != nil {
		res["Error"] = err
		return c.Render(http.StatusOK, "email_verification/verify_email", res)
	}
	return c.Redirect(http.StatusFound, "/dashboard/users/verify_email/"+id)
}

func (ctr *EmailVerificationAdminController) showEmailVerification(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	res := map[string]interface{}{"EmailCode": ""}
	user := web.GetDBUserFromSession(c)
	id := c.Param("id")
	if user == nil {
		err = common.NewNotFoundError("user not found")
		res["Error"] = err
	} else {
		res["User"] = user
		res["ID"] = id
		rec, err := ctr.userManager.GetVerifiedEmailByID(qc, id)
		if err != nil {
			res["Error"] = err
		} else {
			res["Email"] = rec.Email
			res["EmailCode"] = rec.EmailCode
		}
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "email_verification/verify_email", res)
}

func (ctr *EmailVerificationAdminController) verifyEmailVerification(c web.APIContext) (err error) {
	res := map[string]interface{}{"EmailCode": ""}
	qc := web.BuildQueryContext(c)
	user := web.GetDBUserFromSession(c)
	id := c.Param("id")
	if id == "" {
		id = c.FormValue("id")
	}
	if user == nil {
		err = common.NewNotFoundError("user not found")
	} else {
		res["User"] = user
	}
	_, err = ctr.userManager.VerifyEmail(qc, qc.GetUserID(), c.FormValue("code"))
	web.RenderDBUserFromSession(c, res)
	if err != nil {
		res["Error"] = err
		res["ID"] = id
		if rec, dbErr := ctr.userManager.GetVerifiedEmailByID(qc, id); dbErr == nil {
			res["Email"] = rec.Email
			res["EmailCode"] = rec.EmailCode
		}
		return c.Render(http.StatusOK, "email_verification/verify_email", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/"+user.ID))
}
