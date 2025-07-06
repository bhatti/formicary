package tasklet

import (
	"fmt"
	"plexobject.com/formicary/internal/metrics"
	"runtime/debug"
	"sync"

	"plexobject.com/formicary/internal/types"
)

// RequestRegistry interface for keeping track of current jobs
type RequestRegistry interface {
	Add(
		req *types.TaskRequest) error
	Cancel(
		key string) error
	CancelJob(
		requestID string) error
	Remove(
		req *types.TaskRequest) error
	Count() int
	GetAllocations() (allocations map[string]*types.AntAllocation)
}

// RequestRegistryImpl keeps track of in-progress jobs
type RequestRegistryImpl struct {
	commonConfig    *types.CommonConfig
	metricsRegistry *metrics.Registry
	requests        map[string]*types.TaskRequest
	lock            sync.RWMutex
}

// NewRequestRegistry constructor
func NewRequestRegistry(
	commonConfig *types.CommonConfig,
	metricsRegistry *metrics.Registry,
) RequestRegistry {
	return &RequestRegistryImpl{
		commonConfig:    commonConfig,
		metricsRegistry: metricsRegistry,
		requests:        make(map[string]*types.TaskRequest),
	}
}

// Add - adds request
func (r *RequestRegistryImpl) Add(
	req *types.TaskRequest) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if req == nil {
		return fmt.Errorf("request not specified")
	}
	if req.Key() == "" {
		return fmt.Errorf("request task key not specified for add: %s", req)
	}
	if r.requests[req.Key()] != nil {
		return fmt.Errorf("request task key already exists")

	}
	req.Status = types.READY
	r.requests[req.Key()] = req
	return nil
}

// Cancel cancels the request
func (r *RequestRegistryImpl) Cancel(
	key string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if key == "" {
		debug.PrintStack()
		return fmt.Errorf("request task key not specified for cancel: %s", key)
	}
	req := r.requests[key]
	if req == nil {
		return fmt.Errorf("request not found for key: %s", key)
	}
	if req.Cancel == nil {
		return fmt.Errorf("request cancel not found")
	}
	req.Cancel()
	req.Cancelled = true
	return nil
}

// CancelJob cancels the request by job ID
func (r *RequestRegistryImpl) CancelJob(
	requestID string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, req := range r.requests {
		if req.JobRequestID == requestID && req.Cancel != nil {
			//debug.PrintStack()
			req.Cancel()
			delete(r.requests, req.Key())
			break
		}
	}
	return nil
}

// Remove - removes request
func (r *RequestRegistryImpl) Remove(
	req *types.TaskRequest) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if req == nil {
		return fmt.Errorf("request not specified")
	}
	if req.Key() == "" {
		return fmt.Errorf("request task key not specified to remove: %s", req)
	}
	if r.requests[req.Key()] == nil {
		return fmt.Errorf("request task key not found")

	}
	delete(r.requests, req.Key())
	return nil
}

// Count - number of requests
func (r *RequestRegistryImpl) Count() int {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return len(r.requests)
}

// GetAllocations - returns allocations of requests
func (r *RequestRegistryImpl) GetAllocations() (allocations map[string]*types.AntAllocation) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	allocations = make(map[string]*types.AntAllocation)
	for _, req := range r.requests {
		allocations[req.JobRequestID] = types.NewAntAllocation(
			r.commonConfig.ID,
			r.commonConfig.GetRequestTopic(),
			req.JobRequestID,
			req.TaskType,
		)
	}
	return
}
