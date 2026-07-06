package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/security"
	"plexobject.com/formicary/queen/types"
)

// UserAdminController structure
type UserAdminController struct {
	commonCfg          *common.CommonConfig
	userManager        *manager.UserManager
	jobExecRepository  repository.JobExecutionRepository
	artifactRepository repository.ArtifactRepository
	webserver          web.Server
}

// NewUserAdminController admin dashboard for managing users
func NewUserAdminController(
	commonCfg *common.CommonConfig,
	userManager *manager.UserManager,
	jobExecRepository repository.JobExecutionRepository,
	artifactRepository repository.ArtifactRepository,
	webserver web.Server) *UserAdminController {
	jraCtr := &UserAdminController{
		commonCfg:          commonCfg,
		userManager:        userManager,
		jobExecRepository:  jobExecRepository,
		artifactRepository: artifactRepository,
		webserver:          webserver,
	}
	webserver.GET("/dashboard/users", jraCtr.queryUsers, acl.NewPermission(acl.User, acl.Query)).Name = "query_admin_users"
	webserver.GET("/dashboard/users/new", jraCtr.newUser, acl.NewPermission(acl.User, acl.Signup)).Name = "new_admin_users"
	webserver.POST("/dashboard/users", jraCtr.createUser, acl.NewPermission(acl.User, acl.Signup)).Name = "create_admin_users"
	webserver.POST("/dashboard/users/:id", jraCtr.updateUser, acl.NewPermission(acl.User, acl.Update)).Name = "update_admin_users"
	webserver.POST("/dashboard/users/:id/notify", jraCtr.updateUserNotification, acl.NewPermission(acl.User, acl.Update)).Name = "update_admin_users_notify"
	webserver.GET("/dashboard/users/:id", jraCtr.getUser, acl.NewPermission(acl.User, acl.View)).Name = "get_admin_users"
	webserver.GET("/dashboard/users/:id/edit", jraCtr.editUser, acl.NewPermission(acl.User, acl.Update)).Name = "edit_admin_users"
	webserver.POST("/dashboard/users/:id/delete", jraCtr.deleteUser, acl.NewPermission(acl.User, acl.Delete)).Name = "delete_admin_users"
	webserver.GET("/dashboard/users/:user/tokens/new", jraCtr.newUserToken, acl.NewPermission(acl.User, acl.Update)).Name = "new_admin_user_token"
	webserver.POST("/dashboard/users/:user/tokens", jraCtr.createUserToken, acl.NewPermission(acl.User, acl.Update)).Name = "create_admin_user_token"
	webserver.POST("/dashboard/users/:user/tokens/:id/delete", jraCtr.deleteUserToken, acl.NewPermission(acl.User, acl.Update)).Name = "delete_admin_user_token"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryUsers - queries user
func (uc *UserAdminController) queryUsers(c web.APIContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
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
	baseURL := fmt.Sprintf("/users?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/index", res)
}

// newUser - creates a new user
func (uc *UserAdminController) newUser(c web.APIContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session to create user")
	}
	// Clear any org fields inherited from a personal org so the form
	// always starts as "individual" unless an invitation is present.
	user.OrgUnit = ""
	user.BundleID = ""
	user.InvitationCode = ""
	isOrgSignup := false
	invID := c.QueryParam("inv_id")
	if invID != "" {
		if inv, err := uc.userManager.GetInvitation(invID); err == nil {
			user.OrganizationID = inv.OrganizationID
			user.InvitationCode = inv.InvitationCode
			user.OrgUnit = inv.OrgUnit
			isOrgSignup = true
		}
	}
	res := map[string]interface{}{
		"User":         user,
		"InvitationID": invID,
		"IsOrgSignup":  isOrgSignup,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/new", res)
}

// createUser - saves a new user
func (uc *UserAdminController) createUser(c web.APIContext) (err error) {
	invID := c.FormValue("inv_id")
	user, err := uc.createUserFromForm(c)
	if err != nil {
		if user == nil {
			// Reconstruct a display user from the session so the username stays visible.
			sessionUser := web.GetDBLoggedUserFromSession(c)
			if sessionUser != nil {
				user = sessionUser
			} else {
				user = common.NewUser("", "", "", "", acl.NewRoles(""))
			}
			user.Errors = map[string]string{"Error": err.Error()}
			initUserFromForm(c, user)
		}
		if invID != "" {
			if inv, err := uc.userManager.GetInvitation(invID); err == nil {
				user.OrganizationID = inv.OrganizationID
				user.InvitationCode = inv.InvitationCode
				user.OrgUnit = inv.OrgUnit
			}
		}
		logrus.WithFields(logrus.Fields{
			"Component":  "UserAdminController",
			"User":       user,
			"Invitation": user.InvitationCode,
			"Error":      err,
		}).Errorf("failed to save user")
		isOrgSignup := c.FormValue("accountType") == "organization"
		res := map[string]interface{}{
			"User":         user,
			"InvitationID": invID,
			"IsOrgSignup":  isOrgSignup,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "users/new", res)
	}
	// Issue a fresh JWT for the newly created user so they land directly in the dashboard.
	token, expiration, tokenErr := security.BuildToken(
		user,
		uc.commonCfg.Auth.JWTSecret,
		uc.commonCfg.Auth.MaxAge)
	if tokenErr == nil {
		c.SetCookie(uc.commonCfg.Auth.SessionCookie(token, expiration))
	}
	return c.Redirect(http.StatusFound, "/dashboard")
}

// getUser - finds user by id
func (uc *UserAdminController) getUser(c web.APIContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.GetUser(qc, id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"User": user,
	}
	tokens, err := uc.userManager.GetUserTokens(qc, id)
	if err == nil {
		res["Tokens"] = tokens
	}

	ranges := manager.BuildRanges(time.Now(), 1, 1, 1, false)
	res["TodayRange"] = ranges[0].StartString()
	res["WeekRange"] = ranges[1].StartAndEndString()
	res["MonthRange"] = ranges[2].StartAndEndString()
	res["PolicyRange"] = ""
	res["HasPolicyRange"] = false
	if user.Subscription != nil && !user.Subscription.Expired() {
		ranges[2] = types.DateRange{StartDate: user.Subscription.StartedAt, EndDate: user.Subscription.EndedAt}
		res["PolicyRange"] = ranges[2].StartAndEndString()
		res["HasPolicyRange"] = true
	}
	resources := make([]map[string]interface{}, 0)
	userQC := qc.WithOrganizationIDColumn("")
	if qc.IsAdmin() {
		userQC = common.NewQueryContextFromIDs(id, "")
	}
	if cpuUsage, err := uc.jobExecRepository.GetResourceUsage(
		userQC, ranges); err == nil {
		m := map[string]interface{}{
			"Kind":  "CPU",
			"Today": cpuUsage[0],
			"Week":  cpuUsage[1],
		}
		if user.Subscription != nil && !user.Subscription.Expired() {
			m["Subscription"] = cpuUsage[2]
			if cpuUsage[2].Value <= user.Subscription.CPUQuota {
				user.Subscription.RemainingCPUQuota = user.Subscription.CPUQuota - cpuUsage[2].Value
			}
		} else {
			m["Month"] = cpuUsage[2]
		}
		resources = append(resources, m)
	}
	if storageUsage, err := uc.artifactRepository.GetResourceUsage(
		userQC, ranges); err == nil {
		m := map[string]interface{}{
			"Kind":  "Storage",
			"Today": storageUsage[0],
			"Week":  storageUsage[1],
		}
		if user.Subscription != nil && !user.Subscription.Expired() {
			m["Subscription"] = storageUsage[2]
			if storageUsage[2].MValue() <= user.Subscription.DiskQuota {
				user.Subscription.RemainingDiskQuota = user.Subscription.DiskQuota - storageUsage[2].MValue()
			} else {
				m["Month"] = storageUsage[2]
			}
		}
		resources = append(resources, m)
	}
	res["ResourcesUsage"] = resources
	if user.Subscription != nil {
		res["Subscription"] = user.Subscription
	}
	unverifiedEmails := user.GetUnverifiedNotificationEmails()
	if len(unverifiedEmails) > 0 {
		verifiedEmails := uc.userManager.GetVerifiedEmails(qc, user)
		newUnverifiedEmails := make([]string, 0)
		for _, email := range unverifiedEmails {
			if !verifiedEmails[email] {
				newUnverifiedEmails = append(newUnverifiedEmails, email)
			}
		}
		unverifiedEmails = newUnverifiedEmails
	}
	res["UnverifiedEmails"] = unverifiedEmails
	if user.HasOrganization() {
		res["SlackToken"], _ = uc.userManager.GetSlackToken(qc, user.Organization)
	}

	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/view", res)
}

// editUser - shows user for edit
func (uc *UserAdminController) editUser(c web.APIContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.GetUser(qc, id)
	if err != nil {
		user = common.NewUser("", "", "", "", acl.NewRoles(""))
		user.Errors = map[string]string{"Error": err.Error()}
	}
	res := map[string]interface{}{
		"User": user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/edit", res)
}

// TODO add email verification if email is different than user email
func (uc *UserAdminController) updateUserNotification(c web.APIContext) (err error) {
	id := c.Param("id")
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
		if user == nil {
			user = &common.User{ID: id}
		}
		user.Errors = map[string]string{"Notify": err.Error()}
		res := map[string]interface{}{
			"User":   user,
			"Notify": err,
		}
		web.RenderDBUserFromSession(c, res)

		return c.Render(http.StatusOK, "users/view", res)
	}

	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/%s", user.ID))
}

// updateUser - updates user
func (uc *UserAdminController) updateUser(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := common.NewUser(
		qc.GetOrganizationID(),
		qc.GetUsername(),
		c.FormValue("name"),
		c.FormValue("email"),
		acl.NewRoles(""))
	user.ID = c.Param("id")
	err = user.Validate()

	if err == nil {
		user, err = uc.userManager.UpdateUser(qc, user)
	}
	if err != nil {
		if u, err := uc.userManager.GetUser(qc, c.Param("id")); err == nil {
			if user != nil {
				u.Errors = user.Errors
			}
			u.Name = c.FormValue("name")
			user = u
		}
		if user != nil {
			user.Errors = map[string]string{"Error": err.Error()}
		}
		res := map[string]interface{}{
			"User":  user,
			"Error": err,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "users/edit", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/%s", user.ID))
}

// deleteUser - deletes user by id
func (uc *UserAdminController) deleteUser(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.DeleteUser(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/users")
}

// newUserToken new token
func (uc *UserAdminController) newUserToken(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	tok := types.NewUserToken(
		qc.User,
		"")
	res := map[string]interface{}{
		"Token": tok,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/new_token", res)
}

// deleteUserToken - deletes user token by id
func (uc *UserAdminController) deleteUserToken(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.RevokeUserToken(
		qc,
		qc.GetUserID(),
		c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/users/"+qc.GetUserID())
}

// createUserToken - saves a new token
func (uc *UserAdminController) createUserToken(c web.APIContext) (err error) {
	qc := web.BuildQueryContext(c)
	tok, err := uc.userManager.CreateUserToken(
		qc,
		c.FormValue("token"))
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Token": tok,
	}
	web.RenderDBUserFromSession(c, res)
	if err != nil {
		return c.Render(http.StatusOK, "users/new_token", res)
	}
	return c.Render(http.StatusOK, "users/view_token", res)
}

func (uc *UserAdminController) createUserFromForm(c web.APIContext) (*common.User, error) {
	// If the user is already in the DB, signup completed — just return them so
	// the caller can issue a fresh JWT and redirect to the dashboard.
	if dbUser := web.GetDBUserFromSession(c); dbUser != nil {
		return dbUser, nil
	}
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return nil, fmt.Errorf("failed to find user in session and form")
	}
	initUserFromForm(c, user)
	if err := user.Validate(); err != nil {
		return user, err
	}
	if !user.AgreeTerms {
		user.Errors = map[string]string{"AgreeTerms": "you must agree to the terms of service."}
		return user, fmt.Errorf("you must agree to the terms of service")
	}
	qc := common.NewQueryContext(nil, "").WithAdmin()
	saved, _, err := uc.userManager.SignupUser(qc, user, c.Request().RemoteAddr)
	return saved, err
}

func initUserFromForm(c web.APIContext, user *common.User) {
	user.Name = c.FormValue("name")
	user.Email = c.FormValue("email")
	user.AgreeTerms = c.FormValue("agreeTerms") == "agree"
	// Only populate org fields when the user explicitly chose an organization account.
	// This ensures individual-account submissions never accidentally trigger org creation.
	accountType := c.FormValue("accountType")
	logrus.WithFields(logrus.Fields{
		"Component":   "UserAdminController",
		"Username":    user.Username,
		"AccountType": accountType,
		"OrgUnit_raw": c.FormValue("orgUnit"),
		"Bundle_raw":  c.FormValue("bundle"),
		"InvCode_raw": c.FormValue("invitationCode"),
	}).Infof("initUserFromForm")
	// OrganizationID must always be cleared for a new signup — the session user may
	// carry a stale OrganizationID from a personal org created in a prior attempt,
	// which would cause BuildOrgWithInvitation to take the org-creation path.
	user.OrganizationID = ""
	if accountType == "organization" {
		user.BundleID = c.FormValue("bundle")
		user.OrgUnit = c.FormValue("orgUnit")
		user.InvitationCode = c.FormValue("invitationCode")
	} else {
		user.BundleID = ""
		user.OrgUnit = ""
		user.InvitationCode = ""
	}
}

