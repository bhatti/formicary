package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"sort"
)

// ErrorCodeRepositoryCached implements ErrorCodeRepository in-memory cache
type ErrorCodeRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    *ErrorCodeRepositoryImpl
	cache      *ccache.Cache
}

// NewErrorCodeRepositoryCached creates new instance for error-code-repository
func NewErrorCodeRepositoryCached(
	serverConf *config.ServerConfig,
	adapter *ErrorCodeRepositoryImpl) (*ErrorCodeRepositoryCached, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(200))
	return &ErrorCodeRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// Get method finds ErrorCode by id
func (ecc *ErrorCodeRepositoryCached) Get(
	id string) (*common.ErrorCode, error) {
	item, err := ecc.cache.Fetch("errorCodeGet:"+id,
		ecc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return ecc.adapter.Get(id)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*common.ErrorCode), nil
}

// Delete error-code
func (ecc *ErrorCodeRepositoryCached) Delete(
	id string) error {
	err := ecc.adapter.Delete(id)
	if err == nil {
		ecc.cache.Delete("errorCodeGet:" + id)
		ecc.cache.DeletePrefix("errorCodes")
	}
	return err
}

// Save persists error-code
func (ecc *ErrorCodeRepositoryCached) Save(
	errorCode *common.ErrorCode) (*common.ErrorCode, error) {
	saved, err := ecc.adapter.Save(errorCode)
	if err == nil {
		ecc.cache.Delete("errorCodeGet:" + saved.ID)
		ecc.cache.DeletePrefix("errorCodes")
	}
	return saved, err
}

// GetAll returns all error codes
func (ecc *ErrorCodeRepositoryCached) GetAll() (errorCodes []*common.ErrorCode, err error) {
	item, err := ecc.cache.Fetch("errorCodes",
		ecc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			all, err := ecc.adapter.GetAll()
			if err != nil {
				return nil, err
			}
			sortErrorCodes(all)
			return all, nil
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]*common.ErrorCode), nil
}

// Query finds matching configs
func (ecc *ErrorCodeRepositoryCached) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.ErrorCode, totalRecords int64, err error) {
	return ecc.adapter.Query(params, page, pageSize, order) // no caching
}

// Count counts records by query
func (ecc *ErrorCodeRepositoryCached) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	return ecc.adapter.Count(params) // no caching
}

// Clear - for testing
func (ecc *ErrorCodeRepositoryCached) clear() {
	ecc.cache.Clear()
	ecc.adapter.clear()
}

// Match finds error code matching criteria
func (ecc *ErrorCodeRepositoryCached) Match(
	message string,
	platformScope string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	all, err := ecc.GetAll()
	if err != nil {
		return nil, err
	}
	return MatchErrorCode(all, message, platformScope, jobScope, taskScope)
}

// sort error codes so that most precise or longest matches show up first
func sortErrorCodes(all []*common.ErrorCode) {
	// Descending order
	sort.Slice(all, func(i, j int) bool {
		if len(all[i].PlatformScope) > len(all[j].PlatformScope) {
			return true
		} else if len(all[i].PlatformScope) < len(all[j].PlatformScope) {
			return false
		} else {
			if len(all[i].JobType) > len(all[j].JobType) {
				return true
			} else if len(all[i].JobType) < len(all[j].JobType) {
				return false
			} else {
				if len(all[i].TaskTypeScope) > len(all[j].TaskTypeScope) {
					return true
				} else if len(all[i].TaskTypeScope) < len(all[j].TaskTypeScope) {
					return false
				}
				return len(all[i].Regex) > len(all[j].Regex)
			}
		}
	})
}
