package admin

import (
	"fmt"
	"net/http"
	"plexobject.com/formicary/internal/utils"
	"time"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"

	"plexobject.com/formicary/internal/acl"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/types"
)

// OrganizationAdminController structure
type OrganizationAdminController struct {
	userManager *manager.UserManager
	webserver   web.Server
}

// NewOrganizationAdminController admin dashboard for managing orgs
func NewOrganizationAdminController(
	userManager *manager.UserManager,
	webserver web.Server) *OrganizationAdminController {
	ctr := &OrganizationAdminController{
		userManager: userManager,
		webserver:   webserver,
	}
	webserver.GET("/dashboard/orgs", ctr.queryOrganizations, acl.NewPermission(acl.Organization, acl.Query)).Name = "query_admin_orgs"
	webserver.GET("/dashboard/orgs/new", ctr.newOrganization, acl.NewPermission(acl.Organization, acl.Create)).Name = "new_admin_orgs"
	webserver.POST("/dashboard/orgs", ctr.createOrganization, acl.NewPermission(acl.Organization, acl.Create)).Name = "create_admin_orgs"
	webserver.POST("/dashboard/orgs/:id", ctr.updateOrganization, acl.NewPermission(acl.Organization, acl.Update)).Name = "update_admin_orgs"
	webserver.GET("/dashboard/orgs/:id", ctr.getOrganization, acl.NewPermission(acl.Organization, acl.View)).Name = "get_admin_orgs"
	webserver.GET("/dashboard/orgs/:id/edit", ctr.editOrganization, acl.NewPermission(acl.Organization, acl.Update)).Name = "edit_admin_orgs"
	webserver.POST("/dashboard/orgs/:id/delete", ctr.deleteOrganization, acl.NewPermission(acl.Organization, acl.Delete)).Name = "delete_admin_orgs"
	webserver.GET("/dashboard/orgs/usage_report", ctr.usageReport, acl.NewPermission(acl.Report, acl.View)).Name = "admin_usage_report"
	return ctr
}

// ********************************* HTTP Handlers ***********************************
// queryOrganizations - queries org
func (oc *OrganizationAdminController) queryOrganizations(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	recs, total, err := oc.userManager.QueryOrgs(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/orgs?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
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
		org, err = oc.userManager.CreateOrg(qc, org)
	}
	if err != nil {
		return c.Render(http.StatusOK, "orgs/new",
			map[string]interface{}{
				"Org": org,
			})
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s", org.ID))
}

// updateOrganization - updates org
func (oc *OrganizationAdminController) updateOrganization(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	org := buildOrganization(c)
	org.ID = c.Param("id")
	err = org.Validate()

	if err == nil {
		org, err = oc.userManager.UpdateOrg(qc, org)
	}
	if err != nil {
		res := map[string]interface{}{
			"Org": org,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/edit", res)
	}
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
	org, err := oc.userManager.GetOrganization(qc, id)
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
	if qc.IsAdmin() {
		orgQC = common.NewQueryContextFromIDs("", id)
	}
	if cpuUsage, err := oc.userManager.GetCPUResourceUsage(
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
	if storageUsage, err := oc.userManager.GetStorageResourceUsage(
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
	org, err := oc.userManager.GetOrganization(qc, id)
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
	err := oc.userManager.DeleteOrganization(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/orgs")
}

// usageReport -
func (oc *OrganizationAdminController) usageReport(c web.WebContext) error {
	from := utils.ParseStartDateTime(c.QueryParam("from"))
	to := utils.ParseEndDateTime(c.QueryParam("to"))

	combinedUsage := oc.userManager.CombinedResourcesByOrgUser(from, to, 10000)
	res := map[string]interface{}{
		"Records":  combinedUsage,
		"FromDate": from.Format("2006-01-02"),
		"ToDate":   to.Format("2006-01-02"),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/usage_report", res)
}

func buildOrganization(c web.WebContext) *common.Organization {
	qc := web.BuildQueryContext(c)
	org := common.NewOrganization(
		qc.GetUserID(),
		c.FormValue("orgUnit"),
		c.FormValue("orgBundle"),
	)
	org.OwnerUserID = qc.GetUserID()
	return org
}
