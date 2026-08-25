// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/internal/types"
)

// BannerRepository manages persistent dashboard banners.
type BannerRepository interface {
	// GetActive returns active banners matching the given scope and orgID.
	// scope="" returns all active banners. orgID="" returns only global banners.
	GetActive(scope, orgID string) ([]*types.Banner, error)
	// Save creates or updates a banner by ID.
	Save(banner *types.Banner) error
	// UpsertByKey idempotently creates or updates a system banner by its dedup key.
	UpsertByKey(key, level, scope, orgID, message string) error
	// ClearByKey deactivates a system banner (sets active=false) by key.
	ClearByKey(key string) error
	// Delete hard-deletes a banner by ID.
	Delete(id string) error
	// Query returns paginated banners for admin management.
	Query(params map[string]interface{}, page, pageSize int) ([]*types.Banner, int64, error)
}

// BannerRepositoryImpl implements BannerRepository using GORM.
type BannerRepositoryImpl struct {
	db *gorm.DB
}

// NewBannerRepositoryImpl creates a new repository instance.
func NewBannerRepositoryImpl(db *gorm.DB) (*BannerRepositoryImpl, error) {
	return &BannerRepositoryImpl{db: db}, nil
}

// GetActive returns all active banners matching scope and orgID.
func (r *BannerRepositoryImpl) GetActive(scope, orgID string) ([]*types.Banner, error) {
	tx := r.db.Where("active = ?", true)
	if scope != "" {
		tx = tx.Where("scope = ?", scope)
	}
	if orgID != "" {
		tx = tx.Where("org_id = ?", orgID)
	} else if scope == types.BannerScopeGlobal {
		tx = tx.Where("org_id = ''")
	}
	var banners []*types.Banner
	if err := tx.Order("created_at desc").Find(&banners).Error; err != nil {
		return nil, err
	}
	return banners, nil
}

// Save creates or updates a banner. Sets ID via ULID if empty.
func (r *BannerRepositoryImpl) Save(banner *types.Banner) error {
	if banner.ID == "" {
		banner.ID = ulid.Make().String()
	}
	if banner.Level == "" {
		banner.Level = types.BannerLevelWarning
	}
	if banner.Scope == "" {
		banner.Scope = types.BannerScopeGlobal
	}
	if banner.Source == "" {
		banner.Source = types.BannerSourceAdmin
	}
	if banner.Message == "" {
		return fmt.Errorf("banner message is required")
	}
	return r.db.Save(banner).Error
}

// UpsertByKey creates or updates a system banner identified by key.
func (r *BannerRepositoryImpl) UpsertByKey(key, level, scope, orgID, message string) error {
	if key == "" {
		return fmt.Errorf("key is required for system banners")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing types.Banner
		res := tx.Where("key = ?", key).First(&existing)
		if res.Error == gorm.ErrRecordNotFound {
			// Create new
			b := &types.Banner{
				ID:      ulid.Make().String(),
				Key:     key,
				Level:   level,
				Scope:   scope,
				OrgID:   orgID,
				Source:  types.BannerSourceSystem,
				Message: message,
				Active:  true,
			}
			return tx.Create(b).Error
		}
		if res.Error != nil {
			return res.Error
		}
		// Update existing
		return tx.Model(&existing).Updates(map[string]interface{}{
			"level":      level,
			"message":    message,
			"active":     true,
			"updated_at": time.Now(),
		}).Error
	})
}

// ClearByKey deactivates a banner by its dedup key.
func (r *BannerRepositoryImpl) ClearByKey(key string) error {
	if key == "" {
		return nil
	}
	return r.db.Model(&types.Banner{}).
		Where("key = ? AND active = ?", key, true).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()}).Error
}

// Delete hard-deletes a banner by ID.
func (r *BannerRepositoryImpl) Delete(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return r.db.Where("id = ?", id).Delete(&types.Banner{}).Error
}

// Query returns paginated banners for admin management.
func (r *BannerRepositoryImpl) Query(params map[string]interface{}, page, pageSize int) ([]*types.Banner, int64, error) {
	tx := r.db.Model(&types.Banner{})
	if v, ok := params["id"]; ok {
		tx = tx.Where("id = ?", v)
	}
	if v, ok := params["scope"]; ok {
		tx = tx.Where("scope = ?", v)
	}
	if v, ok := params["active"]; ok {
		tx = tx.Where("active = ?", v)
	}
	if v, ok := params["source"]; ok {
		tx = tx.Where("source = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var banners []*types.Banner
	offset := page * pageSize
	if err := tx.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&banners).Error; err != nil {
		return nil, 0, err
	}
	return banners, total, nil
}
