// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	common "plexobject.com/formicary/internal/types"
)

// SlackRegCodeRepository manages one-time registration codes.
type SlackRegCodeRepository interface {
	// Create stores a new code.
	Create(qc *common.QueryContext, code *common.SlackRegCode) error
	// Consume atomically marks the code used and returns it, or returns an error
	// if the code is missing, already used, or expired.
	Consume(code string) (*common.SlackRegCode, error)
	// PurgeExpired deletes codes that have passed their expiry.
	PurgeExpired() error
}

// SlackRegCodeRepositoryImpl implements SlackRegCodeRepository via GORM.
type SlackRegCodeRepositoryImpl struct {
	db *gorm.DB
}

// NewSlackRegCodeRepositoryImpl creates a new repository instance.
func NewSlackRegCodeRepositoryImpl(db *gorm.DB) (*SlackRegCodeRepositoryImpl, error) {
	return &SlackRegCodeRepositoryImpl{db: db}, nil
}

// Create inserts a new registration code.
func (r *SlackRegCodeRepositoryImpl) Create(_ *common.QueryContext, code *common.SlackRegCode) error {
	if code.Code == "" {
		return common.NewValidationError(fmt.Errorf("code is required"))
	}
	if code.UserID == "" {
		return common.NewValidationError(fmt.Errorf("user_id is required"))
	}
	if code.ExpiresAt.IsZero() {
		return common.NewValidationError(fmt.Errorf("expires_at is required"))
	}
	return r.db.Create(code).Error
}

// Consume atomically validates and marks the code as used.
// Returns NotFoundError if the code does not exist, is already used, or is expired.
func (r *SlackRegCodeRepositoryImpl) Consume(code string) (*common.SlackRegCode, error) {
	var rec common.SlackRegCode
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("code = ?", code).First(&rec)
		if res.Error != nil {
			return common.NewNotFoundError(fmt.Errorf("registration code not found"))
		}
		if rec.Used {
			return common.NewValidationError(fmt.Errorf("registration code already used"))
		}
		if time.Now().After(rec.ExpiresAt) {
			return common.NewValidationError(fmt.Errorf("registration code expired"))
		}
		return tx.Model(&rec).Update("used", true).Error
	})
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// PurgeExpired removes expired codes to keep the table small.
func (r *SlackRegCodeRepositoryImpl) PurgeExpired() error {
	return r.db.Where("expires_at < ? OR used = ?", time.Now(), true).
		Delete(&common.SlackRegCode{}).Error
}
