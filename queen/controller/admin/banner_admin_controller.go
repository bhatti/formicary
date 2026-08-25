// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
)

// BannerAdminController manages persistent dashboard banners.
type BannerAdminController struct {
	bannerRepo repository.BannerRepository
	webserver  web.Server
}

// NewBannerAdminController registers banner CRUD routes.
func NewBannerAdminController(bannerRepo repository.BannerRepository, webserver web.Server) *BannerAdminController {
	c := &BannerAdminController{
		bannerRepo: bannerRepo,
		webserver:  webserver,
	}
	webserver.GET("/dashboard/banners", c.index, acl.NewPermission(acl.SystemConfig, acl.Query)).Name = "admin_banners_index"
	webserver.GET("/dashboard/banners/new", c.newForm, acl.NewPermission(acl.SystemConfig, acl.Create)).Name = "admin_banners_new"
	webserver.POST("/dashboard/banners", c.create, acl.NewPermission(acl.SystemConfig, acl.Create)).Name = "admin_banners_create"
	webserver.GET("/dashboard/banners/:id/edit", c.editForm, acl.NewPermission(acl.SystemConfig, acl.Update)).Name = "admin_banners_edit"
	webserver.POST("/dashboard/banners/:id", c.update, acl.NewPermission(acl.SystemConfig, acl.Update)).Name = "admin_banners_update"
	webserver.POST("/dashboard/banners/:id/delete", c.delete, acl.NewPermission(acl.SystemConfig, acl.Delete)).Name = "admin_banners_delete"
	return c
}

func (c *BannerAdminController) index(ctx web.APIContext) error {
	params, _, page, pageSize, q, qs := controller.ParseParams(ctx)
	banners, total, err := c.bannerRepo.Query(params, page, pageSize)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/banners?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Banners":    banners,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "banners/index", res)
}

func (c *BannerAdminController) newForm(ctx web.APIContext) error {
	banner := &common.Banner{
		Level:  common.BannerLevelWarning,
		Scope:  common.BannerScopeGlobal,
		Source: common.BannerSourceAdmin,
		Active: true,
	}
	res := map[string]interface{}{"Banner": banner}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "banners/new", res)
}

func (c *BannerAdminController) create(ctx web.APIContext) error {
	banner := buildBanner(ctx)
	if err := c.bannerRepo.Save(banner); err != nil {
		res := map[string]interface{}{"Banner": banner, "Error": err.Error()}
		web.RenderDBUserFromSession(ctx, res)
		return ctx.Render(http.StatusOK, "banners/new", res)
	}
	return ctx.Redirect(http.StatusFound, "/dashboard/banners")
}

func (c *BannerAdminController) editForm(ctx web.APIContext) error {
	id := ctx.Param("id")
	banners, _, err := c.bannerRepo.Query(map[string]interface{}{}, 0, 1000)
	var banner *common.Banner
	if err == nil {
		for _, b := range banners {
			if b.ID == id {
				banner = b
				break
			}
		}
	}
	if banner == nil {
		banner = &common.Banner{ID: id, Level: common.BannerLevelWarning, Scope: common.BannerScopeGlobal, Source: common.BannerSourceAdmin}
	}
	res := map[string]interface{}{"Banner": banner}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "banners/edit", res)
}

func (c *BannerAdminController) update(ctx web.APIContext) error {
	banner := buildBanner(ctx)
	banner.ID = ctx.Param("id")
	if err := c.bannerRepo.Save(banner); err != nil {
		res := map[string]interface{}{"Banner": banner, "Error": err.Error()}
		web.RenderDBUserFromSession(ctx, res)
		return ctx.Render(http.StatusOK, "banners/edit", res)
	}
	return ctx.Redirect(http.StatusFound, "/dashboard/banners")
}

func (c *BannerAdminController) delete(ctx web.APIContext) error {
	if err := c.bannerRepo.Delete(ctx.Param("id")); err != nil {
		return err
	}
	return ctx.Redirect(http.StatusFound, "/dashboard/banners")
}

func buildBanner(ctx web.APIContext) *common.Banner {
	active := ctx.FormValue("active") == "on" || ctx.FormValue("active") == "true"
	activeStr := ctx.FormValue("active")
	if b, err := strconv.ParseBool(activeStr); err == nil {
		active = b
	}
	return &common.Banner{
		Level:   ctx.FormValue("level"),
		Scope:   ctx.FormValue("scope"),
		OrgID:   ctx.FormValue("org_id"),
		Source:  common.BannerSourceAdmin,
		Message: ctx.FormValue("message"),
		Active:  active,
	}
}
