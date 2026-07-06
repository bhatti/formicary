// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// OrganizationConfigController manages org-scoped config properties.
type OrganizationConfigController struct {
	auditRecordRepository repository.AuditRecordRepository
	configRepository      repository.ConfigRepository
	webserver             web.Server
}

// NewOrganizationConfigController registers org config REST endpoints.
func NewOrganizationConfigController(
	auditRecordRepository repository.AuditRecordRepository,
	configRepository repository.ConfigRepository,
	webserver web.Server) *OrganizationConfigController {
	c := &OrganizationConfigController{
		auditRecordRepository: auditRecordRepository,
		configRepository:      configRepository,
		webserver:             webserver,
	}
	webserver.GET("/api/orgs/:org/configs", c.queryOrganizationConfigs, acl.NewPermission(acl.OrgConfig, acl.View)).Name = "query_org_configs"
	webserver.GET("/api/orgs/:org/configs/:id", c.getOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.View)).Name = "get_org_config"
	webserver.GET("/api/orgs/:org/configs/:id/reveal", c.revealOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Update)).Name = "reveal_org_config"
	webserver.POST("/api/orgs/:org/configs", c.postOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Create)).Name = "create_org_config"
	webserver.PUT("/api/orgs/:org/configs/:id", c.putOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Update)).Name = "update_org_config"
	webserver.DELETE("/api/orgs/:org/configs/:id", c.deleteOrganizationConfig, acl.NewPermission(acl.OrgConfig, acl.Delete)).Name = "delete_org_config"
	return c
}

func (cc *OrganizationConfigController) queryOrganizationConfigs(c web.APIContext) error {
	_, _, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	orgID := qc.GetOrganizationID()
	// Admins may specify any org via the URL param; non-admins are always scoped to their own org.
	if orgID == "" && qc.IsAdmin() {
		orgID = c.Param("org")
	}
	if orgID == "" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "no organization"})
	}
	recs, total, err := cc.configRepository.QueryOrgConfigs(qc, orgID, page, pageSize)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if r.Secret {
			r.Value = "****"
		}
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

func (cc *OrganizationConfigController) postOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	orgID := qc.GetOrganizationID()
	if orgID == "" && qc.IsAdmin() {
		orgID = c.Param("org")
	}
	now := time.Now()
	cfg, err := common.NewOrgConfig(orgID, "", "", false)
	if err != nil {
		return err
	}
	if err = json.NewDecoder(c.Request().Body).Decode(cfg); err != nil {
		return err
	}
	cfg.ConfigurableID = orgID
	cfg.ConfigurableType = common.ConfigurableTypeOrg
	saved, err := cc.configRepository.Save(qc, cfg)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "OrganizationConfigController",
			"Config":    cfg,
			"Error":     err,
		}).Warn("failed to save org config")
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(saved, qc))
	if saved.Secret {
		saved.Value = "****"
	}
	st := http.StatusOK
	if saved.CreatedAt.Unix() >= now.Unix() {
		st = http.StatusCreated
	}
	return c.JSON(st, saved)
}

func (cc *OrganizationConfigController) putOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	orgID := qc.GetOrganizationID()
	if orgID == "" && qc.IsAdmin() {
		orgID = c.Param("org")
	}
	cfg, err := common.NewOrgConfig(orgID, "", "", false)
	if err != nil {
		return err
	}
	if err = json.NewDecoder(c.Request().Body).Decode(cfg); err != nil {
		return err
	}
	cfg.ConfigurableID = orgID
	cfg.ConfigurableType = common.ConfigurableTypeOrg
	if cfg.Secret && cfg.Value == "****" {
		existing, getErr := cc.configRepository.Get(qc, c.Param("id"))
		if getErr != nil {
			return getErr
		}
		cfg.Value = existing.Value
	}
	saved, err := cc.configRepository.Save(qc, cfg)
	if err != nil {
		return err
	}
	if saved.Secret {
		saved.Value = "****"
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(saved, qc))
	return c.JSON(http.StatusOK, saved)
}

func (cc *OrganizationConfigController) revealOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := cc.configRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return c.JSON(http.StatusOK, cfg)
}

func (cc *OrganizationConfigController) getOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := cc.configRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	if cfg.Secret {
		cfg.Value = "****"
	}
	return c.JSON(http.StatusOK, cfg)
}

func (cc *OrganizationConfigController) deleteOrganizationConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	if err := cc.configRepository.Delete(qc, c.Param("id")); err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

type orgConfigQueryParams struct {
	// in:path
	Org string `json:"org"`
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Name     string `json:"name"`
}

type orgConfigQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*common.Config
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

type orgConfigIDParams struct {
	// in:path
	Org string `json:"org"`
	ID  string `json:"id"`
}

type orgConfigParams struct {
	// in:path
	Org string `json:"org"`
	// in:body
	Body common.Config
}

type orgConfigBody struct {
	// in:body
	Body common.Config
}

type orgConfigUpdateParams struct {
	// in:path
	Org string `json:"org"`
	ID  string `json:"id"`
	// in:body
	Body common.Config
}
