package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// OrganizationRepository defines data access methods for orgs
type OrganizationRepository interface {
	// Get - Finds Organization
	Get(
		qc *common.QueryContext,
		id string) (*common.Organization, error)
	// GetByUnit - Finds Organization by unit
	GetByUnit(
		qc *common.QueryContext,
		unit string) (*common.Organization, error)
	// GetByParentID - Finds Organization by parent id
	GetByParentID(
		qc *common.QueryContext,
		parent string) ([]*common.Organization, error)
	// Delete Organization
	Delete(
		qc *common.QueryContext,
		id string) error
	// Create - Saves Organization
	Create(
		qc *common.QueryContext,
		org *common.Organization) (*common.Organization, error)
	// Update - Saves Organization
	Update(
		qc *common.QueryContext,
		org *common.Organization) (*common.Organization, error)
	UpdateStickyMessage(
		qc *common.QueryContext,
		user *common.User,
		org *common.Organization) error
	// Query - queries orgs
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.Organization, totalRecords int64, err error)
	// Count counts
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)

	Clear()
}
