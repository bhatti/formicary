package repository

import (
	"fmt"
	"time"

	"github.com/twinj/uuid"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// AuditRecordRepositoryImpl implements AuditRecordRepository using gorm O/R mapping
type AuditRecordRepositoryImpl struct {
	db *gorm.DB
}

// NewAuditRecordRepositoryImpl creates new instance for audit-record-repository
func NewAuditRecordRepositoryImpl(db *gorm.DB) (*AuditRecordRepositoryImpl, error) {
	return &AuditRecordRepositoryImpl{db: db}, nil
}

// Query finds matching audit-records by parameters
func (arr *AuditRecordRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*types.AuditRecord, totalRecords int64, err error) {
	records = make([]*types.AuditRecord, 0)
	tx := arr.db.Limit(pageSize).Offset(page * pageSize)

	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("target_id LIKE ? OR user_id LIKE ? OR organization_id LIKE ? OR kind = ? OR message LIKE ?",
			qs, qs, qs, q, qs)
	} else {
		tx = addQueryParamsWhere(params, tx)
	}

	if len(order) == 0 {
		tx = tx.Order("created_at desc")
	} else {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	}
	res := tx.Find(&records)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	totalRecords, _ = arr.Count(params)
	return
}

// Count counts records by query
func (arr *AuditRecordRepositoryImpl) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := arr.db.Model(&types.AuditRecord{})
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

// clear - for testing
func (arr *AuditRecordRepositoryImpl) clear() {
	clearDB(arr.db)
}

// Save persists audit-record
func (arr *AuditRecordRepositoryImpl) Save(
	record *types.AuditRecord) (*types.AuditRecord, error) {
	err := record.ValidateBeforeSave()
	if err != nil {
		return nil, err
	}
	err = arr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		record.ID = uuid.NewV4().String()
		record.CreatedAt = time.Now()
		res = tx.Create(record)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return record, err
}
