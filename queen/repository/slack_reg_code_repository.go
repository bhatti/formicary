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
// Uses a single conditional UPDATE (code=? AND used=false AND expires_at>now) so that
// only one concurrent caller can succeed — no TOCTOU race possible.
// Returns NotFoundError if the code does not exist, is already used, or is expired.
func (r *SlackRegCodeRepositoryImpl) Consume(code string) (*common.SlackRegCode, error) {
	now := time.Now()
	result := r.db.Model(&common.SlackRegCode{}).
		Where("code = ? AND used = ? AND expires_at > ?", code, false, now).
		Updates(map[string]interface{}{"used": true, "updated_at": now.UTC()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		// Distinguish: not found vs already used vs expired for a clear error message.
		var rec common.SlackRegCode
		if err := r.db.Where("code = ?", code).First(&rec).Error; err != nil {
			return nil, common.NewNotFoundError(fmt.Errorf("registration code not found"))
		}
		if rec.Used {
			return nil, common.NewValidationError(fmt.Errorf("registration code already used"))
		}
		return nil, common.NewValidationError(fmt.Errorf("registration code expired"))
	}
	var rec common.SlackRegCode
	if err := r.db.Where("code = ?", code).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// PurgeExpired removes expired codes to keep the table small.
func (r *SlackRegCodeRepositoryImpl) PurgeExpired() error {
	return r.db.Where("expires_at < ? OR used = ?", time.Now(), true).
		Delete(&common.SlackRegCode{}).Error
}
