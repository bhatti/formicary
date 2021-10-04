package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// JobResourceRepository defines data access methods for job-resource
type JobResourceRepository interface {
	// Get - Finds JobResource by id
	Get(
		qc *common.QueryContext,
		id string) (*types.JobResource, error)
	// SetPaused pauses/unpauses job-definition
	SetPaused(
		qc *common.QueryContext,
		id string,
		paused bool) error
	// Delete job-resource
	Delete(
		qc *common.QueryContext,
		id string) error
	// Save - Saves job-resource
	Save(
		qc *common.QueryContext,
		resource *types.JobResource) (*types.JobResource, error)
	// Query - Queries job-resource by parameters
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (resources []*types.JobResource, totalRecords int64, err error)
	// MatchByTags matches resources by tags
	MatchByTags(
		qc *common.QueryContext,
		resourceType string,
		platform string,
		tags []string,
		value int) (resources []*types.JobResource, total int, err error)
	// Count - Counts records by query
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)

	// Allocate job-resource
	Allocate(
		resource *types.JobResource,
		use *types.JobResourceUse) (*types.JobResourceUse, error)
	// Deallocate job-resource
	Deallocate(
		use *types.JobResourceUse) error
	// GetResourceUses job-resource uses for given resource id
	GetResourceUses(
		qc *common.QueryContext,
		id string) ([]*types.JobResourceUse, error)
	// GetUsedQuota of job-resource given resource id
	GetUsedQuota(
		id string) (int, error)
	// SaveConfig persists config for job-resource
	SaveConfig(
		qc *common.QueryContext,
		resID string,
		name string,
		value interface{}) (*types.JobResourceConfig, error)
	// DeleteConfig removes config for job-resource
	DeleteConfig(
		qc *common.QueryContext,
		resID string,
		configID string,
	) error
}
