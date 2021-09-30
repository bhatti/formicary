package repository

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
	"time"

	common "plexobject.com/formicary/internal/types"

	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// EmailVerificationRepositoryImpl implements EmailVerificationRepository using gorm O/R mapping
type EmailVerificationRepositoryImpl struct {
	db *gorm.DB
}

// NewEmailVerificationRepositoryImpl creates new instance for user-repository
func NewEmailVerificationRepositoryImpl(
	db *gorm.DB) (*EmailVerificationRepositoryImpl, error) {
	return &EmailVerificationRepositoryImpl{db: db}, nil
}

// Clear - for testing
func (ur *EmailVerificationRepositoryImpl) Clear() {
	clearDB(ur.db)
}

// Create adds new email verification
func (ur *EmailVerificationRepositoryImpl) Create(
	ev *types.EmailVerification) (*types.EmailVerification, error) {
	err := ev.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	var total int64
	res := ur.db.Model(&types.EmailVerification{}).
		Where("user_id = ?", ev.UserID).
		Where("verified_at is NULL").
		Where("expires_at > ?", time.Now()).Count(&total)
	if res.Error != nil {
		return nil, res.Error
	}
	if total > 10 {
		return nil, fmt.Errorf("too many pending requests for email verification")
	}

	res = ur.db.Create(ev)
	if res.Error != nil {
		return nil, res.Error
	}
	return ev, nil
}

// Get finds record
func (ur *EmailVerificationRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*types.EmailVerification, error) {
	var ev types.EmailVerification
	res := qc.AddUserWhere(ur.db, true).Where("id = ? OR email_code = ?", id, id).First(&ev)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &ev, nil
}

// Delete deletes record
func (ur *EmailVerificationRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.AddUserWhere(ur.db, false).Where("id = ?", id).Delete(&types.EmailVerification{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete email verification with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// GetVerifiedEmails finds verified emails
func (ur *EmailVerificationRepositoryImpl) GetVerifiedEmails(
	qc *common.QueryContext,
	userID string,
) (emails map[string]bool) {
	emails = make(map[string]bool)
	recs := make([]*types.EmailVerification, 0)
	tx := qc.AddUserWhere(ur.db, true).Limit(100).
		Where("user_id = ?", userID).
		Where("verified_at is NOT NULL")
	res := tx.Find(&recs)
	if res.Error != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "EmailVerificationRepositoryImpl",
			"Method":    "GetVerifiedEmails",
			"Error":     res.Error,
		}).Warnf("failed to get verified emails")
		return emails
	}
	for _, rec := range recs {
		emails[strings.ToLower(rec.Email)] = true
	}
	return
}

// Verify performs email verification
func (ur *EmailVerificationRepositoryImpl) Verify(
	qc *common.QueryContext,
	userID string,
	code string) (rec *types.EmailVerification, err error) {
	res := qc.AddUserWhere(ur.db, false).Model(&types.EmailVerification{}).
		Where("user_id = ?", userID).
		Where("email_code = ?", code).
		Where("expires_at > ?", time.Now()).
		Where("verified_at is NULL").
		Updates(map[string]interface{}{"verified_at": time.Now()})
	if res.Error != nil {
		return nil, res.Error
	}
	updated := res.RowsAffected
	var ev types.EmailVerification
	res = ur.db.Where("email_code = ?", code).First(&ev)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if ev.VerifiedAt == nil && updated != 1 {
		return nil, common.NewNotFoundError("could not verify email")
	}
	return &ev, nil
}

// Query finds matching configs
func (ur *EmailVerificationRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*types.EmailVerification, totalRecords int64, err error) {
	recs = make([]*types.EmailVerification, 0)
	tx := qc.AddUserWhere(ur.db, true).Limit(pageSize).Offset(page * pageSize)
	tx = ur.addQuery(params, tx)
	if len(order) > 0 {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	} else {
		tx = tx.Order("created_at DESC")
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	totalRecords, _ = ur.Count(qc, params)
	return
}

// Count counts records by query
func (ur *EmailVerificationRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := qc.AddUserWhere(ur.db, true).Model(&types.EmailVerification{})
	tx = ur.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (ur *EmailVerificationRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("email LIKE ? OR email_code LIKE ? OR user_id LIKE ?",
			qs, qs, qs)
	}
	verified := params["verified"]
	if verified == "true" || verified == true {
		tx = tx.Where("verified_at is NOT NULL")
	} else if verified == "false" || verified == false {
		tx = tx.Where("verified_at is NULL")
	}
	return addQueryParamsWhere(filterParams(params, "q", "verified"), tx)
}