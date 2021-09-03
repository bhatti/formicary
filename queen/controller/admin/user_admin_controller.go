package admin

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"strings"
	"time"
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
	webserver.GET("/dashboard/users", jraCtr.queryUsers, acl.New(acl.User, acl.Query)).Name = "query_admin_users"
	webserver.GET("/dashboard/users/new", jraCtr.newUser, acl.New(acl.User, acl.Signup)).Name = "new_admin_users"
	webserver.POST("/dashboard/users", jraCtr.createUser, acl.New(acl.User, acl.Signup)).Name = "create_admin_users"
	webserver.POST("/dashboard/users/:id", jraCtr.updateUser, acl.New(acl.User, acl.Update)).Name = "update_admin_users"
	webserver.POST("/dashboard/users/:id/notify", jraCtr.updateUserNotification, acl.New(acl.User, acl.Update)).Name = "update_admin_users_notify"
	webserver.GET("/dashboard/users/:id", jraCtr.getUser, acl.New(acl.User, acl.View)).Name = "get_admin_users"
	webserver.GET("/dashboard/users/:id/edit", jraCtr.editUser, acl.New(acl.User, acl.Update)).Name = "edit_admin_users"
	webserver.POST("/dashboard/users/:id/delete", jraCtr.deleteUser, acl.New(acl.User, acl.Delete)).Name = "delete_admin_users"
	webserver.GET("/dashboard/users/:user/tokens/new", jraCtr.newUserToken, acl.New(acl.User, acl.Update)).Name = "new_admin_user_token"
	webserver.POST("/dashboard/users/:user/tokens", jraCtr.createUserToken, acl.New(acl.User, acl.Update)).Name = "create_admin_user_token"
	webserver.POST("/dashboard/users/:user/tokens/:id/delete", jraCtr.deleteUserToken, acl.New(acl.User, acl.Update)).Name = "delete_admin_user_token"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryUsers - queries user
func (uc *UserAdminController) queryUsers(c web.WebContext) error {
	params, order, page, pageSize, q := controller.ParseParams(c)
	qc := web.BuildQueryContext(c)
	users, total, err := uc.userManager.QueryUsers(
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
	res := map[string]interface{}{"Users": users,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          params["q"],
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/index", res)
}

// newUser - creates a new user
func (uc *UserAdminController) newUser(c web.WebContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session to create user")
	}
	res := map[string]interface{}{
		"User": user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/new", res)
}

// createUser - saves a new user
func (uc *UserAdminController) createUser(c web.WebContext) (err error) {
	user, err := uc.createUserFromForm(c)
	if err != nil {
		if user == nil {
			user = common.NewUser("", "", "", false)
			user.Errors = map[string]string{"Error": err.Error()}
			initUserFromForm(c, user)
		}
		logrus.WithFields(logrus.Fields{
			"Component": "UserAdminController",
			"User":      user,
			"Error":     err,
		}).Errorf("failed to save user")
		res := map[string]interface{}{
			"User": user,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "users/new", res)
	}
	//return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/%s", user.ID))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard"))
}

// getUser - finds user by id
func (uc *UserAdminController) getUser(c web.WebContext) error {
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
	if qc.Admin() {
		userQC = common.NewQueryContext(id, "", "")
	}
	if cpuUsage, err := uc.jobExecRepository.GetResourceUsage(
		userQC, ranges); err == nil {
		m := map[string]interface{}{
			"Type":  "CPU",
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
			"Type":  "Storage",
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
		verifiedEmails := uc.userManager.GetVerifiedEmails(qc, user.ID)
		newUnverifiedEmails := make([]string, 0)
		for _, email := range unverifiedEmails {
			if !verifiedEmails[email] {
				newUnverifiedEmails = append(newUnverifiedEmails, email)
			}
		}
		unverifiedEmails = newUnverifiedEmails
	}
	res["UnverifiedEmails"] = unverifiedEmails

	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/view", res)
}

// editUser - shows user for edit
func (uc *UserAdminController) editUser(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.GetUser(qc, id)
	if err != nil {
		user = common.NewUser("", "", "", false)
		user.Errors = map[string]string{"Error": err.Error()}
	}
	res := map[string]interface{}{
		"User": user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/edit", res)
}

// TODO add email verification if email is different than user email
func (uc *UserAdminController) updateUserNotification(c web.WebContext) (err error) {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	user, err := uc.userManager.UpdateUserNotification(
		qc,
		c.Param("id"),
		c.FormValue("email"),
		c.FormValue("slackChannel"),
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
func (uc *UserAdminController) updateUser(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := common.NewUser(
		qc.OrganizationID,
		"",
		c.FormValue("name"),
		false)
	user.ID = c.Param("id")
	user.Email = c.FormValue("email")
	user.Username = qc.Username
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
func (uc *UserAdminController) deleteUser(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.DeleteUser(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/users")
}

// newUserToken new token
func (uc *UserAdminController) newUserToken(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	tok := types.NewUserToken(qc.UserID, qc.OrganizationID, "")
	res := map[string]interface{}{
		"Token": tok,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "users/new_token", res)
}

// deleteUserToken - deletes user token by id
func (uc *UserAdminController) deleteUserToken(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := uc.userManager.RevokeUserToken(qc, qc.UserID, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/users/"+qc.UserID)
}

// createUserToken - saves a new token
func (uc *UserAdminController) createUserToken(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	tok, err := uc.userManager.CreateUserToken(qc, web.GetDBLoggedUserFromSession(c), c.FormValue("token"))
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

func (uc *UserAdminController) createUserFromForm(c web.WebContext) (saved *common.User, err error) {
	dbUser := web.GetDBUserFromSession(c)
	if dbUser != nil {
		return nil, fmt.Errorf("already exists user with username %s", dbUser.Username)
	}

	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return nil, fmt.Errorf("failed to find user in session and form")
	}

	initUserFromForm(c, user)

	if err = user.Validate(); err != nil {
		return user, err
	}
	if !user.AgreeTerms {
		user.Errors = map[string]string{"AgreeTerms": "you must agree to the terms of service."}
		return user, fmt.Errorf("you must agree to the terms of service")
	}

	var org *common.Organization
	org, err = uc.userManager.BuildOrgWithInvitation(user)
	if err != nil {
		return nil, err
	}

	saved, err = uc.saveNewUser(user)
	if err != nil {
		return user, err
	}

	logrus.WithFields(logrus.Fields{
		"Component": "UserAdminController",
		"User":      saved,
	}).Infof("created user")

	if org != nil {
		// new org
		var savedOrg *common.Organization
		if org.ID == "" {
			org.OwnerUserID = saved.ID
			savedOrg, err = uc.saveNewOrg(c, org)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "UserAdminController",
					"Org":       org,
					"Error":     err,
				}).Errorf("failed to save organization")
				// delete user as well
				adminQC := common.NewQueryContext("", "", "").WithAdmin()
				_ = uc.userManager.DeleteUser(adminQC, saved.ID)
				return nil, err
			}

			logrus.WithFields(logrus.Fields{
				"Component": "UserAdminController",
				"Org":       savedOrg,
				"OrgID":     savedOrg.ID,
			}).Infof("created organization")
		} else if saved.OrganizationID != org.ID {
			savedOrg = org
		}

		// update user with org
		if savedOrg != nil {
			saved.OrganizationID = savedOrg.ID
			saved.BundleID = savedOrg.BundleID
			org = savedOrg
			// disabling query context here
			adminQC := common.NewQueryContext(saved.ID, savedOrg.ID, "").WithAdmin()
			_, err = uc.userManager.UpdateUser(adminQC, saved)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "UserAdminController",
					"User":      saved,
					"Org":       savedOrg,
					"Error":     err,
				}).Errorf("failed to update organization in user")
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "UserAdminController",
					"User":      saved,
					"UserID":    saved.ID,
					"Org":       savedOrg,
					"OrgID":     savedOrg.ID,
				}).Infof("updated organization-id for user")
			}
		}
	}

	qc := common.NewQueryContextFromUser(saved, org, c.Request().RemoteAddr)
	_ = uc.userManager.PostSignup(qc, saved)
	return saved, nil
}

func initUserFromForm(c web.WebContext, user *common.User) {
	user.Name = c.FormValue("name")
	user.Email = c.FormValue("email")
	user.BundleID = c.FormValue("bundle")
	user.OrgUnit = c.FormValue("orgUnit")
	user.InvitationCode = c.FormValue("invitationCode")
	user.AgreeTerms = c.FormValue("agreeTerms") == "agree"
}

func (uc *UserAdminController) saveNewUser(user *common.User) (saved *common.User, err error) {
	qc := common.NewQueryContext("", "", "").WithAdmin()
	saved, err = uc.userManager.CreateUser(qc, user)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		saved, err = uc.userManager.UpdateUser(qc, user)
	}
	return
}

func (uc *UserAdminController) saveNewOrg(
	_ web.WebContext,
	org *common.Organization) (saved *common.Organization, err error) {
	//qc := web.BuildQueryContext(c)
	qc := common.NewQueryContext("", "", "").WithAdmin()
	saved, err = uc.userManager.CreateOrg(qc, org)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		saved, err = uc.userManager.UpdateOrg(qc, org)
	}
	return
}
