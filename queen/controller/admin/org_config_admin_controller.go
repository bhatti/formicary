// SPDX-License-Identifier: AGPL-3.0-or-later

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

// OrganizationConfigAdminController renders org config dashboard pages.
type OrganizationConfigAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	configRepository      repository.ConfigRepository
	webserver             web.Server
}

// NewOrganizationConfigAdminController registers dashboard routes for org configs.
func NewOrganizationConfigAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	configRepository repository.ConfigRepository,
	webserver web.Server) *OrganizationConfigAdminController {
	c := &OrganizationConfigAdminController{
		auditRecordRepository: auditRecordRepository,
		configRepository:      configRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/orgs/:org/configs", c.queryOrganizationConfigs, acl.NewPermission(acl.OrgConfig, acl.View)).Name = "query_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/new", c.newOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Update)).Name = "new_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs", c.createOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Create)).Name = "create_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs/:id", c.updateOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Update)).Name = "update_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/:id", c.getOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.View)).Name = "get_admin_org_configs"
	webserver.GET("/dashboard/orgs/:org/configs/:id/edit", c.editOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Update)).Name = "edit_admin_org_configs"
	webserver.POST("/dashboard/orgs/:org/configs/:id/delete", c.deleteOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Delete)).Name = "delete_admin_org_configs"
	return c
}

func (c *OrganizationConfigAdminController) queryOrganizationConfigs(ctx web.APIContext) error {
	_, _, page, pageSize, q, qs := controller.ParseParams(ctx)
	qc := web.BuildQueryContext(ctx)
	orgID := qc.GetOrganizationID()
	configs, total, err := c.configRepository.QueryOrgConfigs(
		common.NewQueryContextFromIDs("", orgID), orgID, page, pageSize)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/orgs/%s/configs?%s", orgID, q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Configs":    configs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "orgs/configs/index", res)
}

func (c *OrganizationConfigAdminController) createOrganizationConfig(ctx web.APIContext) (err error) {
	qc := web.BuildQueryContext(ctx)
	cfg, err := common.NewOrgConfig(
		qc.GetOrganizationID(),
		ctx.FormValue("name"),
		ctx.FormValue("value"),
		ctx.FormValue("secret") == "on")
	if err == nil {
		err = cfg.Validate()
		if err == nil {
			cfg, err = c.configRepository.Save(qc, cfg)
		}
	}
	if err != nil {
		res := map[string]interface{}{"Config": cfg}
		web.RenderDBUserFromSession(ctx, res)
		return ctx.Render(http.StatusOK, "orgs/configs/new", res)
	}
	_, _ = c.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return ctx.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs/%s", qc.GetOrganizationID(), cfg.ID))
}

func (c *OrganizationConfigAdminController) updateOrganizationConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	secret := ctx.FormValue("secret") == "on"
	value := ctx.FormValue("value")
	cfg, err := common.NewOrgConfig(qc.GetOrganizationID(), ctx.FormValue("name"), value, secret)
	if err == nil {
		cfg.ID = ctx.Param("id")
		// Preserve the stored encrypted value when the form shows the masked placeholder.
		if secret && value == "****" {
			var existing *common.Config
			existing, err = c.configRepository.Get(qc, cfg.ID)
			if err == nil {
				cfg.Value = existing.Value
			}
		}
		if err == nil {
			err = cfg.Validate()
		}
		if err == nil {
			cfg, err = c.configRepository.Save(qc, cfg)
		}
	}
	if err != nil {
		res := map[string]interface{}{"Config": cfg}
		web.RenderDBUserFromSession(ctx, res)
		return ctx.Render(http.StatusOK, "orgs/configs/edit", res)
	}
	_, _ = c.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return ctx.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs/%s", qc.GetOrganizationID(), cfg.ID))
}

func (c *OrganizationConfigAdminController) newOrganizationConfig(ctx web.APIContext) error {
	cfg, err := common.NewOrgConfig("", "", "", false)
	if err != nil {
		return err
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "orgs/configs/new", res)
}

func (c *OrganizationConfigAdminController) getOrganizationConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	cfg, err := c.configRepository.Get(qc, ctx.Param("id"))
	if err != nil {
		return err
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "orgs/configs/view", res)
}

func (c *OrganizationConfigAdminController) editOrganizationConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	cfg, err := c.configRepository.Get(qc, ctx.Param("id"))
	if err != nil {
		cfg = &common.Config{}
		cfg.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "OrganizationConfigAdminController",
				"Error":     err,
				"ID":        ctx.Param("id"),
			}).Debug("failed to find config")
		}
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "orgs/configs/edit", res)
}

func (c *OrganizationConfigAdminController) deleteOrganizationConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	if err := c.configRepository.Delete(qc, ctx.Param("id")); err != nil {
		return err
	}
	return ctx.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/orgs/%s/configs", qc.GetOrganizationID()))
}
