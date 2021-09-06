package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// BaseRepositoryImpl defines base repository
type BaseRepositoryImpl struct {
	objectUpdatedHandler ObjectUpdatedHandler
}

// NewBaseRepositoryImpl constructor
func NewBaseRepositoryImpl(objectUpdatedHandler ObjectUpdatedHandler) *BaseRepositoryImpl {
	return &BaseRepositoryImpl{
		objectUpdatedHandler: objectUpdatedHandler,
	}
}

// FireObjectUpdatedHandler fires update callback
func (r *BaseRepositoryImpl) FireObjectUpdatedHandler(
	qc *common.QueryContext,
	id string,
	kind UpdateKind,
	obj interface{},
) {
	r.objectUpdatedHandler(
		qc,
		id,
		kind,
		obj)
}

func filterParams(params map[string]interface{}, exclude ...string) map[string]interface{} {
	filteredParams := make(map[string]interface{})
	for k, v := range params {
		matched := false
		for _, excludeKey := range exclude {
			if excludeKey == k {
				matched = true
				break
			}
		}
		if !matched {
			filteredParams[k] = v
		}
	}
	return filteredParams
}
