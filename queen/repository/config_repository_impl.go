// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"

	"plexobject.com/formicary/internal/crypto"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
)

// ConfigRepositoryImpl implements ConfigRepository using GORM.
type ConfigRepositoryImpl struct {
	dbConfig *config.DBConfig
	db       *gorm.DB
	*BaseRepositoryImpl
}

// NewConfigRepositoryImpl creates a new ConfigRepositoryImpl.
func NewConfigRepositoryImpl(
	dbConfig *config.DBConfig,
	db *gorm.DB,
	objectUpdatedHandler ObjectUpdatedHandler,
) (*ConfigRepositoryImpl, error) {
	return &ConfigRepositoryImpl{
		dbConfig:           dbConfig,
		db:                 db,
		BaseRepositoryImpl: NewBaseRepositoryImpl(objectUpdatedHandler),
	}, nil
}

// Get finds a Config by its primary key, scoped strictly to the caller's own tenant.
// A user may only fetch configs they own (by org or by user ID) — guessing another
// tenant's config ID must return not-found, not the decrypted secret.
func (r *ConfigRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.Config, error) {
	var cfg common.Config
	res := r.strictScopedDB(qc, true).Where("id = ?", id).First(&cfg)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := cfg.AfterLoad(r.encryptionKey(qc)); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Delete removes a config by primary key, scoped strictly to the caller's own tenant.
func (r *ConfigRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := r.strictScopedDB(qc, false).Where("id = ?", id).Delete(&common.Config{})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete config with id %v, rows %v", id, res.RowsAffected))
	}
	r.FireObjectUpdatedHandler(qc, id, ObjectDeleted, nil)
	return nil
}

// Save creates or updates a config using an upsert keyed on (configurable_type, configurable_id, name).
func (r *ConfigRepositoryImpl) Save(
	qc *common.QueryContext,
	cfg *common.Config) (*common.Config, error) {
	if err := cfg.ValidateBeforeSave(r.encryptionKey(qc)); err != nil {
		return nil, common.NewValidationError(err)
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		old, _ := r.getByOwnerAndName(cfg.ConfigurableType, cfg.ConfigurableID, cfg.Name)
		if old != nil {
			cfg.ID = old.ID
			cfg.CreatedAt = old.CreatedAt
		}
		var res *gorm.DB
		if cfg.ID == "" {
			cfg.ID = ulid.Make().String()
			cfg.CreatedAt = time.Now()
			cfg.UpdatedAt = time.Now()
			res = tx.Create(cfg)
		} else {
			cfg.UpdatedAt = time.Now()
			res = tx.Save(cfg)
		}
		return res.Error
	})
	if err == nil {
		r.FireObjectUpdatedHandler(qc, cfg.ID, ObjectUpdated, cfg)
	}
	return cfg, err
}

// Query returns a paginated list of configs filtered by params.
func (r *ConfigRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.Config, totalRecords int64, err error) {
	recs = make([]*common.Config, 0)
	tx := r.scopedDB(qc, true).Limit(pageSize).Offset(page * pageSize)
	tx = addQueryParamsWhere(params, tx)
	if len(order) == 0 {
		order = []string{"name"}
	}
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		return nil, 0, res.Error
	}
	totalRecords, _ = r.Count(qc, params)
	for _, rec := range recs {
		if err = rec.AfterLoad(r.encryptionKey(qc)); err != nil {
			return nil, 0, err
		}
	}
	return
}

// Count returns the number of configs matching the query.
func (r *ConfigRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := r.scopedDB(qc, true).Model(&common.Config{})
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	return totalRecords, res.Error
}

// clear is used in tests to wipe all configs.
func (r *ConfigRepositoryImpl) clear() {
	clearDB(r.db)
}

// scopedDB applies tenant-level row visibility based on configurable_type + configurable_id.
// For org-owned configs the caller must have an org; for user-owned configs we scope to the user.
func (r *ConfigRepositoryImpl) scopedDB(qc *common.QueryContext, readonly bool) *gorm.DB {
	if qc.IsAdmin() || (readonly && qc.IsReadAdmin()) {
		return r.db
	}
	if qc.User != nil && qc.User.OrganizationID != "" {
		return r.db.Where(
			"(configurable_type = ? AND configurable_id = ?) OR (configurable_type = ? AND configurable_id = ?)",
			common.ConfigurableTypeOrg, qc.User.OrganizationID,
			common.ConfigurableTypeUser, qc.User.ID,
		)
	}
	if qc.User != nil {
		return r.db.Where("configurable_type = ? AND configurable_id = ?",
			common.ConfigurableTypeUser, qc.User.ID)
	}
	return r.db.Where("1 = 0")
}

// strictScopedDB is used for Get/Delete: it requires the config's owner to match
// EXACTLY the caller's org OR exactly the caller's user ID. It does NOT allow a
// user to access configs that merely share their org — the individual config must
// be owned by an entity the caller controls. This prevents cross-tenant ID-guessing.
func (r *ConfigRepositoryImpl) strictScopedDB(qc *common.QueryContext, readonly bool) *gorm.DB {
	if qc.IsAdmin() || (readonly && qc.IsReadAdmin()) {
		return r.db
	}
	if qc.User != nil && qc.User.OrganizationID != "" {
		// Allow access to configs directly owned by the caller's org, or by the caller as a user.
		return r.db.Where(
			"(configurable_type = ? AND configurable_id = ?) OR (configurable_type = ? AND configurable_id = ?)",
			common.ConfigurableTypeOrg, qc.User.OrganizationID,
			common.ConfigurableTypeUser, qc.User.ID,
		)
	}
	if qc.User != nil {
		return r.db.Where("configurable_type = ? AND configurable_id = ?",
			common.ConfigurableTypeUser, qc.User.ID)
	}
	return r.db.Where("1 = 0")
}

// scopedOrgDB restricts to org-owned configs only.
// Non-admins may only query configs for their own org.
func (r *ConfigRepositoryImpl) scopedOrgDB(qc *common.QueryContext, orgID string) *gorm.DB {
	if qc.IsAdmin() {
		return r.db.Where("configurable_type = ? AND configurable_id = ?",
			common.ConfigurableTypeOrg, orgID)
	}
	// Enforce membership: caller must belong to the requested org.
	if qc.User == nil || qc.User.OrganizationID != orgID {
		return r.db.Where("1 = 0")
	}
	return r.db.Where("configurable_type = ? AND configurable_id = ?",
		common.ConfigurableTypeOrg, orgID)
}

// scopedUserDB restricts to user-owned configs only.
// Non-admins may only query configs owned by themselves.
func (r *ConfigRepositoryImpl) scopedUserDB(qc *common.QueryContext, userID string) *gorm.DB {
	if qc.IsAdmin() {
		return r.db.Where("configurable_type = ? AND configurable_id = ?",
			common.ConfigurableTypeUser, userID)
	}
	// Enforce ownership: caller must be the user whose configs are being requested.
	if qc.User == nil || qc.User.ID != userID {
		return r.db.Where("1 = 0")
	}
	return r.db.Where("configurable_type = ? AND configurable_id = ?",
		common.ConfigurableTypeUser, userID)
}

// QueryOrgConfigs returns org-scoped configs for the given org ID.
func (r *ConfigRepositoryImpl) QueryOrgConfigs(
	qc *common.QueryContext,
	orgID string,
	page int,
	pageSize int) ([]*common.Config, int64, error) {
	var recs []*common.Config
	res := r.scopedOrgDB(qc, orgID).
		Limit(pageSize).Offset(page*pageSize).
		Order("name").
		Find(&recs)
	if res.Error != nil {
		return nil, 0, res.Error
	}
	var total int64
	r.scopedOrgDB(qc, orgID).Model(&common.Config{}).Count(&total)
	for _, rec := range recs {
		if err := rec.AfterLoad(r.encryptionKey(qc)); err != nil {
			return nil, 0, err
		}
	}
	return recs, total, nil
}

// QueryUserConfigs returns user-scoped configs for the given user ID.
func (r *ConfigRepositoryImpl) QueryUserConfigs(
	qc *common.QueryContext,
	userID string,
	page int,
	pageSize int) ([]*common.Config, int64, error) {
	var recs []*common.Config
	res := r.scopedUserDB(qc, userID).
		Limit(pageSize).Offset(page*pageSize).
		Order("name").
		Find(&recs)
	if res.Error != nil {
		return nil, 0, res.Error
	}
	var total int64
	r.scopedUserDB(qc, userID).Model(&common.Config{}).Count(&total)
	for _, rec := range recs {
		if err := rec.AfterLoad(r.encryptionKey(qc)); err != nil {
			return nil, 0, err
		}
	}
	return recs, total, nil
}

func (r *ConfigRepositoryImpl) getByOwnerAndName(
	configurableType common.ConfigurableType,
	configurableID string,
	name string) (*common.Config, error) {
	var rec common.Config
	res := r.db.
		Where("configurable_type = ? AND configurable_id = ? AND name = ?",
			configurableType, configurableID, name).
		First(&rec)
	if res.Error != nil {
		return nil, res.Error
	}
	return &rec, nil
}

func (r *ConfigRepositoryImpl) encryptionKey(qc *common.QueryContext) []byte {
	if r.dbConfig.EncryptionKey == "" {
		return nil
	}
	return crypto.SHA256Key(r.dbConfig.EncryptionKey + qc.GetSalt())
}
