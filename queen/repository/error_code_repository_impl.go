package repository

import (
	"fmt"
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
	id string) (*common.ErrorCode, error) {
	var errorCode common.ErrorCode
	res := ecr.db.Where("id = ?", id).First(&errorCode)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &errorCode, nil
}

// clear - for testing
func (ecr *ErrorCodeRepositoryImpl) clear() {
	clearDB(ecr.db)
}

// Delete error-code
func (ecr *ErrorCodeRepositoryImpl) Delete(
	id string) error {
	res := ecr.db.Where("id = ?", id).Delete(&common.ErrorCode{})
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
	errorCode *common.ErrorCode) (*common.ErrorCode, error) {
	err := errorCode.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	err = ecr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		if errorCode.ID == "" {
			errorCode.ID = uuid.NewV4().String()
			errorCode.CreatedAt = time.Now()
			errorCode.UpdatedAt = time.Now()
			res = tx.Create(errorCode)
		} else {
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
func (ecr *ErrorCodeRepositoryImpl) GetAll() (errorCodes []*common.ErrorCode, err error) {
	errorCodes = make([]*common.ErrorCode, 0)
	res := ecr.db.Order("error_code").Find(&errorCodes)
	if res.Error != nil {
		err = res.Error
		return nil, err
	}
	return errorCodes, nil
}

// Query finds matching configs
func (ecr *ErrorCodeRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.ErrorCode, totalRecords int64, err error) {
	recs = make([]*common.ErrorCode, 0)
	tx := ecr.db.Limit(pageSize).
		Offset(page * pageSize)
	tx = addQueryParamsWhere(params, tx)
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
	totalRecords, _ = ecr.Count(params)
	return
}

// Count counts records by query
func (ecr *ErrorCodeRepositoryImpl) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := ecr.db.Model(&common.ErrorCode{})
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
	message string,
	platformScope string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	all, err := ecr.GetAll()
	if err != nil {
		return nil, err
	}
	return MatchErrorCode(all, message, platformScope, jobScope, taskScope)
}

// MatchErrorCode finds error code matching criteria
func MatchErrorCode(
	errorCodes []*common.ErrorCode,
	message string,
	platformScope string,
	jobScope string,
	taskScope string) (*common.ErrorCode, error) {
	for _, errorCode := range errorCodes {
		if errorCode.JobType != jobScope {
			continue
		}
		if errorCode.PlatformScope != "" && errorCode.PlatformScope != platformScope {
			continue
		}
		if errorCode.TaskTypeScope != "" && errorCode.TaskTypeScope != taskScope {
			continue
		}
		if errorCode.Matches(message) {
			return errorCode, nil
		}
	}
	return nil, fmt.Errorf("no matching error code for message=%s platform=%s job=%s task=%s",
		message, platformScope, jobScope, taskScope)
}
