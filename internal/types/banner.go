// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "time"

const (
	BannerScopeGlobal  = "global"
	BannerScopeOrg     = "org"
	BannerSourceAdmin  = "admin"
	BannerSourceSystem = "system"
	BannerLevelWarning = "warning"
	BannerLevelDanger  = "danger"
	BannerLevelInfo    = "info"
)

// Banner represents a persistent dashboard notification.
// Global banners (scope="global") are shown to all admins.
// Org-scoped banners (scope="org") are shown only to members of the matching org.
// System banners (source="system") are auto-created by health monitors and the ant health bridge.
// Admin banners (source="admin") are manually created via the dashboard.
type Banner struct {
	ID        string    `json:"id"         gorm:"column:id;primaryKey"`
	Key       string    `json:"key"        gorm:"column:key;not null;default:''"`
	Level     string    `json:"level"      gorm:"column:level;not null;default:'warning'"`
	Scope     string    `json:"scope"      gorm:"column:scope;not null;default:'global'"`
	OrgID     string    `json:"org_id"     gorm:"column:org_id;not null;default:''"`
	Source    string    `json:"source"     gorm:"column:source;not null;default:'admin'"`
	Message   string    `json:"message"    gorm:"column:message;not null"`
	Active    bool      `json:"active"     gorm:"column:active;not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName returns the GORM table name.
func (Banner) TableName() string { return "formicary_banners" }
