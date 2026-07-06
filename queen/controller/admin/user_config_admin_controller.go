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

// UserConfigAdminController renders per-user credential/config dashboard pages.
type UserConfigAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	configRepository      repository.ConfigRepository
	webserver             web.Server
}

// NewUserConfigAdminController registers dashboard routes for user configs.
func NewUserConfigAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	configRepository repository.ConfigRepository,
	webserver web.Server) *UserConfigAdminController {
	c := &UserConfigAdminController{
		auditRecordRepository: auditRecordRepository,
		configRepository:      configRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/users/configs", c.queryUserConfigs, acl.NewPermission(acl.UserConfig, acl.Query)).Name = "query_admin_user_configs"
	webserver.GET("/dashboard/users/configs/new", c.newUserConfig, acl.NewPermission(acl.UserConfig, acl.Create)).Name = "new_admin_user_configs"
	webserver.POST("/dashboard/users/configs", c.createUserConfig, acl.NewPermission(acl.UserConfig, acl.Create)).Name = "create_admin_user_configs"
	webserver.POST("/dashboard/users/configs/:id", c.updateUserConfig, acl.NewPermission(acl.UserConfig, acl.Update)).Name = "update_admin_user_configs"
	webserver.GET("/dashboard/users/configs/:id", c.getUserConfig, acl.NewPermission(acl.UserConfig, acl.View)).Name = "get_admin_user_configs"
	webserver.GET("/dashboard/users/configs/:id/edit", c.editUserConfig, acl.NewPermission(acl.UserConfig, acl.Update)).Name = "edit_admin_user_configs"
	webserver.POST("/dashboard/users/configs/:id/delete", c.deleteUserConfig, acl.NewPermission(acl.UserConfig, acl.Delete)).Name = "delete_admin_user_configs"
	return c
}

func (c *UserConfigAdminController) queryUserConfigs(ctx web.APIContext) error {
	_, _, page, pageSize, q, qs := controller.ParseParams(ctx)
	qc := web.BuildQueryContext(ctx)
	configs, total, err := c.configRepository.QueryUserConfigs(qc, qc.GetUserID(), page, pageSize)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/users/configs?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Configs":    configs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "users/configs/index", res)
}

func (c *UserConfigAdminController) createUserConfig(ctx web.APIContext) (err error) {
	qc := web.BuildQueryContext(ctx)
	cfg, err := common.NewUserConfig(
		qc.GetUserID(),
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
		return ctx.Render(http.StatusOK, "users/configs/new", res)
	}
	_, _ = c.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return ctx.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/configs/%s", cfg.ID))
}

func (c *UserConfigAdminController) updateUserConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	secret := ctx.FormValue("secret") == "on"
	value := ctx.FormValue("value")
	cfg, err := common.NewUserConfig(qc.GetUserID(), ctx.FormValue("name"), value, secret)
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
		return ctx.Render(http.StatusOK, "users/configs/edit", res)
	}
	_, _ = c.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return ctx.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/users/configs/%s", cfg.ID))
}

func (c *UserConfigAdminController) newUserConfig(ctx web.APIContext) error {
	cfg, err := common.NewUserConfig("", "", "", false)
	if err != nil {
		return err
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "users/configs/new", res)
}

func (c *UserConfigAdminController) getUserConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	cfg, err := c.configRepository.Get(qc, ctx.Param("id"))
	if err != nil {
		return err
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "users/configs/view", res)
}

func (c *UserConfigAdminController) editUserConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	cfg, err := c.configRepository.Get(qc, ctx.Param("id"))
	if err != nil {
		cfg = &common.Config{}
		cfg.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "UserConfigAdminController",
				"Error":     err,
				"ID":        ctx.Param("id"),
			}).Debug("failed to find user config")
		}
	}
	res := map[string]interface{}{"Config": cfg}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "users/configs/edit", res)
}

func (c *UserConfigAdminController) deleteUserConfig(ctx web.APIContext) error {
	qc := web.BuildQueryContext(ctx)
	if err := c.configRepository.Delete(qc, ctx.Param("id")); err != nil {
		return err
	}
	return ctx.Redirect(http.StatusFound, "/dashboard/users/configs")
}
