package repository

import "plexobject.com/formicary/queen/types"

// SystemConfigRepository defines data access methods for system configs
type SystemConfigRepository interface {
	// Queries configs
	Query(
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobs []*types.SystemConfig, totalRecords int64, err error)
	// GetByKindName - finds config by kind and name
	GetByKindName(kind string, name string) (*types.SystemConfig, error)
	// Count of configs
	Count(params map[string]interface{}) (totalRecords int64, err error)
	// Finds SystemConfig by id
	Get(id string) (*types.SystemConfig, error)
	// Delete system-config
	Delete(id string) error
	// Saves system-config
	Save(ec *types.SystemConfig) (*types.SystemConfig, error)
}
