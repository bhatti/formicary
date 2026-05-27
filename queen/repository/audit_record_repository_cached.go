package repository

import (
	"github.com/karlseguin/ccache/v3"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

// AuditRecordRepositoryCached implements AuditRecordRepository with caching support
type AuditRecordRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    AuditRecordRepository
	cache      *ccache.Cache[[]types.AuditKind]
}

// NewAuditRecordRepositoryCached creates new instance for user-repository
func NewAuditRecordRepositoryCached(
	serverConf *config.ServerConfig,
	adapter AuditRecordRepository) (AuditRecordRepository, error) {
	var cache = ccache.New(ccache.Configure[[]types.AuditKind]().MaxSize(serverConf.Jobs.DBObjectCacheSize))
	return &AuditRecordRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// GetKinds method finds kinds of AuditRecords
func (arcu *AuditRecordRepositoryCached) GetKinds() ([]types.AuditKind, error) {
	item, err := arcu.cache.Fetch("Kinds",
		arcu.serverConf.Jobs.DBObjectCache, func() ([]types.AuditKind, error) {
			return arcu.adapter.GetKinds()
		})
	if err != nil {
		return nil, err
	}
	return item.Value(), nil
}

// Query queries audit-record by parameters
func (arcu *AuditRecordRepositoryCached) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobs []*types.AuditRecord, totalRecords int64, err error) {
	return arcu.adapter.Query(params, page, pageSize, order)
}

// Save - saves audit-records
func (arcu *AuditRecordRepositoryCached) Save(record *types.AuditRecord) (*types.AuditRecord, error) {
	return arcu.adapter.Save(record)
}
