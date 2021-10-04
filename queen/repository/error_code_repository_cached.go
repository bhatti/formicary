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
	qc *common.QueryContext,
	id string) (*common.ErrorCode, error) {
	return ecc.adapter.Get(qc, id)
}

// Delete error-code
func (ecc *ErrorCodeRepositoryCached) Delete(
	qc *common.QueryContext,
	id string) error {
	err := ecc.adapter.Delete(qc, id)
	if err == nil {
		ecc.cache.Delete("errorCodeGet:" + qc.String() + id)
		if (qc.GetUserID() == "" && qc.GetOrganizationID() == "") ||
			qc.IsReadAdmin() {
			ecc.cache.DeletePrefix("errorCodes")
		}
	}
	return err
}

// Save persists error-code
func (ecc *ErrorCodeRepositoryCached) Save(
	qc *common.QueryContext,
	errorCode *common.ErrorCode) (*common.ErrorCode, error) {
	saved, err := ecc.adapter.Save(qc, errorCode)
	if err == nil {
		ecc.cache.Delete("errorCodeGet:" + qc.String() + saved.ID)
		if (qc.GetUserID() == "" && qc.GetOrganizationID() == "") ||
			qc.IsReadAdmin() {
			ecc.cache.DeletePrefix("errorCodes")
		}
	}
	return saved, err
}

// GetAll returns all error codes
func (ecc *ErrorCodeRepositoryCached) GetAll(
	qc *common.QueryContext,
) (errorCodes []*common.ErrorCode, err error) {
	qcErrors, err := ecc.getAll(qc)
	if err != nil || (qc.GetUserID() == "" && qc.GetOrganizationID() == "") {
		return nil, err
	}
	globalErrors, err := ecc.getAll(common.NewQueryContextFromIDs("", ""))
	if err != nil {
		return qcErrors, nil
	}
	qcErrors = append(qcErrors, globalErrors...)
	sortErrorCodes(qcErrors)
	return qcErrors, nil
}

// Query finds matching configs
func (ecc *ErrorCodeRepositoryCached) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.ErrorCode, totalRecords int64, err error) {

	recs, totalRecords, err = ecc.adapter.Query(
		qc,
		params,
		page,
		pageSize,
		order) // no caching
	if err != nil || (qc.GetUserID() == "" && qc.GetOrganizationID() == "") {
		return
	}
	globalRecs, globalTotalRecords, err := ecc.adapter.Query(
		common.NewQueryContextFromIDs("", ""),
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return recs, totalRecords, nil
	}
	recs = append(recs, globalRecs...)
	totalRecords += globalTotalRecords
	return
}

// Count counts records by query
func (ecc *ErrorCodeRepositoryCached) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	return ecc.adapter.Count(qc, params) // no caching
}

// Clear - for testing
func (ecc *ErrorCodeRepositoryCached) clear() {
	ecc.cache.Clear()
	ecc.adapter.clear()
}

// Match finds error code matching criteria
func (ecc *ErrorCodeRepositoryCached) Match(
	qc *common.QueryContext,
	message string,
	platform string,
	command string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	all, err := ecc.getAll(qc)
	if err != nil {
		return nil, err
	}
	ec, err := MatchErrorCode(all, message, platform, command, jobScope, taskScope)
	if err == nil {
		return ec, err
	}
	if global, err := ecc.getAll(common.NewQueryContextFromIDs("", "")); err == nil {
		return MatchErrorCode(global, message, platform, command, jobScope, taskScope)
	}
	return nil, err
}

func (ecc *ErrorCodeRepositoryCached) getAll(
	qc *common.QueryContext,
) (errorCodes []*common.ErrorCode, err error) {
	if qc.GetUserID() == "" && qc.GetOrganizationID() == "" {
		item, err := ecc.cache.Fetch("errorCodes",
			ecc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
				all, err := ecc.adapter.GetAll(qc)
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
	all, err := ecc.adapter.GetAll(qc)
	if err != nil {
		return nil, err
	}
	sortErrorCodes(all)
	return all, nil
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
				} else {
					if len(all[i].CommandScope) > len(all[j].CommandScope) {
						return true
					} else if len(all[i].CommandScope) < len(all[j].CommandScope) {
						return false
					}
					return len(all[i].Regex) > len(all[j].Regex)
				}
			}
		}
	})
}
