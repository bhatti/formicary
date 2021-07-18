package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"

	"plexobject.com/formicary/queen/types"
)

// JobDefinitionRepositoryCached implements JobDefinitionRepository with caching support
type JobDefinitionRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    JobDefinitionRepository
	cache      *ccache.Cache
}

// NewJobDefinitionRepositoryCached creates new instance for job-definition-repository
func NewJobDefinitionRepositoryCached(
	serverConf *config.ServerConfig,
	adapter JobDefinitionRepository) (JobDefinitionRepository, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(100))
	return &JobDefinitionRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// Get method finds JobDefinition by id
func (jdr *JobDefinitionRepositoryCached) Get(
	qc *common.QueryContext,
	id string) (*types.JobDefinition, error) {
	item, err := jdr.cache.Fetch("jobDefinitionGet:"+id+qc.String(),
		jdr.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return jdr.adapter.Get(qc, id)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*types.JobDefinition), nil
}

// GetByTypeAndSemanticVersion - finds JobDefinition by type and version
func (jdr *JobDefinitionRepositoryCached) GetByTypeAndSemanticVersion(
	qc *common.QueryContext,
	jobType string,
	semVersion string) (*types.JobDefinition, error) {
	return jdr.GetByType(qc, jobType+":"+semVersion)
}

// GetByType finds JobDefinition by type -- there should be one job-definition per type
func (jdr *JobDefinitionRepositoryCached) GetByType(
	qc *common.QueryContext,
	jobType string) (*types.JobDefinition, error) {
	item, err := jdr.cache.Fetch("jobDefinitionByType:"+jobType+qc.String(),
		jdr.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return jdr.adapter.GetByType(qc, jobType)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*types.JobDefinition), nil
}

// GetJobTypesAndCronTrigger return types of jobs and cron triggers
func (jdr *JobDefinitionRepositoryCached) GetJobTypesAndCronTrigger(
	qc *common.QueryContext) ([]types.JobTypeCronTrigger, error) {
	item, err := jdr.cache.Fetch("jobDefinitionsTypesAndCronTrigger:"+qc.String(),
		jdr.serverConf.Jobs.DBObjectCache*2, func() (interface{}, error) {
			return jdr.adapter.GetJobTypesAndCronTrigger(qc)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]types.JobTypeCronTrigger), nil
}

// SetPaused - sets paused status job-definition
func (jdr *JobDefinitionRepositoryCached) SetPaused(
	id string, paused bool) error {
	err := jdr.adapter.SetPaused(id, paused)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + id)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + jdr.getIDToTypeMapping(id))
	}
	return err
}

// Delete job-definition
func (jdr *JobDefinitionRepositoryCached) Delete(
	qc *common.QueryContext,
	id string) error {
	err := jdr.adapter.Delete(qc, id)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + id)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + jdr.getIDToTypeMapping(id))
		jdr.cache.DeletePrefix("jobDefinitions")
	}
	return err
}

// SetMaxConcurrency sets max-concurrency
func (jdr *JobDefinitionRepositoryCached) SetMaxConcurrency(
	id string,
	concurrency int) error {
	err := jdr.adapter.SetMaxConcurrency(id, concurrency)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + id)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + jdr.getIDToTypeMapping(id))
		jdr.cache.DeletePrefix("jobDefinitions")
	}
	return err
}

// Save persists job-definition
func (jdr *JobDefinitionRepositoryCached) Save(
	qc *common.QueryContext,
	job *types.JobDefinition) (*types.JobDefinition, error) {
	saved, err := jdr.adapter.Save(qc, job)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + saved.ID)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + job.JobType)
		jdr.cache.DeletePrefix("jobDefinitions")
		jdr.addIDToTypeMapping(saved.ID, saved.JobType)
	}
	return job, err
}

// SaveConfig persists config for job-definition
func (jdr *JobDefinitionRepositoryCached) SaveConfig(
	qc *common.QueryContext,
	jobID string,
	name string,
	value interface{},
	secret bool) (*types.JobDefinitionConfig, error) {
	saved, err := jdr.adapter.SaveConfig(qc, jobID, name, value, secret)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + jobID)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + jdr.getIDToTypeMapping(jobID))
		jdr.cache.DeletePrefix("jobDefinitions")
	}
	return saved, err
}

// DeleteConfig removes config for job-definition
func (jdr *JobDefinitionRepositoryCached) DeleteConfig(
	qc *common.QueryContext,
	jobID string,
	configID string,
) error {
	err := jdr.adapter.DeleteConfig(qc, jobID, configID)
	if err == nil {
		jdr.cache.DeletePrefix("jobDefinitionGet:" + jobID)
		jdr.cache.DeletePrefix("jobDefinitionByType:" + jdr.getIDToTypeMapping(jobID))
		jdr.cache.DeletePrefix("jobDefinitions")
	}
	return err
}

// Query finds matching job-definition by parameters
func (jdr *JobDefinitionRepositoryCached) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobs []*types.JobDefinition, totalRecords int64, err error) {
	return jdr.adapter.Query(qc, params, page, pageSize, order) // no caching
}

// Count counts records by query
func (jdr *JobDefinitionRepositoryCached) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	return jdr.adapter.Count(qc, params) // no caching
}

// Clear - for testing
func (jdr *JobDefinitionRepositoryCached) Clear() {
	jdr.adapter.Clear()
	jdr.cache.Clear()
}

func (jdr *JobDefinitionRepositoryCached) addIDToTypeMapping(id string, jobType string) {
	jdr.cache.Replace("jobDefinitionIDToTypeMapping:"+id, jobType)
}

func (jdr *JobDefinitionRepositoryCached) getIDToTypeMapping(id string) string {
	item := jdr.cache.Get("jobDefinitionIDToTypeMapping:" + id)
	if item == nil {
		return ""
	}
	return item.Value().(string)
}
