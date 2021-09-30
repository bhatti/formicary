package repository

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/crypto"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"

	"github.com/twinj/uuid"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// OrganizationRepositoryImpl implements OrganizationRepository using gorm O/R mapping
type OrganizationRepositoryImpl struct {
	dbConfig *config.DBConfig
	db       *gorm.DB
	*BaseRepositoryImpl
}

// NewOrganizationRepositoryImpl creates new instance for org-repository
func NewOrganizationRepositoryImpl(
	dbConfig *config.DBConfig,
	db *gorm.DB,
	objectUpdatedHandler ObjectUpdatedHandler,
) (*OrganizationRepositoryImpl, error) {
	return &OrganizationRepositoryImpl{
			dbConfig:           dbConfig,
			db:                 db,
			BaseRepositoryImpl: NewBaseRepositoryImpl(objectUpdatedHandler),
		},
		nil
}

// Get method finds Organization by id
func (orc *OrganizationRepositoryImpl) Get(
	_ *common.QueryContext,
	id string) (*common.Organization, error) {
	var org common.Organization
	now := time.Now()
	res := orc.db.Where("id = ?", id).
		Where("active = ?", true).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Preload("Configs").
		First(&org)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := org.AfterLoad(orc.encryptionKey(&org)); err != nil {
		return nil, err
	}
	return &org, nil
}

// GetByUnit method finds Organization by unit
func (orc *OrganizationRepositoryImpl) GetByUnit(
	_ *common.QueryContext,
	unit string) (*common.Organization, error) {
	now := time.Now()
	var org common.Organization
	res := orc.db.Where("org_unit = ?", unit).
		Where("active = ?", true).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Preload("Configs").
		First(&org)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := org.AfterLoad(orc.encryptionKey(&org)); err != nil {
		return nil, err
	}
	return &org, nil
}

// lookupOrg method finds Organization by unit
func (orc *OrganizationRepositoryImpl) lookupOrg(
	_ *common.QueryContext,
	unit string) (*common.Organization, error) {
	var org common.Organization
	res := orc.db.Where("org_unit = ?", unit).
		First(&org)
	if res.Error != nil {
		return nil, res.Error
	}
	if err := org.AfterLoad(orc.encryptionKey(&org)); err != nil {
		return nil, err
	}
	return &org, nil
}

// lookupBundle method finds Organization by unit
func (orc *OrganizationRepositoryImpl) lookupBundle(
	bundle string, unit string) (total int64) {
	_ = orc.db.Model(&common.Organization{}).
		Where("org_unit != ?", unit).
		Where("bundle_id = ?", bundle).
		Count(&total)
	return
}

// GetByParentID method finds Organization by parent-id
func (orc *OrganizationRepositoryImpl) GetByParentID(
	_ *common.QueryContext,
	parentID string) (recs []*common.Organization, err error) {
	now := time.Now()
	recs = make([]*common.Organization, 0)
	res := orc.db.Limit(100).
		Where("parent_id = ?", parentID).
		Where("active = ?", true).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Preload("Configs").
		Find(&recs)
	if res.Error != nil {
		return nil, res.Error
	}
	for _, org := range recs {
		if err := org.AfterLoad(orc.encryptionKey(org)); err != nil {
			return nil, err
		}
	}
	return
}

// Clear - for testing
func (orc *OrganizationRepositoryImpl) Clear() {
	clearDB(orc.db)
}

// Delete org
func (orc *OrganizationRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) (err error) {
	err = orc.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&common.Organization{}).
			Where("id = ?", id).
			Where("active = ?", true).
			Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to delete org with id %v, rows %v", id, res.RowsAffected))
		}
		res = tx.Model(&common.User{}).
			Where("organization_id = ?", id).
			Where("active = ?", true).
			Updates(map[string]interface{}{
				"active":          false,
				"name":            "",
				"organization_id": "",
				"bundle_id":       "",
				"updated_at":      time.Now()})
		if res.Error != nil {
			return res.Error
		}
		logrus.WithFields(logrus.Fields{
			"Component":         "OrganizationRepositoryImpl",
			"Org":               id,
			"UsersRowsAffected": res.RowsAffected,
		}).Warnf("removing org and users")
		return nil
	})
	if err == nil {
		orc.FireObjectUpdatedHandler(qc, id, ObjectDeleted, nil)
	}
	return
}

// Create persists org
func (orc *OrganizationRepositoryImpl) Create(
	qc *common.QueryContext,
	org *common.Organization) (*common.Organization, error) {
	err := org.ValidateBeforeSave(orc.encryptionKey(org))
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	old, _ := orc.lookupOrg(qc, org.OrgUnit)
	if old != nil {
		return nil, common.NewDuplicateError(
			fmt.Errorf("organization %s already exists", org.OrgUnit))
	}
	if orc.lookupBundle(org.BundleID, org.OrgUnit) > 0 {
		return nil, common.NewDuplicateError(
			fmt.Errorf("organization bundle %s already exists", org.BundleID))
	}
	err = orc.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		org.Active = true
		org.ID = uuid.NewV4().String()
		org.CreatedAt = time.Now()
		org.UpdatedAt = time.Now()
		for _, cfg := range org.Configs {
			cfg.OrganizationID = org.ID
		}
		res = tx.Create(org)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return org, err
}

// UpdateStickyMessage updates sticky message for user and org
func (orc *OrganizationRepositoryImpl) UpdateStickyMessage(
	qc *common.QueryContext,
	user *common.User,
) (err error) {
	err = orc.db.Transaction(func(tx *gorm.DB) error {
		if user != nil {
			res := tx.Exec("update formicary_users set sticky_message = ? where id = ?", user.StickyMessage, user.ID)
			if res.Error != nil {
				return fmt.Errorf("fail to set sticky message '%s' for user '%s' due to '%s'",
					user.StickyMessage, user.ID, res.Error)
			}
			orc.FireObjectUpdatedHandler(qc, user.ID, ObjectUpdated, user)
		}
		if user.HasOrganization() {
			res := tx.Exec("update formicary_orgs set sticky_message = ? where id = ?",
				user.Organization.StickyMessage, user.OrganizationID)
			if res.Error != nil {
				return fmt.Errorf("fail to set sticky message '%s' for org '%s' due to '%s'",
					user.Organization.StickyMessage, user.OrganizationID, res.Error)
			}
			orc.FireObjectUpdatedHandler(qc, user.OrganizationID, ObjectUpdated, user.Organization)
		}
		return nil
	})
	return
}

// Update persists org
func (orc *OrganizationRepositoryImpl) Update(
	qc *common.QueryContext,
	org *common.Organization) (*common.Organization, error) {
	err := org.ValidateBeforeSave(orc.encryptionKey(org))
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	old, _ := orc.lookupOrg(qc, org.OrgUnit)
	if old == nil {
		return nil, common.NewNotFoundError(
			fmt.Errorf("organization %s does not exists", org.OrgUnit))
	}
	if !qc.Matches(old.OwnerUserID, old.ID, false) {
		return nil, common.NewPermissionError(
			fmt.Errorf("organization '%s' with id '%s' cannot be edited by non-member %s",
				org.OrgUnit, old.ID, qc.GetOrganizationID()))
	}
	err = orc.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		if org.OrgUnit != "" {
			old.OrgUnit = org.OrgUnit
		}
		if org.BundleID != "" {
			old.BundleID = org.BundleID
		}
		old.Active = true
		old.UpdatedAt = time.Now()
		res = tx.Save(old)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err == nil {
		orc.FireObjectUpdatedHandler(qc, org.ID, ObjectUpdated, org)
	}
	return old, err
}

// AddInvitation adds invitation
func (orc *OrganizationRepositoryImpl) AddInvitation(invitation *types.UserInvitation) error {
	err := invitation.ValidateBeforeSave()
	if err != nil {
		return err
	}
	var total int64
	_ = orc.db.Model(&types.UserInvitation{}).
		Where("invitation_code = ?", invitation.InvitationCode).
		Count(&total)
	if total > 0 {
		return common.NewDuplicateError(
			fmt.Errorf("invitation code %s already exists", invitation.InvitationCode))
	}

	_ = orc.db.Model(&common.User{}).
		Where("email = ?", invitation.Email).
		Count(&total)
	if total > 0 {
		return common.NewDuplicateError(
			fmt.Errorf("user with email %s already exists", invitation.Email))
	}
	res := orc.db.Model(&types.UserInvitation{}).
		Where("organization_id = ?", invitation.OrganizationID).
		Where("accepted_at is NULL").
		Where("expires_at > ?", time.Now()).Count(&total)
	if res.Error != nil {
		return res.Error
	}
	if total > 100 {
		return fmt.Errorf("too many pending invitations")
	}

	return orc.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		res = tx.Create(invitation)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

// GetInvitation finds invitation
func (orc *OrganizationRepositoryImpl) GetInvitation(id string) (*types.UserInvitation, error) {
	var invitation types.UserInvitation
	res := orc.db.
		Where("id = ?", id).
		First(&invitation)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &invitation, nil
}

// FindInvitation finds invitation
func (orc *OrganizationRepositoryImpl) FindInvitation(email string, code string) (*types.UserInvitation, error) {
	var invitation types.UserInvitation
	res := orc.db.
		Where("email = ?", email).
		Where("invitation_code = ?", code).
		First(&invitation)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	return &invitation, nil
}

// AcceptInvitation accepts invitation
func (orc *OrganizationRepositoryImpl) AcceptInvitation(email string, code string) (*types.UserInvitation, error) {
	res := orc.db.Model(&types.UserInvitation{}).
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
	return orc.FindInvitation(email, code)
}

// Query finds matching configs
func (orc *OrganizationRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.Organization, totalRecords int64, err error) {
	now := time.Now()
	recs = make([]*common.Organization, 0)
	tx := orc.db.Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true).
		Preload("Subscription", "active = ? AND started_at <= ? AND ended_at >= ?", true, now, now).
		Preload("Configs")
	tx = orc.addQuery(params, tx)

	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&recs)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	totalRecords, _ = orc.Count(qc, params)
	for _, org := range recs {
		if err := org.AfterLoad(orc.encryptionKey(org)); err != nil {
			return nil, 0, err
		}
	}
	return
}

// Count counts records by query
func (orc *OrganizationRepositoryImpl) Count(
	_ *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := orc.db.Model(&common.Organization{}).Where("active = ?", true)
	tx = orc.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (orc *OrganizationRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("bundle_id LIKE ? OR owner_user_id LIKE ? OR id LIKE ? OR org_unit LIKE ? OR sticky_message LIKE ?",
			qs, qs, qs, qs, qs)
	}
	return addQueryParamsWhere(filterParams(params, "q", "verified"), tx)
}

func (orc *OrganizationRepositoryImpl) encryptionKey(
	org *common.Organization) []byte {
	if org == nil || orc.dbConfig.EncryptionKey == "" {
		return nil
	}
	return crypto.SHA256Key(orc.dbConfig.EncryptionKey + org.Salt)
}
