package repository

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
	"time"

	common "plexobject.com/formicary/internal/types"

	"github.com/twinj/uuid"
	"gorm.io/gorm"
)

// ErrorCodeRepositoryImpl implements ErrorCodeRepository using gorm O/R mapping
type ErrorCodeRepositoryImpl struct {
	db *gorm.DB
}

// NewErrorCodeRepositoryImpl creates new instance for error-code-repository
func NewErrorCodeRepositoryImpl(
	db *gorm.DB) (*ErrorCodeRepositoryImpl, error) {
	return &ErrorCodeRepositoryImpl{db: db}, nil
}

// Get method finds ErrorCode by id
func (ecr *ErrorCodeRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.ErrorCode, error) {
	var errorCode common.ErrorCode
	tx := ecr.db.Where("id = ?", id)
	if qc.HasOrganization() {
		tx = tx.Where("organization_id = ? OR (organization_id = '' AND user_id = '')", qc.GetOrganizationID())
	} else {
		tx = tx.Where("user_id = ? OR (organization_id = '' AND user_id = '')", qc.GetUserID())
	}
	res := tx.First(&errorCode)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &errorCode, nil
}

// Delete error-code
func (ecr *ErrorCodeRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.AddOrgElseUserWhere(ecr.db, true).
		Where("id = ?", id).Delete(&common.ErrorCode{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete error-code with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Save persists error-code
func (ecr *ErrorCodeRepositoryImpl) Save(
	qc *common.QueryContext,
	errorCode *common.ErrorCode) (*common.ErrorCode, error) {
	err := errorCode.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	if qc.IsReadAdmin() || (qc.GetUserID() == "" && qc.GetOrganizationID() == "") {
		errorCode.OrganizationID = ""
		errorCode.UserID = ""
	} else {
		errorCode.OrganizationID = qc.GetOrganizationID()
		errorCode.UserID = qc.GetUserID()
	}
	err = ecr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		if errorCode.ID == "" {
			errorCode.ID = uuid.NewV4().String()
			errorCode.CreatedAt = time.Now()
			errorCode.UpdatedAt = time.Now()
			res = tx.Create(errorCode)
		} else {
			old, err := ecr.Get(qc, errorCode.ID)
			if err != nil {
				return err

			}
			if !old.Editable(qc.GetUserID(), qc.GetOrganizationID()) {
				logrus.WithFields(logrus.Fields{
					"Component": "ErrorCodeRepositoryImpl",
					"ErrorCodd": errorCode,
					"QC":        qc,
				}).Warnf("invalid owner %s / %s didn't match query context",
					errorCode.UserID, errorCode.OrganizationID)
				return common.NewPermissionError(
					fmt.Errorf("cannot access error code %s", errorCode.ID))
			}
			errorCode.UpdatedAt = time.Now()
			res = tx.Save(errorCode)
		}
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return errorCode, err
}

// GetAll returns all error codes
func (ecr *ErrorCodeRepositoryImpl) GetAll(
	qc *common.QueryContext,
) (errorCodes []*common.ErrorCode, err error) {
	errorCodes = make([]*common.ErrorCode, 0)
	tx := ecr.db.Limit(10000)
	if qc.GetUserID() == "" && qc.GetOrganizationID() == "" {
		tx = tx.Where("organization_id = ? OR organization_id is null", qc.GetOrganizationID()).
			Where("user_id = ? OR user_id is null", qc.GetUserID())
	} else {
		if qc.HasOrganization() {
			tx = tx.Where("organization_id = ?", qc.GetOrganizationID())
		} else {
			tx = tx.Where("user_id = ?", qc.GetUserID())
		}
	}
	res := tx.Order("error_code").Find(&errorCodes)
	if res.Error != nil {
		err = res.Error
		return nil, err
	}
	return errorCodes, nil
}

// Query finds matching configs
func (ecr *ErrorCodeRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.ErrorCode, totalRecords int64, err error) {
	recs = make([]*common.ErrorCode, 0)
	tx := ecr.db.Limit(pageSize).Offset(page * pageSize)
	if qc.IsReadAdmin() {
	} else if qc.HasOrganization() {
		tx = tx.Where("organization_id = ?", qc.GetOrganizationID())
	} else {
		tx = tx.Where("user_id = ?", qc.GetUserID())
	}
	tx = ecr.addQuery(params, tx)
	if len(order) == 0 {
		order = []string{"error_code"}
	}
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	totalRecords, _ = ecr.Count(qc, params)
	return
}

func (ecr *ErrorCodeRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("regex LIKE ? OR error_code LIKE ? OR description LIKE ? OR display_message LIKE ? OR display_code LIKE ? OR job_type LIKE ? OR task_type_scope LIKE ? OR platform_scope LIKE ? OR command_scope LIKE ? OR action LIKE ?",
			qs, qs, qs, qs, qs, qs, qs, qs, qs, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}

// Count counts records by query
func (ecr *ErrorCodeRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := ecr.db.Model(&common.ErrorCode{})
	if qc.IsReadAdmin() {
	} else if qc.HasOrganization() {
		tx = tx.Where("organization_id = ?", qc.GetOrganizationID())
	} else {
		tx = tx.Where("user_id = ?", qc.GetUserID())
	}
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

// Match finds error code matching criteria
func (ecr *ErrorCodeRepositoryImpl) Match(
	qc *common.QueryContext,
	message string,
	platform string,
	command string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	all, err := ecr.GetAll(qc)
	if err != nil {
		return nil, err
	}
	return MatchErrorCode(all, message, platform, command, jobScope, taskScope)
}

// MatchErrorCode finds error code matching criteria
func MatchErrorCode(
	errorCodes []*common.ErrorCode,
	message string,
	platform string,
	command string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	wildOrEmpty := func(s string) bool {
		return s == "" || s == "*"
	}

	for _, errorCode := range errorCodes {
		if !wildOrEmpty(errorCode.PlatformScope) && errorCode.PlatformScope != platform {
			continue
		}
		if !wildOrEmpty(errorCode.JobType) && errorCode.JobType != jobScope {
			continue
		}
		if !wildOrEmpty(errorCode.TaskTypeScope) && errorCode.TaskTypeScope != taskScope {
			continue
		}
		if !wildOrEmpty(errorCode.CommandScope) && !strings.Contains(command, errorCode.CommandScope) {
			continue
		}
		if errorCode.Matches(message) {
			return errorCode, nil
		}
	}
	return nil, fmt.Errorf("no matching error code for message=%s platform=%s command=%s job=%s task=%s",
		message, platform, command, jobScope, taskScope)
}

// clear - for testing
func (ecr *ErrorCodeRepositoryImpl) clear() {
	clearDB(ecr.db)
}
