package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/internal/acl"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// OrganizationConfigController structure
type OrganizationConfigController struct {
	auditRecordRepository repository.AuditRecordRepository
	orgConfigRepository   repository.OrganizationConfigRepository
	webserver             web.Server
}

// NewOrganizationConfigController instantiates controller for updating org-configs
func NewOrganizationConfigController(
	auditRecordRepository repository.AuditRecordRepository,
	orgConfigRepository repository.OrganizationConfigRepository,
	webserver web.Server) *OrganizationConfigController {
	cfgCtrl := &OrganizationConfigController{
		orgConfigRepository:   orgConfigRepository,
		auditRecordRepository: auditRecordRepository,
		webserver:             webserver,
	}
	webserver.GET("/api/orgs/:org/configs", cfgCtrl.queryOrganizationConfigs, acl.NewPermission(acl.Organization, acl.View)).Name = "query_org_configs"
	webserver.GET("/api/orgs/:org/configs/:id", cfgCtrl.getOrganizationConfig, acl.NewPermission(acl.Organization, acl.View)).Name = "get_org_config"
	webserver.POST("/api/orgs/:org/configs", cfgCtrl.postOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "create_org_config"
	webserver.PUT("/api/orgs/:org/configs/:id", cfgCtrl.putOrganizationConfig, acl.NewPermission(acl.Organization, acl.View)).Name = "update_org_config"
	webserver.DELETE("/api/orgs/:org/configs/:id", cfgCtrl.deleteOrganizationConfig, acl.NewPermission(acl.Organization, acl.Update)).Name = "delete_org_config"
	return cfgCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/orgs/{orgId}/configs organization-configs queryOrganizationConfigs
// Queries organization configs by criteria such as name, type, etc.
// responses:
//   200: orgConfigQueryResponse
func (cc *OrganizationConfigController) queryOrganizationConfigs(c web.APIContext) error {
	params, order, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := cc.orgConfigRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// swagger:route POST /api/orgs/{orgId}/configs organization-configs postOrganizationConfig
// Adds a config for the organization.
// responses:
//   200: orgConfig
func (cc *OrganizationConfigController) postOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	now := time.Now()
	cfg, err := common.NewOrganizationConfig(qc.GetOrganizationID(), "", "", false)
	if err != nil {
		return err
	}
	err = json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	cfg.OrganizationID = qc.GetOrganizationID()
	saved, err := cc.orgConfigRepository.Save(qc, cfg)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "OrganizationConfigController",
			"Config":    cfg,
			"Error":     err,
		}).Warn("failed to save org config")
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromOrganizationConfig(saved, qc))
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	return c.JSON(status, saved)
}

// swagger:route PUT /api/orgs/{orgId}/configs/{id} organization-configs putOrganizationConfig
// Updates a config for the organization.
// responses:
//   200: orgConfig
func (cc *OrganizationConfigController) putOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := common.NewOrganizationConfig(qc.GetOrganizationID(), "", "", false)
	if err != nil {
		return err
	}
	err = json.NewDecoder(c.Request().Body).Decode(cfg)
	if err != nil {
		return err
	}
	cfg.OrganizationID = qc.GetOrganizationID()
	saved, err := cc.orgConfigRepository.Save(qc, cfg)
	if err != nil {
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromOrganizationConfig(saved, qc))
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/orgs/{orgId}/configs/{id} organization-configs getOrganizationConfig
// Finds a config for the organization by id.
// responses:
//   200: orgConfig
func (cc *OrganizationConfigController) getOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := cc.orgConfigRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cfg)
}

// swagger:route DELETE /api/orgs/{orgId}/configs/{id} organization-configs deleteOrganizationConfig
// Deletes a config for the organization by id.
// responses:
//   200: emptyResponse
// deleteOrganizationConfig - deletes org-config by id
func (cc *OrganizationConfigController) deleteOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := cc.orgConfigRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryOrgConfigs
// The params for querying orgConfigs.
type orgConfigQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Name defines name of property
	Name string `yaml:"name" json:"name"`
}

// Paginated results of orgConfigs matching query
// swagger:response orgConfigQueryResponse
type orgConfigQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []common.OrganizationConfig
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters deleteOrganizationConfig getOrganizationConfig
// The parameters for accessing org-config by id
type orgConfigIDParams struct {
	// in:path
	OrgID string `json:"orgId"`
	// in:path
	ID string `json:"id"`
}

// swagger:parameters putOrganizationConfig
// The parameters for updating org config by id
type orgConfigUpdateParams struct {
	// in:path
	OrgID string `json:"orgId"`
	// in:path
	ID string `json:"id"`
	// in:body
	Body common.OrganizationConfig
}

// swagger:parameters postOrganizationConfig
// The request body includes job-request for persistence.
type orgConfigParams struct {
	// in:body
	Body common.OrganizationConfig
}

// OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution.
// swagger:response orgConfig
type orgConfigBody struct {
	// in:body
	Body common.OrganizationConfig
}
