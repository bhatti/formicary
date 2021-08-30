package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// EmailVerificationRepository defines data access methods for email verification
type EmailVerificationRepository interface {
	// Get finds record
	Get(
		qc *common.QueryContext,
		id string) (*types.EmailVerification, error)

	// Delete deletes record
	Delete(
		qc *common.QueryContext,
		id string) error

	// Create - creates new email verification
	Create(
		ev *types.EmailVerification) (*types.EmailVerification, error)

	GetVerifiedEmails(
		qc *common.QueryContext,
		userID string,
	) map[string]bool

	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*types.EmailVerification, totalRecords int64, err error)

	Verify(
		qc *common.QueryContext,
		userID string,
		code string) (*types.EmailVerification, error)
}
