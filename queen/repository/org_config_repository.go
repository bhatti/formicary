package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// OrganizationConfigRepository defines data access methods for org configs
type OrganizationConfigRepository interface {
	// Query Queries configs
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.OrganizationConfig, totalRecords int64, err error)
	// Count of configs
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)
	// Get Finds OrganizationConfig by id
	Get(
		qc *common.QueryContext,
		id string) (*common.OrganizationConfig, error)
	// Delete org config
	Delete(
		qc *common.QueryContext,
		id string) error
	// Save Saves org-config
	Save(
		qc *common.QueryContext,
		ec *common.OrganizationConfig) (*common.OrganizationConfig, error)
}
