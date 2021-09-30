package repository

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"plexobject.com/formicary/internal/crypto"
	"plexobject.com/formicary/queen/config"

	common "plexobject.com/formicary/internal/types"

	log "github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// JobDefinitionRepositoryImpl implements JobDefinitionRepository using gorm O/R mapping
type JobDefinitionRepositoryImpl struct {
	dbConfig *config.DBConfig
	db       *gorm.DB
}

// NewJobDefinitionRepositoryImpl creates new instance for job-definition-repository
func NewJobDefinitionRepositoryImpl(dbConfig *config.DBConfig, db *gorm.DB) (*JobDefinitionRepositoryImpl, error) {
	return &JobDefinitionRepositoryImpl{dbConfig: dbConfig, db: db}, nil
}

// Get method finds JobDefinition by id
func (jdr *JobDefinitionRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (job *types.JobDefinition, err error) {
	if id == "" {
		debug.PrintStack()
		return nil, common.NewValidationError(
			fmt.Errorf("job-id is not specified for fetching job-definition"))
	}
	job = &types.JobDefinition{}
	scopeCond, scopeArg := qc.AddOrgUserWhereSQL(true)
	res := jdr.db.Preload("Tasks").
		Preload("Configs").
		Preload("Variables").
		Preload("Tasks.Variables").
		Where("id = ?", id).
		Where("public_plugin = ? OR "+scopeCond, true, scopeArg).
		First(job)
	if res.Error != nil {
		log.WithFields(log.Fields{
			"Component": "JobDefinitionRepositoryImpl",
			"ID":        id,
			"QC":        qc,
			"Scope":     scopeCond,
			"ScopeArg":  scopeArg,
		}).Warnf("JobDefinitionRepositoryImpl.Get couldn't find job")
		return nil, common.NewNotFoundError(res.Error)
	}

	if job, err = jdr.postProcessJob(qc, job); err != nil {
		return nil, err
	}

	if !job.PublicPlugin && !qc.IsReadAdmin() {
		if (job.OrganizationID != "" && job.OrganizationID != qc.GetOrganizationID()) ||
			(job.OrganizationID == "" && job.UserID != qc.GetUserID()) {
			debug.PrintStack()
			log.WithFields(log.Fields{
				"Component":     "JobDefinitionRepositoryImpl",
				"JobDefinition": job,
				"QC":            qc,
			}).Warnf("JobDefinitionRepositoryImpl.Get job owner %s / %s didn't match query context",
				job.UserID, job.OrganizationID)
			return nil, common.NewPermissionError(
				fmt.Errorf("cannot access job by id %s", id))
		}
	}
	for _, t := range job.Tasks {
		sort.Slice(t.Variables, func(i, j int) bool { return t.Variables[i].Name < t.Variables[j].Name })
	}
	return job, nil
}

// GetByTypeAndSemanticVersion - finds JobDefinition by type and version
func (jdr *JobDefinitionRepositoryImpl) GetByTypeAndSemanticVersion(
	qc *common.QueryContext,
	jobType string,
	semVersion string) (*types.JobDefinition, error) {
	return jdr.GetByType(qc, jobType+":"+semVersion)
}

// GetByType finds JobDefinition by type -- there should be one job-definition per type
func (jdr *JobDefinitionRepositoryImpl) GetByType(
	qc *common.QueryContext,
	jobType string) (job *types.JobDefinition, err error) {
	semVersion := ""
	scopeCond, scopeArg := qc.AddOrgUserWhereSQL(true)
	jobTypeAndVersion := strings.Split(jobType, ":")
	if len(jobTypeAndVersion) == 2 {
		jobType = jobTypeAndVersion[0]
		semVersion = jobTypeAndVersion[1]
	}
	if jobType == "" {
		debug.PrintStack()
		return nil, common.NewValidationError("job-type is not specified")
	}
	job = &types.JobDefinition{}
	tx := jdr.db.Preload("Tasks").
		Preload("Configs").
		Preload("Variables").
		Preload("Tasks.Variables").
		Where("job_type = ?", jobType)

	var res *gorm.DB
	if semVersion == "" {
		res = tx.Where("active = ?", true).Where(scopeCond, scopeArg).First(job)
	} else {
		res = tx.Where("sem_version = ? AND public_plugin = ?", semVersion, true).First(job)
		if res.Error != nil {
			res = tx.Where("active = ?", true).Where(scopeCond, scopeArg).First(job)
		}
	}
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}

	if job, err = jdr.postProcessJob(qc, job); err != nil {
		return nil, err
	}

	if !job.PublicPlugin && !qc.IsReadAdmin() {
		if (job.OrganizationID != "" && job.OrganizationID != qc.GetOrganizationID()) ||
			(job.OrganizationID == "" && job.UserID != qc.GetUserID()) {
			debug.PrintStack()
			log.WithFields(log.Fields{
				"Component":     "JobDefinitionRepositoryImpl",
				"JobDefinition": job,
				"QC":            qc,
			}).Warnf("JobDefinitionRepositoryImpl.GetByType job owner %s / %s didn't match query context",
				job.UserID, job.OrganizationID)
			return nil, common.NewPermissionError(
				fmt.Errorf("cannot access job by type %s", jobType))
		}
	}
	for _, t := range job.Tasks {
		sort.Slice(t.Variables, func(i, j int) bool { return t.Variables[i].Name < t.Variables[j].Name })
	}
	return job, nil
}

// SetPaused - sets paused status job-definition -- only admin can do it so no need for query context
func (jdr *JobDefinitionRepositoryImpl) SetPaused(id string, paused bool) error {
	var job types.JobDefinition
	res := jdr.db.Model(&job).
		Where("id = ?", id).
		Updates(map[string]interface{}{"paused": paused, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to set paused job with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Delete job-definition
func (jdr *JobDefinitionRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.AddOrgElseUserWhere(jdr.db.Model(&types.JobDefinition{}), false).
		Where("id = ?", id).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete job with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// SetMaxConcurrency sets max-concurrency -- only admin can do it so no need for query context
func (jdr *JobDefinitionRepositoryImpl) SetMaxConcurrency(id string, concurrency int) error {
	if concurrency <= 0 {
		return common.NewValidationError(
			fmt.Errorf("concurrency is not valid %d", concurrency))
	}
	if concurrency > jdr.dbConfig.MaxConcurrency {
		return common.NewValidationError(
			fmt.Errorf("concurrency exceeds max limit %d", concurrency))
	}
	var job types.JobDefinition
	res := jdr.db.Model(&job).
		Where("id = ?", id).
		Where("active = ?", true).
		Updates(map[string]interface{}{"max_concurrency": concurrency, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to update concurrency to %d with id %s, affected %d", concurrency, id, res.RowsAffected))
	}
	return nil
}

// GetJobTypesAndCronTrigger returns types of jobs and cron trigger -- only admin can do it so no need for query context
func (jdr *JobDefinitionRepositoryImpl) GetJobTypesAndCronTrigger(
	qc *common.QueryContext) ([]types.JobTypeCronTrigger, error) {
	sql := "SELECT distinct user_id, organization_id, job_type, cron_trigger FROM formicary_job_definitions WHERE active = ? "
	args := []interface{}{true}
	if qc.IsAdmin() {
	} else if qc.GetOrganizationID() != "" {
		sql += " AND organization_id = ? "
		args = append(args, qc.GetOrganizationID())
	} else if qc.GetUserID() != "" {
		sql += " AND user_id = ? "
		args = append(args, qc.GetUserID())
	}
	sql += " limit 1000000"

	rows, err := jdr.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	cronTriggersByJobType := make([]types.JobTypeCronTrigger, 0)

	for rows.Next() {
		var trigger types.JobTypeCronTrigger
		if err = jdr.db.ScanRows(rows, &trigger); err != nil {
			return nil, err
		}
		cronTriggersByJobType = append(cronTriggersByJobType, trigger)
	}
	sort.Slice(cronTriggersByJobType, func(i, j int) bool { return cronTriggersByJobType[i].JobType < cronTriggersByJobType[j].JobType })
	return cronTriggersByJobType, nil
}

// SaveConfig persists config for job-definition
func (jdr *JobDefinitionRepositoryImpl) SaveConfig(
	qc *common.QueryContext,
	jobID string,
	name string,
	value interface{},
	secret bool) (config *types.JobDefinitionConfig, err error) {
	err = jdr.db.Transaction(func(tx *gorm.DB) error {
		old, _ := jdr.Get(qc, jobID)
		if old == nil {
			return common.NewNotFoundError(fmt.Errorf("saving config failed because cannot find job id '%s'", jobID))
		}

		config, err = old.AddConfig(name, value, secret)
		if err != nil {
			return common.NewValidationError(err)
		}
		config.JobDefinitionID = old.ID
		if err = config.ValidateBeforeSave(jdr.encryptionKey(qc)); err != nil {
			return common.NewValidationError(err)
		}
		if config.ID == "" {
			config.ID = uuid.NewV4().String()
		}
		res := tx.Save(config)
		return res.Error
	})
	return config, err
}

// DeleteConfig removes config for job-definition
func (jdr *JobDefinitionRepositoryImpl) DeleteConfig(
	qc *common.QueryContext,
	jobID string,
	configID string,
) error {
	return jdr.db.Transaction(func(tx *gorm.DB) error {
		old, _ := jdr.Get(qc, jobID)
		if old == nil {
			return common.NewNotFoundError(fmt.Errorf("deleting config failed because cannot find job id '%s'", jobID))
		}
		cfg := old.GetConfigByID(configID)
		if cfg == nil {
			return common.NewNotFoundError(fmt.Errorf("deleting config failed because cannot find job id '%s'", jobID))
		}
		res := tx.Delete(cfg)
		return res.Error
	})
}

// Save persists job-definition
func (jdr *JobDefinitionRepositoryImpl) Save(
	qc *common.QueryContext,
	job *types.JobDefinition) (*types.JobDefinition, error) {
	if qc.GetUserID() != job.UserID {
		return nil, fmt.Errorf("user-id doesn't match '%s:%s'", qc.GetUserID(), job.UserID)
	}
	if qc.GetOrganizationID() != job.OrganizationID {
		return nil, fmt.Errorf("organization-id doesn't match '%s:%s'", qc.GetOrganizationID(), job.OrganizationID)
	}
	err := job.ValidateBeforeSave(jdr.encryptionKey(qc))
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	if job.PublicPlugin {
		if semType, err := job.CheckSemVersion(); err == nil {
			// plugin allows publishing using sem-version only once
			if semType == types.ValidSemanticVersion {
				if old, _ := jdr.GetByTypeAndSemanticVersion(
					qc,
					job.JobType,
					job.SemVersion); old != nil &&
					(old.SemVersion == job.SemVersion || job.NormalizedSemVersion() < old.NormalizedSemVersion()) {
					return nil, common.NewDuplicateError(
						fmt.Errorf("plugin %s - %s older version already exists for the version %s",
							job.JobType, old.SemVersion, job.SemVersion))
				}
			}
		} else {
			return nil, common.NewValidationError(err)
		}
	}
	err = jdr.db.Transaction(func(tx *gorm.DB) error {
		old, err := jdr.getLatestByType(qc, job.JobType)
		var res *gorm.DB
		if err == nil && old.ID != "" {
			if job.RawYaml == old.RawYaml &&
				job.MaxConcurrency == old.MaxConcurrency &&
				job.Paused == old.Paused &&
				job.ConfigsString() == old.ConfigsString() &&
				job.VariablesString() == old.VariablesString() &&
				old.Active {
				log.WithFields(log.Fields{
					"Component":   "JobDefinitionRepositoryImpl",
					"Job":         job.String(),
					"Concurrency": old.MaxConcurrency,
					"Paused":      old.Paused,
					"Version":     old.Version,
				}).Info("skip saving job-definition because nothing changed")
				return nil // nothing to do
			}

			if err = jdr.Delete(
				qc,
				old.ID); err != nil {
				return err
			}

			job.Version = old.Version + 1
			job.CreatedAt = time.Now()
			job.UpdatedAt = time.Now()
			job.MaxConcurrency = old.MaxConcurrency // Set it explicitly
			job.Paused = old.Paused                 // Set it explicitly
			job.Configs = old.Configs
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithFields(log.Fields{
					"Component": "JobDefinitionRepositoryImpl",
					"Job":       job.String(),
					"Version":   job.Version,
				}).Debug("saving job-definition...")
			}
		} else {
			job.Version = 0
			job.Paused = false
			job.UpdatedAt = time.Now()

			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithFields(log.Fields{
					"Component": "JobDefinitionRepositoryImpl",
					"Job":       job.String(),
					"Version":   job.Version,
				}).Debug("creating job-definition...")
			}
		}

		job.ID = uuid.NewV4().String() //old.ID
		job.Active = true
		for _, c := range job.Configs {
			if c.ID == "" {
				c.ID = uuid.NewV4().String()
			}
			c.JobDefinitionID = job.ID
		}
		for _, c := range job.Variables {
			c.ID = uuid.NewV4().String()
			c.JobDefinitionID = job.ID
		}
		for _, t := range job.Tasks {
			t.JobDefinitionID = job.ID
			t.ID = uuid.NewV4().String()
			for _, c := range t.Variables {
				c.ID = uuid.NewV4().String()
				c.TaskDefinitionID = t.ID
			}
		}
		res = tx.Omit("Tasks", "Variables").Create(job)
		if res.Error != nil {
			return res.Error
		}
		if err = tx.Model(job).Association("Variables").Replace(job.Variables); err != nil {
			return err
		}
		for _, t := range job.Tasks {
			res = tx.Omit("Variables").Create(t)
			if res.Error != nil {
				return res.Error
			}
			err = tx.Model(t).Association("Variables").Replace(t.Variables)
			if err != nil {
				return err
			}

		}
		if err = tx.Model(job).Association("Configs").Replace(job.Configs); err != nil {
			return err
		}
		return nil
	})
	return job, err
}

// Query finds matching job-definition by parameters
func (jdr *JobDefinitionRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobs []*types.JobDefinition, totalRecords int64, err error) {
	jobs = make([]*types.JobDefinition, 0)
	tx := qc.AddOrgElseUserWhere(jdr.db, true).Preload("Tasks").
		//Preload("Configs").
		Preload("Variables").
		Preload("Tasks.Variables").
		Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true)
	tx = jdr.addQuery(params, tx)

	if len(order) == 0 {
		order = []string{"job_type"}
	}
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&jobs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	for i, job := range jobs {
		if jobs[i], err = jdr.postProcessJob(qc, job); err != nil {
			return
		}
	}
	totalRecords, _ = jdr.Count(qc, params)
	return
}

// Count counts records by query
func (jdr *JobDefinitionRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := qc.AddOrgElseUserWhere(jdr.db.Model(&types.JobDefinition{}), true).Where("active = ?", true)
	tx = jdr.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// getLatestByType finds JobDefinition by type without active flag
func (jdr *JobDefinitionRepositoryImpl) getLatestByType(
	qc *common.QueryContext,
	jobType string) (*types.JobDefinition, error) {
	var job types.JobDefinition
	var count int64

	qc.AddOrgElseUserWhere(jdr.db, true).Model(&job).
		Where("job_type = ?", jobType).
		Count(&count)
	res := qc.AddOrgElseUserWhere(jdr.db, true).Preload("Tasks").Preload("Configs").
		Where("job_type = ?", jobType).
		Where("version = ?", count-1).
		First(&job)
	if res.Error != nil {
		return nil, res.Error
	}
	return &job, nil
}

// Clear - for testing
func (jdr *JobDefinitionRepositoryImpl) Clear() {
	clearDB(jdr.db)
}

// encryptionKey encrypted key
func (jdr *JobDefinitionRepositoryImpl) encryptionKey(
	qc *common.QueryContext) []byte {
	if jdr.dbConfig.EncryptionKey == "" {
		return nil
	}
	return crypto.SHA256Key(jdr.dbConfig.EncryptionKey + qc.GetSalt())
}

func (jdr *JobDefinitionRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("raw_yaml LIKE ? OR description LIKE ? OR user_id LIKE ? OR organization_id LIKE ? OR job_type = ? OR platform = ?",
			qs, qs, qs, qs, q, q)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}

func (jdr *JobDefinitionRepositoryImpl) postProcessJob(
	qc *common.QueryContext,
	job *types.JobDefinition,
) (*types.JobDefinition, error) {
	// job.UsesTemplate
	if parsedJob, err := types.ReloadFromYaml(job.RawYaml); err == nil {
		job = parsedJob
	}
	if err := job.AfterLoad(jdr.encryptionKey(qc)); err != nil {
		return nil, err
	}
	return job, nil
}
