package repository

import (
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"time"

	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// InvitationRepositoryImpl implements InvitationRepository using gorm O/R mapping
type InvitationRepositoryImpl struct {
	dbConfig *config.DBConfig
	db       *gorm.DB
}

// NewInvitationRepositoryImpl creates new instance for org-repository
func NewInvitationRepositoryImpl(
	dbConfig *config.DBConfig,
	db *gorm.DB) (*InvitationRepositoryImpl, error) {
	return &InvitationRepositoryImpl{
			dbConfig: dbConfig,
			db:       db},
		nil
}

// Clear - for testing
func (r *InvitationRepositoryImpl) Clear() {
	clearDB(r.db)
}

// Create adds invitation
func (r *InvitationRepositoryImpl) Create(invitation *types.UserInvitation) error {
	err := invitation.ValidateBeforeSave()
	if err != nil {
		return err
	}
	var total int64
	_ = r.db.Model(&types.UserInvitation{}).
		Where("invitation_code = ?", invitation.InvitationCode).
		Count(&total)
	if total > 0 {
		return common.NewDuplicateError(
			fmt.Errorf("invitation code %s already exists", invitation.InvitationCode))
	}

	// disabling following check because it can reveal users
	//_ = r.db.Model(&common.User{}).
	//	Where("email = ?", invitation.Email).
	//	Count(&total)
	//if total > 0 {
	//	return common.NewDuplicateError(
	//		fmt.Errorf("user with email %s already exists", invitation.Email))
	//}
	res := r.db.Model(&types.UserInvitation{}).
		Where("organization_id = ?", invitation.OrganizationID).
		Where("accepted_at is NULL").
		Where("expires_at > ?", time.Now()).Count(&total)
	if res.Error != nil {
		return res.Error
	}
	if total > 100 {
		return fmt.Errorf("too many pending invitations")
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		res = tx.Create(invitation)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

// Get finds invitation - called internally so no query-context
func (r *InvitationRepositoryImpl) Get(
	id string) (*types.UserInvitation, error) {
	var invitation types.UserInvitation
	res := r.db.Where("id = ? OR invitation_code = ?", id, id).
		First(&invitation)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &invitation, nil
}

// Delete deletes record - called internally so no query-context
func (r *InvitationRepositoryImpl) Delete(
	id string) error {
	res := r.db.Where("id = ?", id).Delete(&types.UserInvitation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete user invitation with id %v, rows %v", id, res.RowsAffected))
	}
	return nil
}

// Find finds invitation
func (r *InvitationRepositoryImpl) Find(email string, code string) (*types.UserInvitation, error) {
	var invitation types.UserInvitation
	res := r.db.
		Where("email = ?", email).
		Where("invitation_code = ?", code).
		First(&invitation)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &invitation, nil
}

// Accept accepts invitation
func (r *InvitationRepositoryImpl) Accept(email string, code string) (*types.UserInvitation, error) {
	res := r.db.Model(&types.UserInvitation{}).
		Where("email = ?", email).
		Where("invitation_code = ?", code).
		Where("expires_at >= ?", time.Now()).
		Where("accepted_at is null").
		Updates(map[string]interface{}{"accepted_at": time.Now()})
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return nil, common.NewNotFoundError(
			fmt.Errorf("failed to accept invitation rows %v", res.RowsAffected))
	}
	return r.Find(email, code)
}

// Query finds matching configs
func (r *InvitationRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*types.UserInvitation, totalRecords int64, err error) {
	recs = make([]*types.UserInvitation, 0)
	tx := qc.AddOrgWhere(r.db).Limit(pageSize).Offset(page * pageSize)
	tx = r.addQuery(params, tx)

	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	return
}

// Count counts records by query
func (r *InvitationRepositoryImpl) Count(
	_ *common.QueryContext, // TODO fix
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := r.db.Model(&types.UserInvitation{})
	tx = r.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (r *InvitationRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	accepted := params["accepted"]
	if accepted == "true" || accepted == true {
		tx = tx.Where("accepted_at is NOT NULL")
	} else if accepted == "false" || accepted == false {
		tx = tx.Where("accepted_at is NULL")
	}
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("email LIKE ? OR organization_id LIKE ? OR invited_by_user_id LIKE ?",
			qs, qs, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q", "accepted"), tx)
}
