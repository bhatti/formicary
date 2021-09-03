package repository

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"

	"gorm.io/gorm"
)

// ArtifactRepositoryImpl implements ArtifactRepository using gorm O/R mapping
type ArtifactRepositoryImpl struct {
	db *gorm.DB
}

// NewArtifactRepositoryImpl creates new instance for artifact-repository
func NewArtifactRepositoryImpl(db *gorm.DB) (*ArtifactRepositoryImpl, error) {
	return &ArtifactRepositoryImpl{db: db}, nil
}

// GetResourceUsage - Finds usage between time
func (ar *ArtifactRepositoryImpl) GetResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	res := make([]types.ResourceUsage, 0)
	if ranges == nil || len(ranges) == 0 {
		return res, nil
	}
	orgSQL, orgArg := qc.AddOrgUserWhereSQL()
	sql := "SELECT COUNT(*) as count, SUM(content_length) as value FROM formicary_artifacts WHERE active = ? AND updated_at >= ? AND updated_at <= ? AND " + orgSQL
	for _, r := range ranges {
		rows, err := ar.db.Raw(sql, true, r.StartDate, r.EndDate, orgArg).Rows()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			usage := types.ResourceUsage{}
			if err = ar.db.ScanRows(rows, &usage); err != nil {
				return nil, err
			}
			usage.ResourceType = types.DiskResource
			usage.UserID = qc.UserID
			usage.OrganizationID = qc.OrganizationID
			usage.StartDate = r.StartDate
			usage.EndDate = r.EndDate
			usage.ValueUnit = "bytes"
			res = append(res, usage)
		}
	}
	return res, nil
}

// ExpiredArtifacts finds expired artifact
func (ar *ArtifactRepositoryImpl) ExpiredArtifacts(
	qc *common.QueryContext,
	expiration time.Duration,
	limit int) (records []*common.Artifact, err error) {
	records = make([]*common.Artifact, 0)
	tx := qc.AddOrgElseUserWhere(ar.db.Model(&common.Artifact{})).
		Limit(limit).
		Where("active = ?", true).
		Where("expires_at < ? OR updated_at < ?", time.Now(), time.Now().Add(-expiration))
	res := tx.Find(&records)
	if res.Error != nil {
		err = res.Error
		return nil, err
	}
	for _, a := range records {
		if err = a.AfterLoad(); err != nil {
			return
		}
	}
	return
}

// Query finds matching artifacts by parameters
func (ar *ArtifactRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*common.Artifact, total int64, err error) {
	records = make([]*common.Artifact, 0)
	tx := qc.AddOrgElseUserWhere(ar.db.Model(&common.Artifact{})).
		Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true)
	q := params["q"]
	if q != nil {
		reqID, _ := strconv.ParseInt(fmt.Sprintf("%s", q), 10, 64)
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("metadata_serialized LIKE ? OR name LIKE ? OR user_id LIKE ? OR organization_id LIKE ? OR kind = ? OR task_type = ? OR job_request_id = ?",
			qs, qs, qs, qs, q, q, reqID)
	} else {
		tx = addQueryParamsWhere(params, tx)
	}

	if len(order) == 0 {
		tx = tx.Order("created_at desc")
	} else {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	}
	res := tx.Find(&records)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	for _, a := range records {
		if err = a.AfterLoad(); err != nil {
			return
		}
	}
	total, _ = ar.Count(qc, params)
	return
}

// Count counts artifacts by query
func (ar *ArtifactRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (total int64, err error) {
	tx := qc.AddOrgElseUserWhere(ar.db).Where("active = ?", true)
	tx = addQueryParamsWhere(params, tx)
	res := tx.Model(&common.Artifact{}).Count(&total)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

// Clear - for testing
func (ar *ArtifactRepositoryImpl) Clear() {
	clearDB(ar.db)
}

// GetBySHA256 - Finds Artifact by sha256
func (ar *ArtifactRepositoryImpl) GetBySHA256(
	qc *common.QueryContext,
	sha256 string) (*common.Artifact, error) {
	var art common.Artifact
	res := qc.AddOrgElseUserWhere(ar.db).
		Where("sha256 = ?", sha256).
		Where("active = ?", true).
		First(&art)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := art.AfterLoad(); err != nil {
		return nil, err
	}
	return &art, nil
}

// Get method finds artifact by id
func (ar *ArtifactRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.Artifact, error) {
	var art common.Artifact
	res := qc.AddOrgElseUserWhere(ar.db).
		Where("id = ?", id).
		Where("active = ?", true).
		First(&art)
	if res.Error != nil {
		debug.PrintStack()
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := art.AfterLoad(); err != nil {
		return nil, err
	}
	return &art, nil
}

// Delete artifact
func (ar *ArtifactRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	tx := ar.db.Model(&common.Artifact{}).
		Where("id = ?", id).
		Where("active = ?", true)
	tx = qc.AddOrgElseUserWhere(tx)
	res := tx.Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete artifact with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Save persists artifact
func (ar *ArtifactRepositoryImpl) Save(
	art *common.Artifact) (*common.Artifact, error) {
	err := art.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	art.Active = true
	err = ar.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		art.CreatedAt = time.Now()
		art.UpdatedAt = time.Now()
		res = tx.Create(art)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return art, err
}

// Update persists artifact
func (ar *ArtifactRepositoryImpl) Update(
	qc *common.QueryContext,
	art *common.Artifact) (*common.Artifact, error) {
	err := art.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	if !qc.Matches(art.UserID, art.OrganizationID) {
		// TODO remove user id from the error message
		return nil, common.NewPermissionError(
			fmt.Errorf("artifact owner %s / %s didn't match query context %s",
				art.UserID, art.OrganizationID, qc))
	}
	art.Active = true
	err = ar.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		art.UpdatedAt = time.Now()
		res = tx.Save(art)
		if res.Error != nil {
			debug.PrintStack()
			return res.Error
		}
		return nil
	})
	return art, err
}
