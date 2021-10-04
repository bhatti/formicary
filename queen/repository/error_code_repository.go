package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// ErrorCodeRepository defines data access methods for error-codes
type ErrorCodeRepository interface {
	// GetAll returns all error-codes
	GetAll(
		qc *common.QueryContext,
	) ([]*common.ErrorCode, error)
	// Get finds ErrorCode by id
	Get(
		qc *common.QueryContext,
		id string,
	) (*common.ErrorCode, error)
	// Delete error-code
	Delete(
		qc *common.QueryContext,
		id string,
	) error
	// Save - persists error-code
	Save(
		qc *common.QueryContext,
		ec *common.ErrorCode) (*common.ErrorCode, error)
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.ErrorCode, totalRecords int64, err error)
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)
	Match(
		qc *common.QueryContext,
		message string,
		platform string,
		command string,
		jobScope string,
		taskScope string) (*common.ErrorCode, error)
}
