package admin

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// OrganizationConfigAdminController structure
type OrganizationConfigAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	orgConfigRepository   repository.OrganizationConfigRepository
	webserver             web.Server
}

// NewOrganizationConfigAdminController admin dashboard for managing org-configs
func NewOrganizationConfigAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	orgConfigRepository repository.OrganizationConfigRepository,
	webserver web.Server) *OrganizationConfigAdminController {
	jraCtr := &OrganizationConfigAdminController{
		auditRecordRepository: auditRecordRepository,
		orgConfigRepository:   orgConfigRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/orgs/:org/configs", jraCtr.queryOrganizationConfigs, acl.NewPermission(acl.Organization, acl.View)).Name = "query_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/new", jraCtr.newOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "new_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs", jraCtr.createOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "create_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs/:id", jraCtr.updateOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "update_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/:id", jraCtr.getOrganizationConfig, acl.NewPermission(acl.Organization, acl.View)).Name = "get_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/:id/edit", jraCtr.editOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "edit_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs/:id/delete", jraCtr.deleteOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "delete_admin_org_configs"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryOrganizationConfigs - queries org-config
func (jraCtr *OrganizationConfigAdminController) queryOrganizationConfigs(c web.WebContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	qc := web.BuildQueryContext(c)
	configs, total, err := jraCtr.orgConfigRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/orgs/%s/configs?%s", qc.GetOrganizationID(), q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Configs":    configs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/configs/index", res)
}

// createOrganizationConfig - saves a new org-config
func (jraCtr *OrganizationConfigAdminController) createOrganizationConfig(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	//orgID := c.Param("org")
	config, err := common.NewOrganizationConfig(
		qc.GetOrganizationID(),
		c.FormValue("name"),
		c.FormValue("value"),
		c.FormValue("secret") == "on")
	if err == nil {
		err = config.Validate()
		if err == nil {
			config, err = jraCtr.orgConfigRepository.Save(qc, config)
		}
	}
	if err != nil {
		res := map[string]interface{}{
			"Config": config,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/configs/new", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromOrganizationConfig(config, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs/%s", qc.GetOrganizationID(), config.ID))
}

// updateOrganizationConfig - updates org-config
func (jraCtr *OrganizationConfigAdminController) updateOrganizationConfig(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	//orgID := c.Param("org")
	config, err := common.NewOrganizationConfig(
		qc.GetOrganizationID(),
		c.FormValue("name"),
		c.FormValue("value"),
		c.FormValue("secret") == "on")
	if err == nil {
		err = config.Validate()
		if err == nil {
			config.ID = c.Param("id")
			config, err = jraCtr.orgConfigRepository.Save(qc, config)
		}
	}

	if err != nil {
		res := map[string]interface{}{
			"Config": config,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "orgs/configs/edit", res)
	}
	_, _ = jraCtr.auditRecordRepository.Save(types.NewAuditRecordFromOrganizationConfig(config, qc))
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs/%s", qc.GetOrganizationID(), config.ID))
}

// newOrganizationConfig - creates a new org config
func (jraCtr *OrganizationConfigAdminController) newOrganizationConfig(c web.WebContext) error {
	config, err := common.NewOrganizationConfig("", "", "", false)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/configs/new", res)
}

// getOrganizationConfig - finds org-config by id
func (jraCtr *OrganizationConfigAdminController) getOrganizationConfig(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	config, err := jraCtr.orgConfigRepository.Get(qc, id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/configs/view", res)
}

// editOrganizationConfig - shows org-config for edit
func (jraCtr *OrganizationConfigAdminController) editOrganizationConfig(c web.WebContext) error {
	id := c.Param("id")
	qc := web.BuildQueryContext(c)
	config, err := jraCtr.orgConfigRepository.Get(qc, id)
	if err != nil {
		config = &common.OrganizationConfig{}
		config.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "OrganizationConfigAdminController",
				"Error":     err,
				"ID":        id,
			}).Debug("failed to find config")
		}
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "orgs/configs/edit", res)
}

// deleteOrganizationConfig - deletes org-config by id
func (jraCtr *OrganizationConfigAdminController) deleteOrganizationConfig(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jraCtr.orgConfigRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs", qc.GetOrganizationID()))
}
