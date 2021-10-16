package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// JobDefinitionRepository defines data access methods for job-definition
type JobDefinitionRepository interface {
	// Get finds JobDefinition by id
	Get(
		qc *common.QueryContext,
		id string) (*types.JobDefinition, error)
	// GetByType - finds JobDefinition by type -- there should be one job-definition per type
	GetByType(
		qc *common.QueryContext,
		jobType string) (*types.JobDefinition, error)
	// GetByTypeAndSemanticVersion - finds JobDefinition by type and version
	GetByTypeAndSemanticVersion(
		qc *common.QueryContext,
		jobType string,
		semVersion string) (*types.JobDefinition, error)
	// SetDisabled disables/enables job-definition
	SetDisabled(id string, disabled bool) error
	// Delete job-definition
	Delete(
		qc *common.QueryContext,
		id string) error
	// Save - saves job-definition
	Save(
		qc *common.QueryContext,
		job *types.JobDefinition) (*types.JobDefinition, error)
	// SetMaxConcurrency sets concurrency
	SetMaxConcurrency(id string, concurrency int) error
	// GetJobTypesAndCronTrigger finds job-types and triggers
	GetJobTypesAndCronTrigger(
		qc *common.QueryContext) ([]types.JobTypeCronTrigger, error)
	// Query - queries job-definition by parameters
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobs []*types.JobDefinition, totalRecords int64, err error)
	// Count - counts records by query
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)
	// SaveConfig persists config for job-definition
	SaveConfig(
		qc *common.QueryContext,
		jobID string,
		name string,
		value interface{},
		secret bool) (*types.JobDefinitionConfig, error)
	// DeleteConfig removes config for job-definition
	DeleteConfig(
		qc *common.QueryContext,
		jobID string,
		configID string,
	) error
	// Clear - for testing
	Clear()
}
