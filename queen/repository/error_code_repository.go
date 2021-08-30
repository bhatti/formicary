package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// ErrorCodeRepository defines data access methods for error-codes
type ErrorCodeRepository interface {
	// GetAll returns all error-codes
	GetAll() ([]*common.ErrorCode, error)
	// Get finds ErrorCode by id
	Get(id string) (*common.ErrorCode, error)
	// Delete error-code
	Delete(id string) error
	// Save - persists error-code
	Save(ec *common.ErrorCode) (*common.ErrorCode, error)
	Query(params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.ErrorCode, totalRecords int64, err error)
	Count(params map[string]interface{}) (totalRecords int64, err error)
	Match(message string, platformScope string, jobScope string, taskScope string) (*common.ErrorCode, error)
}
