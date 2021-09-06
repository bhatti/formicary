package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// InvitationRepository defines data access methods for invitations
type InvitationRepository interface {
	// Query - queries invitation
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*types.UserInvitation, totalRecords int64, err error)
	// Count counts
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)

	Create(invitation *types.UserInvitation) error

	// Get finds record - called internally so no query-context
	Get(
		id string) (*types.UserInvitation, error)

	// Delete deletes record - called internally so no query-context
	Delete(
		id string) error

	Find(email string, code string) (*types.UserInvitation, error)

	Accept(email string, code string) (*types.UserInvitation, error)

	Clear()
}
