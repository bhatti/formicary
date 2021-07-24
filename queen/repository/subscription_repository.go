package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// SubscriptionRepository defines data access methods for subscription
type SubscriptionRepository interface {
	// Query Queries subscription
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (subs []*common.Subscription, total int64, err error)
	// Get - Finds Subscription by id
	Get(
		qc *common.QueryContext,
		id string) (*common.Subscription, error)
	// Delete subscription by id
	Delete(
		qc *common.QueryContext,
		id string) error
	// Create - Saves subscription
	Create(
		subscription *common.Subscription) (*common.Subscription, error)
	// Update - Saves subscription
	Update(
		qc *common.QueryContext,
		subscription *common.Subscription) (*common.Subscription, error)
	// Clear for testing
	Clear()
}
