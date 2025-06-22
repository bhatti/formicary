package repository

import (
	"fmt"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
	"sort"
	"time"
)

// AuditRecordRepositoryImpl implements AuditRecordRepository using gorm O/R mapping
type AuditRecordRepositoryImpl struct {
	db *gorm.DB
}

// NewAuditRecordRepositoryImpl creates new instance for audit-record-repository
func NewAuditRecordRepositoryImpl(db *gorm.DB) (*AuditRecordRepositoryImpl, error) {
	return &AuditRecordRepositoryImpl{db: db}, nil
}

// GetKinds returns list of audit record kinds
func (arr *AuditRecordRepositoryImpl) GetKinds() (kinds []types.AuditKind, err error) {
	arr.db.Model(&types.AuditRecord{}).Distinct().Pluck("kind", &kinds)
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return
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
		record.ID = ulid.Make().String()
		record.CreatedAt = time.Now()
		res = tx.Create(record)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return record, err
}

// Query finds matching audit-records by parameters
func (arr *AuditRecordRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*types.AuditRecord, totalRecords int64, err error) {
	records = make([]*types.AuditRecord, 0)
	tx := arr.db.Limit(pageSize).Offset(page * pageSize)

	tx = arr.addQuery(params, tx)

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
	tx = arr.addQuery(params, tx)
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

func (arr *AuditRecordRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("target_id LIKE ? OR user_id LIKE ? OR organization_id LIKE ? OR kind = ? OR message LIKE ?",
			qs, qs, qs, q, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}
