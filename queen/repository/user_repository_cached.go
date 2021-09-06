package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

// UserRepositoryCached implements UserRepository with caching support
type UserRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    UserRepository
	cache      *ccache.Cache
}

// NewUserRepositoryCached creates new instance for user-repository
func NewUserRepositoryCached(
	serverConf *config.ServerConfig,
	adapter UserRepository) (UserRepository, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(1000))
	return &UserRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// Get method finds User by id
func (urc *UserRepositoryCached) Get(
	qc *common.QueryContext,
	id string) (*common.User, error) {
	item, err := urc.cache.Fetch("User:"+id+qc.String(),
		urc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return urc.adapter.Get(qc, id)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*common.User), nil
}

// GetByUsername method finds User by username
func (urc *UserRepositoryCached) GetByUsername(
	qc *common.QueryContext,
	username string) (*common.User, error) {
	item, err := urc.cache.Fetch("Username:"+username+qc.String(),
		urc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return urc.adapter.GetByUsername(qc, username)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*common.User), nil
}

// Delete org
func (urc *UserRepositoryCached) Delete(
	qc *common.QueryContext,
	id string) error {
	org, err := urc.Get(qc, id)
	if err != nil {
		return err
	}
	err = urc.adapter.Delete(qc, id)
	if err != nil {
		return err
	}
	urc.ClearCacheFor(id, org.Username)
	return nil
}

// Create persists org
func (urc *UserRepositoryCached) Create(
	org *common.User) (*common.User, error) {
	saved, err := urc.adapter.Create(org)
	if err != nil {
		return nil, err
	}
	urc.ClearCacheFor(saved.ID, saved.Username)
	return saved, nil
}

// Update persists org
func (urc *UserRepositoryCached) Update(
	qc *common.QueryContext,
	org *common.User) (*common.User, error) {
	saved, err := urc.adapter.Update(qc, org)
	if err != nil {
		return nil, err
	}
	urc.ClearCacheFor(saved.ID, saved.Username)
	return saved, nil
}

// Query finds matching configs
func (urc *UserRepositoryCached) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.User, totalRecords int64, err error) {
	return urc.adapter.Query(qc, params, page, pageSize, order)
}

// Count counts records by query
func (urc *UserRepositoryCached) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	return urc.adapter.Count(qc, params)
}

// GetSession returns session
func (urc *UserRepositoryCached) GetSession(
	sessionID string) (*types.UserSession, error) {
	return urc.adapter.GetSession(sessionID)
}

// AddSession adds session
func (urc *UserRepositoryCached) AddSession(
	session *types.UserSession) error {
	return urc.adapter.AddSession(session)
}

// UpdateSession updates session
func (urc *UserRepositoryCached) UpdateSession(
	session *types.UserSession) error {
	return urc.adapter.UpdateSession(session)
}

// AddToken adds token
func (urc *UserRepositoryCached) AddToken(
	token *types.UserToken) error {
	return urc.adapter.AddToken(token)
}

// RevokeToken removes token
func (urc *UserRepositoryCached) RevokeToken(
	qc *common.QueryContext,
	userID string,
	id string) error {
	return urc.adapter.RevokeToken(qc, userID, id)
}

// HasToken validates token
func (urc *UserRepositoryCached) HasToken(
	userID string, tokenName string, sha256 string) bool {
	return urc.adapter.HasToken(userID, tokenName, sha256)
}

// GetTokens - returns tokens for user
func (urc *UserRepositoryCached) GetTokens(
	qc *common.QueryContext,
	userID string) ([]*types.UserToken, error) {
	return urc.adapter.GetTokens(qc, userID)
}

// Clear clears cache
func (urc *UserRepositoryCached) Clear() {
	urc.cache.DeletePrefix("User")
	urc.cache.DeletePrefix("Username")
	urc.adapter.Clear()
}

// ClearCacheFor - clears cache
func (urc *UserRepositoryCached) ClearCacheFor(
	id string,
	username string) {
	if id != "" {
		urc.cache.DeletePrefix("User:" + id)
	}
	if username != "" {
		urc.cache.DeletePrefix("Username:" + username)
	}
}
