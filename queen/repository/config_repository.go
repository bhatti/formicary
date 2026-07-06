// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	common "plexobject.com/formicary/internal/types"
)

// ConfigRepository defines data access methods for polymorphic configs
// (owned by either an organization or a user).
type ConfigRepository interface {
	// Query returns configs for a given owner (organization or user).
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (recs []*common.Config, totalRecords int64, err error)
	// Count returns the number of configs matching the query.
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)
	// Get finds a Config by its primary key.
	Get(
		qc *common.QueryContext,
		id string) (*common.Config, error)
	// Delete removes a config by its primary key.
	Delete(
		qc *common.QueryContext,
		id string) error
	// Save creates or updates a config.
	Save(
		qc *common.QueryContext,
		cfg *common.Config) (*common.Config, error)
	// QueryOrgConfigs returns org-scoped configs for the given org ID.
	QueryOrgConfigs(
		qc *common.QueryContext,
		orgID string,
		page int,
		pageSize int) ([]*common.Config, int64, error)
	// QueryUserConfigs returns user-scoped configs for the given user ID.
	QueryUserConfigs(
		qc *common.QueryContext,
		userID string,
		page int,
		pageSize int) ([]*common.Config, int64, error)
}
