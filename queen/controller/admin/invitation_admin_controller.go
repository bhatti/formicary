package admin

import (
	"fmt"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/queen/manager"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/types"
)

// InvitationAdminController structure
type InvitationAdminController struct {
	userManager *manager.UserManager
	webserver   web.Server
}

// NewInvitationAdminController admin dashboard for managing orgs
func NewInvitationAdminController(
	userManager *manager.UserManager,
	webserver web.Server) *InvitationAdminController {
	jraCtr := &InvitationAdminController{
		userManager: userManager,
		webserver:   webserver,
	}
	webserver.GET("/dashboard/orgs/invite/:org", jraCtr.newInvite, acl.New(acl.UserInvitation, acl.Read)).Name = "new_admin_new_invite_user"
	webserver.POST("/dashboard/orgs/invite/:org", jraCtr.invite, acl.New(acl.UserInvitation, acl.Update)).Name = "new_admin_invite_user"
	webserver.GET("/dashboard/orgs/invited/:id", jraCtr.invited, acl.New(acl.UserInvitation, acl.Read)).Name = "new_admin_invited_user"
	webserver.GET("/dashboard/orgs/invitations", jraCtr.invitations, acl.New(acl.UserInvitation, acl.Query)).Name = "new_admin_invitations"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************

// newInvite - invites to org
func (oc *InvitationAdminController) newInvite(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session")
	}
	id := c.Param("org")
	orgID := user.OrganizationID
	if user.Admin && id != "" {
		org, err := oc.userManager.GetOrganization(qc, id)
		if err != nil {
			return err
		}
		orgID = org.ID
	}
	if orgID == "" {
		logrus.WithFields(logrus.Fields{
			"Component": "InvitationAdminController",
			"Admin":     user.Admin,
			"Org":       id,
			"User":      user,
		}).Warnf("no orgs for invitation")
		return fmt.Errorf("organization is not available for invitation")
	}

	inv := &types.UserInvitation{}
	inv.InvitedByUserID = user.ID
	inv.OrganizationID = orgID

	res := map[string]interface{}{
		"Invitation": inv,
		"User":       user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/inv/new", res)
}

// invite - adds invitation
func (oc *InvitationAdminController) invite(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session for invitation")
	}
	id := c.Param("org")
	orgID := user.OrganizationID
	if user.Admin && id != "" {
		org, err := oc.userManager.GetOrganization(qc, id)
		if err != nil {
			return err
		}
		orgID = org.ID
	}
	if orgID == "" {
		return fmt.Errorf("organization is not available for invitation")
	}
	inv := &types.UserInvitation{}
	inv.Email = c.FormValue("email")
	inv.InvitedByUserID = user.ID
	inv.OrganizationID = orgID
	if err = oc.userManager.InviteUser(qc, user, inv); err != nil {
		res := map[string]interface{}{
			"Invitation": inv,
			"User":       user,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/inv/new", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/invited/%s", inv.ID))
}

// invited - show invitation
func (oc *InvitationAdminController) invited(c web.WebContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session for invited")
	}
	id := c.Param("id")
	inv, err := oc.userManager.GetInvitation(id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Invitation": inv,
		"User":       user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/inv/view", res)
}

// invitations - queries org invitations
func (oc *InvitationAdminController) invitations(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	recs, total, err := oc.userManager.QueryInvitations(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/orgs/invitations?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/inv/index", res)
}
