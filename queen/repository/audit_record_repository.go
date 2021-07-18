package repository

import "plexobject.com/formicary/queen/types"

// AuditRecordRepository defines data access methods for audit-records
type AuditRecordRepository interface {
	// Query queries audit-record by parameters
	Query(
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobs []*types.AuditRecord, totalRecords int64, err error)
	// Save - saves audit-records
	Save(job *types.AuditRecord) (*types.AuditRecord, error)
}
