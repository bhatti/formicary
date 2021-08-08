package admin

import (
	"fmt"
	"net/http"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"time"

	"plexobject.com/formicary/internal/acl"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// OrganizationAdminController structure
type OrganizationAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	orgRepository         repository.OrganizationRepository
	jobExecRepository     repository.JobExecutionRepository
	artifactRepository    repository.ArtifactRepository
	webserver             web.Server
}

// NewOrganizationAdminController admin dashboard for managing orgs
func NewOrganizationAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	orgRepository repository.OrganizationRepository,
	jobExecRepository repository.JobExecutionRepository,
	artifactRepository repository.ArtifactRepository,
	webserver web.Server) *OrganizationAdminController {
	jraCtr := &OrganizationAdminController{
		auditRecordRepository: auditRecordRepository,
		orgRepository:         orgRepository,
		jobExecRepository:     jobExecRepository,
		artifactRepository:    artifactRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/orgs", jraCtr.queryOrganizations, acl.New(acl.Organization, acl.Query)).Name = "query_admin_orgs"
	webserver.GET("/dashboard/orgs/new", jraCtr.newOrganization, acl.New(acl.Organization, acl.Create)).Name = "new_admin_orgs"
	webserver.POST("/dashboard/orgs", jraCtr.createOrganization, acl.New(acl.Organization, acl.Create)).Name = "create_admin_orgs"
	webserver.POST("/dashboard/orgs/:id", jraCtr.updateOrganization, acl.New(acl.Organization, acl.Update)).Name = "update_admin_orgs"
	webserver.GET("/dashboard/orgs/:id", jraCtr.getOrganization, acl.New(acl.Organization, acl.View)).Name = "get_admin_orgs"
	webserver.GET("/dashboard/orgs/:id/edit", jraCtr.editOrganization, acl.New(acl.Organization, acl.Update)).Name = "edit_admin_orgs"
	webserver.POST("/dashboard/orgs/:id/delete", jraCtr.deleteOrganization, acl.New(acl.Organization, acl.Delete)).Name = "delete_admin_orgs"
	webserver.GET("/dashboard/orgs/invite/:org", jraCtr.newInvite, acl.New(acl.Organization, acl.Invite)).Name = "new_admin_new_invite_user"
	webserver.POST("/dashboard/orgs/invite/:org", jraCtr.invite, acl.New(acl.Organization, acl.Invite)).Name = "new_admin_invite_user"
	webserver.GET("/dashboard/orgs/invited/:id", jraCtr.invited, acl.New(acl.Organization, acl.Invite)).Name = "new_admin_invited_user"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryOrganizations - queries org
func (oc *OrganizationAdminController) queryOrganizations(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q := controller.ParseParams(c)
	orgs, total, err := oc.orgRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/orgs?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{"Orgs": orgs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          params["q"],
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/index", res)
}

// createOrganization - saves a new org
func (oc *OrganizationAdminController) createOrganization(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	org := buildOrganization(c)
	err = org.Validate()
	if err == nil {
		org, err = oc.orgRepository.Create(qc, org)
	}
	if err != nil {
		return c.Render(http.StatusOK, "orgs/new",
			map[string]interface{}{
				"Org": org,
			})
	}
	_, _ = oc.auditRecordRepository.Save(types.NewAuditRecordFromOrganization(org, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s", org.ID))
}

// updateOrganization - updates org
func (oc *OrganizationAdminController) updateOrganization(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	org := buildOrganization(c)
	org.ID = c.Param("id")
	err = org.Validate()

	if err == nil {
		org, err = oc.orgRepository.Update(qc, org)
	}
	if err != nil {
		res := map[string]interface{}{
			"Org": org,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/edit", res)
	}
	_, _ = oc.auditRecordRepository.Save(types.NewAuditRecordFromOrganization(org, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s", org.ID))
}

// newOrganization - creates a new org
func (oc *OrganizationAdminController) newOrganization(c web.WebContext) error {
	org := common.NewOrganization("", "", "")
	res := map[string]interface{}{
		"Org": org,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/new", res)
}

// getOrganization - finds org by id
func (oc *OrganizationAdminController) getOrganization(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	org, err := oc.orgRepository.Get(qc, id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Org": org,
	}

	ranges := manager.BuildRanges(time.Now(), 1, 1, 1, false)
	res["TodayRange"] = ranges[0].StartString()
	res["WeekRange"] = ranges[1].StartAndEndString()
	res["MonthRange"] = ranges[2].StartAndEndString()
	res["PolicyRange"] = ""
	res["HasPolicyRange"] = false
	if org.Subscription != nil {
		ranges[2] = types.DateRange{StartDate: org.Subscription.StartedAt, EndDate: org.Subscription.EndedAt}
		res["PolicyRange"] = ranges[2].StartAndEndString()
		res["HasPolicyRange"] = true
	}
	resources := make([]map[string]interface{}, 0)
	orgQC := qc
	if qc.Admin() {
		orgQC = common.NewQueryContext("", id, "")
	}
	if cpuUsage, err := oc.jobExecRepository.GetResourceUsage(
		orgQC, ranges); err == nil {
		m := map[string]interface{}{
			"Type":  "CPU",
			"Today": cpuUsage[0],
			"Week":  cpuUsage[1],
		}
		if org.Subscription != nil && !org.Subscription.Expired() {
			m["Subscription"] = cpuUsage[2]
			if cpuUsage[2].Value <= org.Subscription.CPUQuota {
				org.Subscription.RemainingCPUQuota = org.Subscription.CPUQuota - cpuUsage[2].Value
			}
		} else {
			m["Month"] = cpuUsage[2]
		}
		resources = append(resources, m)
	}
	if storageUsage, err := oc.artifactRepository.GetResourceUsage(
		orgQC, ranges); err == nil {
		m := map[string]interface{}{
			"Type":  "Storage",
			"Today": storageUsage[0],
			"Week":  storageUsage[1],
		}
		if org.Subscription != nil && !org.Subscription.Expired() {
			m["Subscription"] = storageUsage[2]
			if storageUsage[2].MValue() <= org.Subscription.DiskQuota {
				org.Subscription.RemainingDiskQuota = org.Subscription.DiskQuota - storageUsage[2].MValue()
			} else {
				m["Month"] = storageUsage[2]
			}
		}
		resources = append(resources, m)
	}
	res["ResourcesUsage"] = resources
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/view", res)
}

// editOrganization - shows org for edit
func (oc *OrganizationAdminController) editOrganization(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	org, err := oc.orgRepository.Get(qc, id)
	if err != nil {
		org = common.NewOrganization("", "", "")
		org.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "OrganizationAdminController",
				"Error":     err,
				"ID":        id,
			}).Debug("failed to find org")
		}
	}
	res := map[string]interface{}{
		"Org": org,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/edit", res)
}

// deleteOrganization - deletes org by id
func (oc *OrganizationAdminController) deleteOrganization(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := oc.orgRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/orgs")
}

// newInvite - invites to org
func (oc *OrganizationAdminController) newInvite(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session")
	}
	id := c.Param("org")
	orgID := user.OrganizationID
	if user.Admin && id != "" {
		org, err := oc.orgRepository.Get(qc, id)
		if err != nil {
			return err
		}
		orgID = org.ID
	}
	if orgID == "" {
		logrus.WithFields(logrus.Fields{
			"Component": "OrganizationAdminController",
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
	return c.Render(http.StatusOK, "orgs/invite", res)
}

// invite - adds invitation
func (oc *OrganizationAdminController) invite(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session for invitation")
	}
	id := c.Param("org")
	orgID := user.OrganizationID
	if user.Admin && id != "" {
		org, err := oc.orgRepository.Get(qc, id)
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
	if err = oc.orgRepository.AddInvitation(inv); err != nil {
		res := map[string]interface{}{
			"Invitation": inv,
			"User":       user,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/invite", res)
	}
	_, _ = oc.auditRecordRepository.Save(types.NewAuditRecordFromInvite(inv, qc))
	logrus.WithFields(logrus.Fields{
		"Component":  "OrganizationAdminController",
		"Admin":      user.Admin,
		"Org":        id,
		"User":       user,
		"Invitation": inv,
	}).Infof("user invited")
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/invited/%s", inv.ID))
}

// invited - show invitation
func (oc *OrganizationAdminController) invited(c web.WebContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return fmt.Errorf("failed to find user in session for invited")
	}
	id := c.Param("id")
	inv, err := oc.orgRepository.GetInvitation(id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Invitation": inv,
		"User":       user,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/invited", res)
}

func buildOrganization(c web.WebContext) *common.Organization {
	qc := web.BuildQueryContext(c)
	org := common.NewOrganization(
		qc.UserID,
		c.FormValue("orgUnit"),
		c.FormValue("orgBundle"),
	)
	org.OwnerUserID = qc.UserID
	return org
}
