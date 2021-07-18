package repository

import (
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"time"

	"github.com/twinj/uuid"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// SystemConfigRepositoryImpl implements SystemConfigRepository using gorm O/R mapping
type SystemConfigRepositoryImpl struct {
	db *gorm.DB
}

// NewSystemConfigRepositoryImpl creates new instance for system-config-repository
func NewSystemConfigRepositoryImpl(db *gorm.DB) (*SystemConfigRepositoryImpl, error) {
	return &SystemConfigRepositoryImpl{db: db}, nil
}

// Get method finds SystemConfig by id
func (scr *SystemConfigRepositoryImpl) Get(id string) (*types.SystemConfig, error) {
	var config types.SystemConfig
	res := scr.db.Where("id = ?", id).First(&config)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &config, nil
}

// clear - for testing
func (scr *SystemConfigRepositoryImpl) clear() {
	clearDB(scr.db)
}

// Delete system-config
func (scr *SystemConfigRepositoryImpl) Delete(
	id string) error {
	res := scr.db.Where("id = ?", id).Delete(&types.SystemConfig{})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete system-config with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Save persists system-config
func (scr *SystemConfigRepositoryImpl) Save(
	config *types.SystemConfig) (*types.SystemConfig, error) {
	err := config.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	err = scr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		old, _ := scr.GetByKindName(config.Kind, config.Name)
		if old != nil {
			config.ID = old.ID
			config.CreatedAt = old.CreatedAt
		}
		if config.ID == "" {
			config.ID = uuid.NewV4().String()
			config.CreatedAt = time.Now()
			config.UpdatedAt = time.Now()
			res = tx.Create(config)
		} else {
			config.UpdatedAt = time.Now()
			res = tx.Save(config)
		}
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return config, err
}

// Query finds matching configs
func (scr *SystemConfigRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*types.SystemConfig, totalRecords int64, err error) {
	recs = make([]*types.SystemConfig, 0)
	tx := scr.db.Limit(pageSize).
		Offset(page * pageSize)
	tx = addQueryParamsWhere(params, tx)
	if len(order) == 0 {
		order = []string{"name"}
	}
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	totalRecords, _ = scr.Count(params)
	return
}

// Count counts records by query
func (scr *SystemConfigRepositoryImpl) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := scr.db.Model(&types.SystemConfig{})
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

// GetByKindName - finds config by kind and name
func (scr *SystemConfigRepositoryImpl) GetByKindName(
	kind string,
	name string) (*types.SystemConfig, error) {
	var rec types.SystemConfig
	res := scr.db.Where("kind = ?", kind).Where("name = ?", name).First(&rec)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &rec, nil
}
