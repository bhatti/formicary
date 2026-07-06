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

// UserConfigController manages per-user credential/config properties.
type UserConfigController struct {
	auditRecordRepository repository.AuditRecordRepository
	configRepository      repository.ConfigRepository
	webserver             web.Server
}

// NewUserConfigController registers user config REST endpoints.
// All endpoints are scoped to the authenticated user — no `:user` path param needed.
func NewUserConfigController(
	auditRecordRepository repository.AuditRecordRepository,
	configRepository repository.ConfigRepository,
	webserver web.Server) *UserConfigController {
	c := &UserConfigController{
		auditRecordRepository: auditRecordRepository,
		configRepository:      configRepository,
		webserver:             webserver,
	}
	webserver.GET("/api/users/configs", c.queryUserConfigs, acl.NewPermission(acl.UserConfig, acl.Query)).Name = "query_user_configs"
	webserver.GET("/api/users/configs/:id", c.getUserConfig, acl.NewPermission(acl.UserConfig, acl.View)).Name = "get_user_config"
	webserver.GET("/api/users/configs/:id/reveal", c.revealUserConfig, acl.NewPermission(acl.UserConfig, acl.Update)).Name = "reveal_user_config"
	webserver.POST("/api/users/configs", c.postUserConfig, acl.NewPermission(acl.UserConfig, acl.Create)).Name = "create_user_config"
	webserver.PUT("/api/users/configs/:id", c.putUserConfig, acl.NewPermission(acl.UserConfig, acl.Update)).Name = "update_user_config"
	webserver.DELETE("/api/users/configs/:id", c.deleteUserConfig, acl.NewPermission(acl.UserConfig, acl.Delete)).Name = "delete_user_config"
	return c
}

func (uc *UserConfigController) queryUserConfigs(c web.APIContext) error {
	_, _, page, pageSize, _, _ := ParseParams(c)
	qc := web.BuildQueryContext(c)
	recs, total, err := uc.configRepository.QueryUserConfigs(qc, qc.GetUserID(), page, pageSize)
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

func (uc *UserConfigController) postUserConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	now := time.Now()
	cfg, err := common.NewUserConfig(qc.GetUserID(), "", "", false)
	if err != nil {
		return err
	}
	if err = json.NewDecoder(c.Request().Body).Decode(cfg); err != nil {
		return err
	}
	cfg.ConfigurableID = qc.GetUserID()
	cfg.ConfigurableType = common.ConfigurableTypeUser
	saved, err := uc.configRepository.Save(qc, cfg)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "UserConfigController",
			"Config":    cfg,
			"Error":     err,
		}).Warn("failed to save user config")
		return err
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(saved, qc))
	if saved.Secret {
		saved.Value = "****"
	}
	st := http.StatusOK
	if saved.CreatedAt.Unix() >= now.Unix() {
		st = http.StatusCreated
	}
	return c.JSON(st, saved)
}

func (uc *UserConfigController) putUserConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := common.NewUserConfig(qc.GetUserID(), "", "", false)
	if err != nil {
		return err
	}
	if err = json.NewDecoder(c.Request().Body).Decode(cfg); err != nil {
		return err
	}
	cfg.ConfigurableID = qc.GetUserID()
	cfg.ConfigurableType = common.ConfigurableTypeUser
	if cfg.Secret && cfg.Value == "****" {
		existing, getErr := uc.configRepository.Get(qc, c.Param("id"))
		if getErr != nil {
			return getErr
		}
		cfg.Value = existing.Value
	}
	saved, err := uc.configRepository.Save(qc, cfg)
	if err != nil {
		return err
	}
	if saved.Secret {
		saved.Value = "****"
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(saved, qc))
	return c.JSON(http.StatusOK, saved)
}

func (uc *UserConfigController) revealUserConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := uc.configRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	_, _ = uc.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return c.JSON(http.StatusOK, cfg)
}

func (uc *UserConfigController) getUserConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	cfg, err := uc.configRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	if cfg.Secret {
		cfg.Value = "****"
	}
	return c.JSON(http.StatusOK, cfg)
}

func (uc *UserConfigController) deleteUserConfig(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	if err := uc.configRepository.Delete(qc, c.Param("id")); err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
