package repository

import (
	"fmt"
	"runtime/debug"
	"time"

	common "plexobject.com/formicary/internal/types"

	log "github.com/sirupsen/logrus"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// JobResourceRepositoryImpl implements JobResourceRepository using gorm O/R mapping
type JobResourceRepositoryImpl struct {
	db *gorm.DB
}

// NewJobResourceRepositoryImpl creates new instance for job-resource-repository
func NewJobResourceRepositoryImpl(db *gorm.DB) (*JobResourceRepositoryImpl, error) {
	return &JobResourceRepositoryImpl{db: db}, nil
}

// Get method finds JobResource by id
func (jrr *JobResourceRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*types.JobResource, error) {
	var resource types.JobResource
	res := qc.AddOrgElseUserWhere(jrr.db, true).Preload("Configs").
		Where("id = ?", id).
		Where("active = ?", true).
		First(&resource)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := resource.AfterLoad(); err != nil {
		return nil, common.NewValidationError(err)
	}
	return &resource, nil
}

// clear - for testing
func (jrr *JobResourceRepositoryImpl) clear() {
	clearDB(jrr.db)
}

// SetDisabled - sets disabled status job-definition
func (jrr *JobResourceRepositoryImpl) SetDisabled(
	qc *common.QueryContext,
	id string,
	disabled bool) error {
	res := qc.AddOrgElseUserWhere(jrr.db.Model(&types.JobResource{}), false).
		Where("id = ?", id).
		Updates(map[string]interface{}{"disabled": disabled, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to set disabled resource (%v) with id %v, rows %v", disabled, id, res.RowsAffected))
	}
	return nil
}

// Delete job-resource
func (jrr *JobResourceRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.AddOrgElseUserWhere(jrr.db.Model(&types.JobResource{}), false).
		Where("id = ?", id).
		Where("active = ?", true).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete resource with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Save persists job-resource
func (jrr *JobResourceRepositoryImpl) Save(
	qc *common.QueryContext,
	resource *types.JobResource) (*types.JobResource, error) {
	err := resource.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	resource.OrganizationID = qc.GetOrganizationID()
	resource.UserID = qc.GetUserID()
	err = jrr.db.Transaction(func(tx *gorm.DB) error {
		old, err := jrr.getByExternalID(qc, resource.ExternalID)
		if err == nil {
			resource.ID = old.ID
			resource.CreatedAt = old.CreatedAt
		}
		newReq := false
		if resource.ExternalID == "" {
			resource.ExternalID = ulid.Make().String()
		}
		if resource.ID == "" {
			resource.ID = ulid.Make().String()
			resource.CreatedAt = time.Now()
			resource.UpdatedAt = time.Now()
			newReq = true
		} else {
			old, err := jrr.Get(qc, resource.ID)
			if err != nil {
				return err
			}
			if !old.Editable(qc.GetUserID(), qc.GetOrganizationID()) {
				debug.PrintStack()
				log.WithFields(log.Fields{
					"Component":   "JobResourceRepositoryImpl",
					"JobResource": resource,
					"QC":          qc,
				}).Warnf("invalid owner %s / %s didn't match query context",
					resource.UserID, resource.OrganizationID)
				return common.NewPermissionError(
					fmt.Errorf("cannot access job resource %s", resource.ID))
			}
			resource.UpdatedAt = time.Now()
			jrr.clearOrphanJobConfigs(tx, resource)
		}
		resource.Active = true
		var res *gorm.DB

		for _, c := range resource.Configs {
			if c.ID == "" {
				c.ID = ulid.Make().String()
			}
			c.JobResourceID = resource.ID
		}

		if newReq {
			res = tx.Omit("Uses").Create(resource)
		} else {
			res = tx.Omit("Uses").Save(resource)
		}
		if res.Error != nil {
			return res.Error
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(log.Fields{
				"Component": "JobResourceRepositoryImpl",
				"Resource":  resource.String(),
				"ID":        resource.ID,
				"New":       newReq,
			}).
				Debug("saving resource")
		}
		return nil
	})
	return resource, err
}

// getLatestByType finds JobDefinition by type without active flag
func (jrr *JobResourceRepositoryImpl) getByExternalID(qc *common.QueryContext, id string) (*types.JobResource, error) {
	var resource types.JobResource
	res := qc.AddOrgElseUserWhere(jrr.db.Model(&resource), true).
		Where("external_id = ?", id).Find(&resource)
	if res.Error != nil {
		return nil, res.Error
	}
	return &resource, nil
}

// MatchByTags matches tags
func (jrr *JobResourceRepositoryImpl) MatchByTags(
	qc *common.QueryContext,
	resourceType string,
	platform string,
	tags []string,
	value int) (matching []*types.JobResource, total int, err error) {
	resources := make([]*types.JobResource, 0)
	tx := qc.AddOrgElseUserWhere(jrr.db, true).
		Preload("Configs").
		Preload("Uses", "active = ?", true).
		Limit(1000).
		Where("active = ?", true).
		Where("disabled = ?", false).
		Where("platform = ?", platform).
		Where("resource_type = ?", resourceType)
	res := tx.Find(&resources)
	if res.Error != nil {
		return nil, 0, err
	}
	total = len(resources)
	for _, res := range resources {
		if err = res.AfterLoad(); err != nil {
			return
		}

		if res.RemainingQuota() >= value && res.MatchTag(tags) == nil {
			matching = append(matching, res)
		}
	}
	return
}

// Allocate job-resource
func (jrr *JobResourceRepositoryImpl) Allocate(
	resource *types.JobResource,
	use *types.JobResourceUse) (*types.JobResourceUse, error) {
	err := use.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	err = jrr.db.Transaction(func(tx *gorm.DB) error {
		use.ID = ulid.Make().String()
		use.JobResourceID = resource.ID
		use.CreatedAt = time.Now()
		use.UpdatedAt = time.Now()
		use.Active = true
		res := tx.Create(use)
		if res.Error != nil {
			return res.Error
		}
		sum, err := jrr.usedQuota(resource.ID, tx)
		if err != nil {
			return err
		}
		if sum > resource.Quota {
			return fmt.Errorf("failed to save %v because it exceeded quota %d", use, sum)
		}
		use.RemainingQuota = sum
		//log.WithFields(log.Fields{"use": use.String(), "id": resource.ID, "remaining": sum}).Info("allocating resource")
		return nil
	})
	return use, err
}

// Deallocate job-resource
func (jrr *JobResourceRepositoryImpl) Deallocate(
	use *types.JobResourceUse) error {
	res := jrr.db.Model(&use).
		Where("id = ?", use.ID).
		Where("active = ?", true).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete resource use with id %v, rows %v", use.ID, res.RowsAffected))
	}
	return nil

}

// GetUsedQuota of job-resource given resource id
func (jrr *JobResourceRepositoryImpl) GetUsedQuota(
	id string) (total int, err error) {
	return jrr.usedQuota(id, jrr.db)
}

// GetResourceUses job-resource uses for given resource id
func (jrr *JobResourceRepositoryImpl) GetResourceUses(
	qc *common.QueryContext,
	id string) ([]*types.JobResourceUse, error) {
	uses := make([]*types.JobResourceUse, 0)
	tx := qc.AddOrgElseUserWhere(jrr.db, true).
		Limit(10000).
		Where("active = ?", true).Where("job_resource_id = ?", id)
	res := tx.Where("job_resource_id = ?", id).Find(&uses)
	if res.Error != nil {
		return nil, res.Error
	}
	return uses, nil
}

// clearOrphanJobConfigs remove any configs that are no longer active
func (jrr *JobResourceRepositoryImpl) clearOrphanJobConfigs(
	tx *gorm.DB,
	resource *types.JobResource) {
	configIDs := make([]string, len(resource.Configs))
	for i, c := range resource.Configs {
		configIDs[i] = c.ID
	}

	// check in-clause
	tx.Where("id NOT IN ? AND job_resource_id = ?", configIDs, resource.ID).Delete(types.JobResourceConfig{})
}

func (jrr *JobResourceRepositoryImpl) usedQuota(
	id string,
	db *gorm.DB) (total int, err error) {
	err = db.Select("COALESCE(SUM(value), 0) as total").
		Where("job_resource_id = ?", id).
		Where("active = ?", true).
		Table("formicary_job_resource_uses").Row().Scan(&total)
	return
}

// SaveConfig persists config for job-resource
func (jrr *JobResourceRepositoryImpl) SaveConfig(
	qc *common.QueryContext,
	resID string,
	name string,
	value interface{}) (config *types.JobResourceConfig, err error) {
	err = jrr.db.Transaction(func(tx *gorm.DB) error {
		old, _ := jrr.Get(qc, resID)
		if old == nil {
			return common.NewNotFoundError(fmt.Errorf("saving config failed because cannot find resource id '%s'", resID))
		}

		config, err = old.AddConfig(name, value)
		if err != nil {
			return common.NewValidationError(err)
		}
		config.JobResourceID = old.ID
		err = config.Validate()
		if err != nil {
			return common.NewValidationError(err)
		}
		if config.ID == "" {
			config.ID = ulid.Make().String()
		}
		res := tx.Save(config)
		return res.Error
	})
	return config, err
}

// DeleteConfig removes config for job-resource
func (jrr *JobResourceRepositoryImpl) DeleteConfig(
	qc *common.QueryContext,
	resID string,
	configID string,
) error {
	return jrr.db.Transaction(func(tx *gorm.DB) error {
		old, _ := jrr.Get(qc, resID)
		if old == nil {
			return common.NewNotFoundError(fmt.Errorf("deleting config failed because cannot find resource id '%s'", resID))
		}
		cfg := old.GetConfigByID(configID)
		if cfg == nil {
			return common.NewNotFoundError(fmt.Errorf("deleting config failed because cannot find resource id '%s'", resID))
		}
		res := tx.Delete(cfg)
		return res.Error
	})
}

// Query finds matching job-resource by parameters
func (jrr *JobResourceRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (resources []*types.JobResource, totalRecords int64, err error) {
	resources = make([]*types.JobResource, 0)
	tx := qc.AddOrgElseUserWhere(jrr.db, true).
		Preload("Configs").
		Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true)
	tx = jrr.addQuery(params, tx)

	if len(order) == 0 {
		order = []string{"resource_type"}
	}
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&resources)
	if res.Error != nil {
		return nil, 0, err
	}
	for _, resource := range resources {
		if err = resource.AfterLoad(); err != nil {
			return
		}
	}
	totalRecords, _ = jrr.Count(qc, params)
	return
}

// Count counts records by query
func (jrr *JobResourceRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := qc.AddOrgElseUserWhere(jrr.db.Model(&types.JobResource{}), true).
		Where("active = ?", true)
	tx = jrr.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		return 0, err
	}
	return
}

func (jrr *JobResourceRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("resource_type LIKE ? OR description LIKE ? OR platform LIKE ? OR category = ? OR tags_serialized LIKE ?",
			qs, qs, qs, q, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}
