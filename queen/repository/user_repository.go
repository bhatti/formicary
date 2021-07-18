package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// UserRepository defines data access methods for users
type UserRepository interface {
	// Get - finds User
	Get(
		qc *common.QueryContext,
		id string) (*common.User, error)
	// GetByUsername - finds User
	GetByUsername(
		qc *common.QueryContext,
		username string) (*common.User, error)
	// Delete User
	Delete(
		qc *common.QueryContext,
		id string) error
	// Create - creates new user
	Create(user *common.User) (*common.User, error)
	// Update - update new user
	Update(
		qc *common.QueryContext,
		user *common.User) (*common.User, error)
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.User, totalRecords int64, err error)
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)

	AddSession(session *types.UserSession) error

	GetSession(sessionID string) (*types.UserSession, error)

	UpdateSession(session *types.UserSession) error

	AddToken(token *types.UserToken) error

	GetTokens(
		qc *common.QueryContext,
		userID string) ([]*types.UserToken, error)

	RevokeToken(
		qc *common.QueryContext,
		userID string,
		id string) error

	HasToken(userID string, tokenName string, sha256 string) bool

	// for testing
	Clear()
}
