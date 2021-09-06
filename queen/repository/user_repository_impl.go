package repository

import (
	"fmt"
	"runtime/debug"
	"time"

	common "plexobject.com/formicary/internal/types"

	"github.com/twinj/uuid"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

const maxTokens = 10

// UserRepositoryImpl implements UserRepository using gorm O/R mapping
type UserRepositoryImpl struct {
	db *gorm.DB
}

// NewUserRepositoryImpl creates new instance for user-repository
func NewUserRepositoryImpl(
	db *gorm.DB) (*UserRepositoryImpl, error) {
	return &UserRepositoryImpl{db: db}, nil
}

// Get method finds User by id
func (ur *UserRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.User, error) {
	var user common.User
	now := time.Now()
	res := qc.WithUserIDColumn("id").AddOrgElseUserWhere(ur.db).Where("id = ?", id).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Where("active = ?", true).
		First(&user)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := user.AfterLoad(); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername method finds User by username
func (ur *UserRepositoryImpl) GetByUsername(
	qc *common.QueryContext,
	username string) (*common.User, error) {
	now := time.Now()
	var user common.User
	res := qc.WithUserIDColumn("id").AddOrgElseUserWhere(ur.db).Where("username = ?", username).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Where("active = ?", true).
		First(&user)
	if res.Error != nil {
		debug.PrintStack()
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := user.AfterLoad(); err != nil {
		return nil, err
	}
	return &user, nil
}

// lookupUsername method finds User by username
func (ur *UserRepositoryImpl) lookupUsername(
	username string) (*common.User, error) {
	var user common.User
	res := ur.db.Where("username = ?", username).
		First(&user)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := user.AfterLoad(); err != nil {
		return nil, err
	}
	return &user, nil
}

// Clear - for testing
func (ur *UserRepositoryImpl) Clear() {
	clearDB(ur.db)
}

// Delete user
func (ur *UserRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	res := qc.WithUserIDColumn("id").AddOrgElseUserWhere(ur.db.Model(&common.User{})).
		Where("id = ?", id).
		Where("active = ?", true).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete user with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Create persists user
func (ur *UserRepositoryImpl) Create(
	user *common.User) (*common.User, error) {
	err := user.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	old, _ := ur.lookupUsername(user.Username)
	if old != nil {
		return nil, common.NewDuplicateError(
			fmt.Errorf("username %s already exists", user.Username))
	}
	err = ur.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		user.Active = true
		user.ID = uuid.NewV4().String()
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()
		res = tx.Create(user)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return user, err
}

// Update persists user
func (ur *UserRepositoryImpl) Update(
	qc *common.QueryContext,
	user *common.User) (*common.User, error) {
	err := user.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	old, _ := ur.lookupUsername(user.Username)
	if old == nil {
		return nil, common.NewNotFoundError(
			fmt.Errorf("username %s does not exists", user.Username))
	}
	if !qc.Admin() && !qc.Matches(old.ID, old.OrganizationID) {
		return nil, common.NewPermissionError(
			fmt.Errorf("user '%s' with id '%s' cannot be edited by another '%s'", user.Username, old.ID, qc.UserID))
	}
	err = ur.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		old.Active = true
		if user.Name != "" {
			old.Name = user.Name
		}
		if user.PictureURL != "" {
			old.PictureURL = user.PictureURL
		}
		if user.URL != "" {
			old.URL = user.URL
		}
		if user.Email != "" {
			if old.Email != user.Email {
				user.EmailVerified = false
			}
			old.Email = user.Email
		}
		if user.AuthProvider != "" {
			old.AuthProvider = user.AuthProvider
		}
		if user.AuthID != "" {
			old.AuthID = user.AuthID
		}
		old.OrganizationID = qc.OrganizationID
		if user.BundleID != "" {
			old.BundleID = user.BundleID
		}
		if user.NotifySerialized != "" {
			old.NotifySerialized = user.NotifySerialized
		}
		if user.EmailVerified {
			old.EmailVerified = true
		}
		old.UpdatedAt = time.Now()
		res = tx.Save(old)
		if res.Error != nil {
			return res.Error
		}
		return old.AfterLoad()
	})
	return old, err
}

// AddSession adds session
func (ur *UserRepositoryImpl) AddSession(
	session *types.UserSession) error {
	err := session.Validate()
	if err != nil {
		return common.NewValidationError(err)
	}
	return ur.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		session.ID = uuid.NewV4().String()
		session.CreatedAt = time.Now()
		session.UpdatedAt = time.Now()
		res = tx.Create(session)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

// GetSession returns session
func (ur *UserRepositoryImpl) GetSession(
	sessionID string) (*types.UserSession, error) {
	var old types.UserSession
	res := ur.db.Where("session_id = ?", sessionID).
		First(&old)
	if res.Error != nil {
		return nil, common.NewNotFoundError(
			fmt.Errorf("could not find old session %s due to %s", sessionID, res.Error.Error()))
	}
	return &old, nil
}

// UpdateSession update session
func (ur *UserRepositoryImpl) UpdateSession(
	session *types.UserSession) error {
	err := session.Validate()
	if err != nil {
		return common.NewValidationError(err)
	}
	old, err := ur.GetSession(session.SessionID)
	if err != nil {
		return err
	}
	return ur.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		if session.PictureURL != "" {
			old.PictureURL = session.PictureURL
		}
		if session.AuthProvider != "" {
			old.AuthProvider = session.AuthProvider
		}
		if session.UserID != "" {
			old.UserID = session.UserID
		}
		if session.Username != "" {
			old.Username = session.Username
		}
		if session.Email != "" {
			old.Email = session.Email
		}
		old.UpdatedAt = time.Now()
		res = tx.Save(old)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

// AddToken adds token
func (ur *UserRepositoryImpl) AddToken(
	token *types.UserToken) error {
	err := token.Validate()
	if err != nil {
		return common.NewValidationError(err)
	}
	total := ur.countToken(token.UserID)
	if total >= maxTokens {
		return common.NewValidationError(
			fmt.Errorf("you already have %d API tokens, please revoke some of existing token before creating new one", total))
	}
	return ur.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		token.Active = true
		token.ID = uuid.NewV4().String()
		token.CreatedAt = time.Now()
		res = tx.Create(token)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

// RevokeToken removes token
func (ur *UserRepositoryImpl) RevokeToken(
	qc *common.QueryContext,
	userID string,
	id string) error {
	res := qc.WithUserIDColumn("user_id").AddUserWhere(ur.db.Model(&types.UserToken{})).
		Where("user_id = ?", userID).
		Where("id = ?", id).
		Where("active = ?", true).
		Updates(map[string]interface{}{"active": false})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to revoke token with id %s by user %s, rows %v",
				id, userID, res.RowsAffected))
	}
	return nil
}

// GetTokens returns tokens for the user
func (ur *UserRepositoryImpl) GetTokens(
	qc *common.QueryContext,
	userID string) ([]*types.UserToken, error) {
	recs := make([]*types.UserToken, 0)
	tx := qc.WithUserIDColumn("user_id").AddUserWhere(ur.db).
		Where("user_id = ?", userID).
		Where("expires_at > ?", time.Now()).
		Where("active = ?", true)
	res := tx.Find(&recs)
	if res.Error != nil {
		return nil, res.Error
	}
	return recs, nil
}

// HasToken validates token
func (ur *UserRepositoryImpl) HasToken(
	userID string, tokenName string, sha256 string) bool {
	tx := ur.db.Model(&types.UserToken{}).
		Where("user_id = ?", userID).
		Where("token_name = ?", tokenName).
		Where("sha256 = ?", sha256).
		Where("expires_at > ?", time.Now()).
		Where("active = ?", true)
	var totalRecords int64
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		return false
	}
	return totalRecords > 0
}

// Query finds matching configs
func (ur *UserRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.User, totalRecords int64, err error) {
	recs = make([]*common.User, 0)
	tx := qc.WithUserIDColumn("id").AddOrgElseUserWhere(ur.db).Limit(pageSize).
		Offset(page*pageSize).Where("active = ?", true)
	tx = ur.addQuery(params, tx)
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	for _, rec := range recs {
		_ = rec.AfterLoad()
	}
	totalRecords, _ = ur.Count(qc, params)
	return
}

// Count counts records by query
func (ur *UserRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := qc.WithUserIDColumn("id").AddOrgElseUserWhere(ur.db.Model(&common.User{})).Where("active = ?", true)
	tx = ur.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (ur *UserRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("name LIKE ? OR username LIKE ? OR email LIKE ? OR sticky_message LIKE ?",
			qs, qs, qs, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}

// countToken validates token
func (ur *UserRepositoryImpl) countToken(
	userID string) int64 {
	tx := ur.db.Model(&types.UserToken{}).
		Where("user_id = ?", userID).
		Where("expires_at > ?", time.Now()).
		Where("active = ?", true)
	var totalRecords int64
	_ = tx.Count(&totalRecords)
	return totalRecords
}
