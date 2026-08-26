// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "time"

// SlackRegCode is a short-lived one-time code used to register a Slack user
// without exposing a raw API token in DM history.
type SlackRegCode struct {
	Code      string    `json:"code"       gorm:"column:code;primaryKey"`
	UserID    string    `json:"user_id"    gorm:"column:user_id;not null"`
	OrgID     string    `json:"org_id"     gorm:"column:org_id;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"column:expires_at;not null"`
	Used      bool      `json:"used"       gorm:"column:used;not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt *time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName returns the DB table name.
func (SlackRegCode) TableName() string { return "formicary_slack_reg_codes" }
