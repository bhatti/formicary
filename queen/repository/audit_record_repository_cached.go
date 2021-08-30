package repository

import (
	"github.com/karlseguin/ccache/v2"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

// AuditRecordRepositoryCached implements AuditRecordRepository with caching support
type AuditRecordRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    AuditRecordRepository
	cache      *ccache.Cache
}

// NewAuditRecordRepositoryCached creates new instance for user-repository
func NewAuditRecordRepositoryCached(
	serverConf *config.ServerConfig,
	adapter AuditRecordRepository) (AuditRecordRepository, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(1000))
	return &AuditRecordRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// GetKinds method finds kinds of AuditRecords
func (arcu *AuditRecordRepositoryCached) GetKinds() ([]types.AuditKind, error) {
	item, err := arcu.cache.Fetch("Kinds",
		arcu.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return arcu.adapter.GetKinds()
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]types.AuditKind), nil
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
