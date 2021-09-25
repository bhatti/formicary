package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
	"sync"
)

// UserRepositoryCached implements UserRepository with caching support
type UserRepositoryCached struct {
	serverConf     *config.ServerConfig
	adapter        UserRepository
	cache          *ccache.Cache
	lock           sync.RWMutex
	orgIDToUserIDs map[string][]string
}

// NewUserRepositoryCached creates new instance for user-repository
func NewUserRepositoryCached(
	serverConf *config.ServerConfig,
	adapter UserRepository) (*UserRepositoryCached, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(1000))
	return &UserRepositoryCached{
		adapter:        adapter,
		serverConf:     serverConf,
		cache:          cache,
		orgIDToUserIDs: make(map[string][]string),
	}, nil
}

// Get method finds User by id
func (urc *UserRepositoryCached) Get(
	qc *common.QueryContext,
	id string) (user *common.User, err error) {
	item, err := urc.cache.Fetch("User:"+id+qc.String(),
		urc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return urc.adapter.Get(qc, id)
		})
	if err != nil {
		return nil, err
	}
	user = item.Value().(*common.User)
	urc.addUserOrgMapping(user)
	return
}

// GetByUsername method finds User by username
func (urc *UserRepositoryCached) GetByUsername(
	qc *common.QueryContext,
	username string) (user *common.User, err error) {
	item, err := urc.cache.Fetch("Username:"+username+qc.String(),
		urc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return urc.adapter.GetByUsername(qc, username)
		})
	if err != nil {
		return nil, err
	}
	user = item.Value().(*common.User)
	urc.addUserOrgMapping(user)
	return
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
	org *common.User) (saved *common.User, err error) {
	saved, err = urc.adapter.Create(org)
	if err != nil {
		return nil, err
	}
	urc.ClearCacheFor(saved.ID, saved.Username)
	urc.addUserOrgMapping(saved)
	return
}

// Update persists org
func (urc *UserRepositoryCached) Update(
	qc *common.QueryContext,
	org *common.User) (saved *common.User, err error) {
	saved, err = urc.adapter.Update(qc, org)
	if err != nil {
		return nil, err
	}
	urc.ClearCacheFor(saved.ID, saved.Username)
	urc.addUserOrgMapping(saved)
	return
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

// ClearCacheForOrg - clears cache for org
func (urc *UserRepositoryCached) ClearCacheForOrg(
	orgID string,
) {
	if orgID != "" {
		userids := urc.getUserOrgMapping(orgID)
		for _, id := range userids {
			urc.cache.DeletePrefix("User:" + id)
		}
	}
}

func (urc *UserRepositoryCached) addUserOrgMapping(user *common.User) {
	if user != nil {
		urc.addUserIDOrgMapping(user.ID, user.OrganizationID)
	}
}

func (urc *UserRepositoryCached) addUserIDOrgMapping(userID string, orgID string) {
	if userID != "" && orgID != "" {
		urc.lock.Lock()
		defer urc.lock.Unlock()
		userids := append(urc.orgIDToUserIDs[orgID], userID)
		urc.orgIDToUserIDs[orgID] = userids
	}
}

func (urc *UserRepositoryCached) getUserOrgMapping(orgID string) (userids []string) {
	if orgID != "" {
		urc.lock.RLock()
		defer urc.lock.RUnlock()
		userids = urc.orgIDToUserIDs[orgID]
	}
	if userids == nil {
		userids = make([]string, 0)
	}
	return
}
