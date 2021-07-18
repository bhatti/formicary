package admin

import (
	"fmt"
	"net/http"

	"plexobject.com/formicary/internal/acl"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// SystemConfigAdminController structure
type SystemConfigAdminController struct {
	systemConfigRepository repository.SystemConfigRepository
	webserver              web.Server
}

// NewSystemConfigAdminController admin dashboard for managing system-configs -- only admin
func NewSystemConfigAdminController(
	repo repository.SystemConfigRepository,
	webserver web.Server) *SystemConfigAdminController {
	jraCtr := &SystemConfigAdminController{
		systemConfigRepository: repo,
		webserver:              webserver,
	}
	webserver.GET("/dashboard/configs", jraCtr.querySystemConfigs, acl.New(acl.SystemConfig, acl.Query)).Name = "query_admin_system_configs"
	webserver.GET("/dashboard/configs/new", jraCtr.newSystemConfig, acl.New(acl.SystemConfig, acl.Create)).Name = "new_admin_system_configs"
	webserver.POST("/dashboard/configs", jraCtr.createSystemConfig, acl.New(acl.SystemConfig, acl.Create)).Name = "create_admin_system_configs"
	webserver.POST("/dashboard/configs/:id", jraCtr.updateSystemConfig, acl.New(acl.SystemConfig, acl.Update)).Name = "update_admin_system_configs"
	webserver.GET("/dashboard/configs/:id", jraCtr.getSystemConfig, acl.New(acl.SystemConfig, acl.View)).Name = "get_admin_system_configs"
	webserver.GET("/dashboard/configs/:id/edit", jraCtr.editSystemConfig, acl.New(acl.SystemConfig, acl.Update)).Name = "edit_admin_system_configs"
	webserver.POST("/dashboard/configs/:id/delete", jraCtr.deleteSystemConfig, acl.New(acl.SystemConfig, acl.Delete)).Name = "delete_admin_system_configs"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// querySystemConfigs - queries system-config
func (jraCtr *SystemConfigAdminController) querySystemConfigs(c web.WebContext) error {
	params, order, page, pageSize, q := controller.ParseParams(c)
	configs, total, err := jraCtr.systemConfigRepository.Query(params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/configs?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{"Configs": configs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "configs/index", res)
}

// createSystemConfig - saves a new system-config
func (jraCtr *SystemConfigAdminController) createSystemConfig(c web.WebContext) (err error) {
	config := buildConfig(c)
	err = config.Validate()
	if err == nil {
		config, err = jraCtr.systemConfigRepository.Save(config)
	}
	if err != nil {
		res := map[string]interface{}{
			"Config": config,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "configs/new", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/configs/%s", config.ID))
}

// updateSystemConfig - updates system-config
func (jraCtr *SystemConfigAdminController) updateSystemConfig(c web.WebContext) (err error) {
	config := buildConfig(c)
	config.ID = c.Param("id")
	err = config.Validate()

	if err == nil {
		config, err = jraCtr.systemConfigRepository.Save(config)
	}
	if err != nil {
		res := map[string]interface{}{
			"Config": config,
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "configs/edit", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/configs/%s", config.ID))
}

// newSystemConfig - creates a new system config
func (jraCtr *SystemConfigAdminController) newSystemConfig(c web.WebContext) error {
	config := types.NewSystemConfig("", "", "", "")
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "configs/new", res)
}

// getSystemConfig - finds system-config by id
func (jraCtr *SystemConfigAdminController) getSystemConfig(c web.WebContext) error {
	id := c.Param("id")
	config, err := jraCtr.systemConfigRepository.Get(id)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "configs/view", res)
}

// editSystemConfig - shows system-config for edit
func (jraCtr *SystemConfigAdminController) editSystemConfig(c web.WebContext) error {
	id := c.Param("id")
	config, err := jraCtr.systemConfigRepository.Get(id)
	if err != nil {
		config = types.NewSystemConfig("", "", "", "")
		config.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "SystemConfigAdminController",
				"Error":     err,
				"ID":        id,
			}).Debug("failed to find config")
		}
	}
	res := map[string]interface{}{
		"Config": config,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "configs/edit", res)
}

// deleteSystemConfig - deletes system-config by id
func (jraCtr *SystemConfigAdminController) deleteSystemConfig(c web.WebContext) error {
	err := jraCtr.systemConfigRepository.Delete(c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/configs")
}

func buildConfig(c web.WebContext) *types.SystemConfig {
	config := types.NewSystemConfig(
		c.FormValue("scope"),
		c.FormValue("kind"),
		c.FormValue("name"),
		c.FormValue("value"))
	config.Secret = c.FormValue("secret") == "on"
	return config
}
