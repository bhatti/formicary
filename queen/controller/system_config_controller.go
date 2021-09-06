package controller

import (
	"encoding/json"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"time"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// SystemConfigController structure
type SystemConfigController struct {
	systemConfigRepository repository.SystemConfigRepository
	webserver              web.Server
}

// NewSystemConfigController instantiates controller for updating system-configs
func NewSystemConfigController(
	repo repository.SystemConfigRepository,
	webserver web.Server) *SystemConfigController {
	cfgCtrl := &SystemConfigController{
		systemConfigRepository: repo,
		webserver:              webserver,
	}
	webserver.GET("/api/configs", cfgCtrl.querySystemConfigs, acl.New(acl.SystemConfig, acl.Query)).Name = "query_configs"
	webserver.GET("/api/configs/:id", cfgCtrl.getSystemConfig, acl.New(acl.SystemConfig, acl.View)).Name = "get_config"
	webserver.POST("/api/configs", cfgCtrl.postSystemConfig, acl.New(acl.SystemConfig, acl.Create)).Name = "create_config"
	webserver.PUT("/api/configs/:id", cfgCtrl.putSystemConfig, acl.New(acl.SystemConfig, acl.Update)).Name = "update_config"
	webserver.DELETE("/api/configs/:id", cfgCtrl.deleteSystemConfig, acl.New(acl.SystemConfig, acl.Delete)).Name = "delete_config"
	return cfgCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/configs system-configs querySystemConfigs
// Queries system configs
// `This requires admin access`
// responses:
//   200: sysConfigQueryResponse
func (cc *SystemConfigController) querySystemConfigs(c web.WebContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	recs, total, err := cc.systemConfigRepository.Query(params, page, pageSize, order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// swagger:route POST /api/configs system-configs postSystemConfig
// Creates new system config based on request body.
// `This requires admin access`
// responses:
//   200: sysConfigResponse
func (cc *SystemConfigController) postSystemConfig(c web.WebContext) error {
	now := time.Now()
	cfg := types.NewSystemConfig("", "", "", "")
	err := json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	saved, err := cc.systemConfigRepository.Save(cfg)
	if err != nil {
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	return c.JSON(status, saved)
}

// swagger:route PUT /api/configs/{id} system-configs putSystemConfig
// Updates an existing system config based on request body.
// `This requires admin access`
// responses:
//   200: sysConfigResponse
func (cc *SystemConfigController) putSystemConfig(c web.WebContext) error {
	cfg := types.NewSystemConfig("", "", "", "")
	err := json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	saved, err := cc.systemConfigRepository.Save(cfg)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/configs/{id} system-configs getSystemConfig
// Finds an existing system config based on id.
// `This requires admin access`
// responses:
//   200: sysConfigResponse
func (cc *SystemConfigController) getSystemConfig(c web.WebContext) error {
	cfg, err := cc.systemConfigRepository.Get(c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cfg)
}

// swagger:route DELETE /api/configs/{id} system-configs getSystemConfig
// Deletes an existing system config based on id.
// `This requires admin access`
// responses:
//   200: emptyResponse
func (cc *SystemConfigController) deleteSystemConfig(c web.WebContext) error {
	err := cc.systemConfigRepository.Delete(c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters querySystemConfigs
// The params for querying system-configs
type sysConfigsQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Scope defines scope such as default or org-unit
	Scope string `json:"scope"`
	// Kind defines kind of config property
	Kind string `json:"kind"`
	// Name defines name of config property
	Name string `json:"name"`
}

// Query results of system-configs
// swagger:response sysConfigQueryResponse
type sysConfigsQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*types.SystemConfig
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters postSystemConfig
// The params for system-config
type sysConfigCreateParams struct {
	// in:body
	Body types.SystemConfig
}

// swagger:parameters putSystemConfig
// The params for system-config
type sysConfigUpdateParams struct {
	// in:path
	ID string `json:"id"`
	// in:body
	Body types.SystemConfig
}

// SystemConfig body for update
// swagger:response sysConfigResponse
type sysConfigResponseBody struct {
	// in:body
	Body types.SystemConfig
}

// swagger:parameters deleteSystemConfig getSystemConfig
// The parameters for finding system-config by id
type sysConfigIDParams struct {
	// in:path
	ID string `json:"id"`
}
