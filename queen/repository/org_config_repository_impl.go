package repository

import (
	"fmt"
	"time"

	"plexobject.com/formicary/internal/crypto"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

// OrganizationConfigRepositoryImpl implements OrganizationConfigRepository using gorm O/R mapping
type OrganizationConfigRepositoryImpl struct {
	dbConfig *config.DBConfig
	db       *gorm.DB
	*BaseRepositoryImpl
}

// NewOrganizationConfigRepositoryImpl creates new instance for org-config-repository
func NewOrganizationConfigRepositoryImpl(
	dbConfig *config.DBConfig,
	db *gorm.DB,
	objectUpdatedHandler ObjectUpdatedHandler,
) (*OrganizationConfigRepositoryImpl, error) {
	return &OrganizationConfigRepositoryImpl{
		dbConfig:           dbConfig,
		db:                 db,
		BaseRepositoryImpl: NewBaseRepositoryImpl(objectUpdatedHandler),
	}, nil
}

// Get method finds OrganizationConfig by id
func (scr *OrganizationConfigRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.OrganizationConfig, error) {
	var cfg common.OrganizationConfig
	res := qc.AddOrgElseUserWhere(scr.db, true).Where("id = ?", id).First(&cfg)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := cfg.AfterLoad(scr.encryptionKey(qc)); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// clear - for testing
func (scr *OrganizationConfigRepositoryImpl) clear() {
	clearDB(scr.db)
}

// Delete org-config
func (scr *OrganizationConfigRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.AddOrgElseUserWhere(scr.db, false).Where("id = ?", id).Delete(&common.OrganizationConfig{})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete org-config with id %v, rows %v", id, res.RowsAffected))
	}
	scr.FireObjectUpdatedHandler(qc, id, ObjectDeleted, nil)
	return nil
}

// Save persists org-config
func (scr *OrganizationConfigRepositoryImpl) Save(
	qc *common.QueryContext,
	config *common.OrganizationConfig) (*common.OrganizationConfig, error) {
	err := config.ValidateBeforeSave(scr.encryptionKey(qc))
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	config.OrganizationID = qc.GetOrganizationID()
	err = scr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		old, _ := scr.get(config.OrganizationID, config.Name)
		if old != nil {
			config.ID = old.ID
			config.CreatedAt = old.CreatedAt
		}
		if config.ID == "" {
			config.ID = ulid.Make().String()
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
	if err == nil {
		scr.FireObjectUpdatedHandler(qc, config.ID, ObjectUpdated, config)
	}
	return config, err
}

// Query finds matching configs
func (scr *OrganizationConfigRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.OrganizationConfig, totalRecords int64, err error) {
	recs = make([]*common.OrganizationConfig, 0)
	tx := qc.AddOrgElseUserWhere(scr.db, true).Limit(pageSize).
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
	totalRecords, _ = scr.Count(qc, params)
	for _, rec := range recs {
		if err := rec.AfterLoad(scr.encryptionKey(qc)); err != nil {
			return nil, 0, err
		}
	}
	return
}

// Count counts records by query
func (scr *OrganizationConfigRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := qc.AddOrgElseUserWhere(scr.db, true).Model(&common.OrganizationConfig{})
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (scr *OrganizationConfigRepositoryImpl) get(
	orgID string,
	name string) (*common.OrganizationConfig, error) {
	var rec common.OrganizationConfig
	res := scr.db.Where("organization_id = ?", orgID).Where("name = ?", name).First(&rec)
	if res.Error != nil {
		return nil, res.Error
	}
	return &rec, nil
}

// encryptionKey encrypted key
func (scr *OrganizationConfigRepositoryImpl) encryptionKey(
	qc *common.QueryContext) []byte {
	if scr.dbConfig.EncryptionKey == "" {
		return nil
	}
	return crypto.SHA256Key(scr.dbConfig.EncryptionKey + qc.GetSalt())
}
