package repository

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/oklog/ulid/v2"
	"time"

	"gorm.io/gorm"
	common "plexobject.com/formicary/internal/types"
)

// SubscriptionRepositoryImpl implements SubscriptionRepository using gorm O/R mapping
type SubscriptionRepositoryImpl struct {
	db *gorm.DB
	*BaseRepositoryImpl
}

// NewSubscriptionRepositoryImpl creates new instance for subscription-repository
func NewSubscriptionRepositoryImpl(
	db *gorm.DB,
	objectUpdatedHandler ObjectUpdatedHandler,
) (*SubscriptionRepositoryImpl, error) {
	return &SubscriptionRepositoryImpl{
		db:                 db,
		BaseRepositoryImpl: NewBaseRepositoryImpl(objectUpdatedHandler),
	}, nil
}

// Query finds matching subscriptions by parameters
func (sr *SubscriptionRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*common.Subscription, total int64, err error) {
	records = make([]*common.Subscription, 0)
	tx := qc.AddOrgElseUserWhere(sr.db.Model(&common.Subscription{}), true).
		Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true)
	tx = addQueryParamsWhere(params, tx)
	if len(order) == 0 {
		tx = tx.Order("created_at desc")
	} else {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	}
	// See tx.Statement for debugging
	res := tx.Find(&records)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	for _, rec := range records {
		rec.LoadedAt = time.Now()
	}
	total, _ = sr.Count(qc, params)
	return
}

// Count counts subscriptions by query
func (sr *SubscriptionRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (total int64, err error) {
	tx := qc.AddOrgElseUserWhere(sr.db, true).Where("active = ?", true)
	tx = addQueryParamsWhere(params, tx)
	res := tx.Model(&common.Subscription{}).Count(&total)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

// Clear - for testing
func (sr *SubscriptionRepositoryImpl) Clear() {
	clearDB(sr.db)
}

// Get method finds subscription by id
func (sr *SubscriptionRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*common.Subscription, error) {
	var subscription common.Subscription
	res := qc.AddOrgElseUserWhere(sr.db, true).
		Where("id = ?", id).
		Where("active = ?", true).
		First(&subscription)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	subscription.LoadedAt = time.Now()
	return &subscription, nil
}

// Delete subscription
func (sr *SubscriptionRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	tx := sr.db.Model(&common.Subscription{}).
		Where("id = ?", id).
		Where("active = ?", true)
	tx = qc.AddOrgElseUserWhere(tx, false)
	res := tx.Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete subscription with id %v, rows %v", id, res.RowsAffected))
	}
	sr.FireObjectUpdatedHandler(qc, id, ObjectDeleted, nil)
	return nil
}

// Create persists subscription
func (sr *SubscriptionRepositoryImpl) Create(
	qc *common.QueryContext,
	subscription *common.Subscription) (*common.Subscription, error) {
	err := subscription.Validate()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	subscription.ID = ulid.Make().String()
	err = sr.db.Transaction(func(tx *gorm.DB) error {
		// only one subscription can be active
		res := tx.Model(subscription).
			Where("active = ? AND id != ? AND (user_id = ? || organization_id = ?)", true, subscription.ID, subscription.UserID, subscription.OrganizationID).
			Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
		if res.Error != nil {
			logrus.WithFields(logrus.Fields{
				"Component":    "SubscriptionRepositoryImpl",
				"Subscription": subscription,
				"Error":        res.Error,
			}).Warnf("failed to reset old subscriptions")
		}
		subscription.CreatedAt = time.Now()
		subscription.UpdatedAt = time.Now()
		subscription.Active = true
		res = tx.Create(subscription)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err == nil {
		sr.FireObjectUpdatedHandler(qc, subscription.ID, ObjectUpdated, subscription)
	}
	return subscription, err
}

// Update persists subscription
func (sr *SubscriptionRepositoryImpl) Update(
	qc *common.QueryContext,
	subscription *common.Subscription) (*common.Subscription, error) {
	err := subscription.Validate()
	if err != nil {
		return nil, common.NewValidationError(err)
	}

	if !qc.Matches(subscription.UserID, subscription.OrganizationID, false) {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionRepositoryImpl",
			"Subscription": subscription,
			"QC":           qc,
		}).Warnf("subscription owner %s / %s didn't match query context %s",
			subscription.UserID, subscription.OrganizationID, qc)
		return nil, common.NewPermissionError(
			fmt.Errorf("subscription owner didn't match"))
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionRepositoryImpl",
			"Subscription": subscription,
			"QC":           qc,
		}).Debugf("updating subscription")
	}

	subscription.Active = true
	err = sr.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		subscription.UpdatedAt = time.Now()
		res = tx.Save(subscription)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err == nil {
		sr.FireObjectUpdatedHandler(qc, subscription.ID, ObjectUpdated, subscription)
	}
	return subscription, err
}
