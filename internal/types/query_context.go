package types

import (
	"fmt"
	"gorm.io/gorm"
)

// QueryContext context for user/org scope so that you cannot accidentally leak any private data
type QueryContext struct {
	UserIDColumn         string
	OrganizationIDColumn string
	UserID               string
	Username             string
	OrganizationID       string
	Salt                 string
	IPAddress            string
	admin                bool
}

// NewQueryContextFromUser constructor
func NewQueryContextFromUser(user *User, org *Organization, ipAddr string) *QueryContext {
	if user == nil {
		return &QueryContext{
			UserIDColumn:         "user_id",
			OrganizationIDColumn: "organization_id",
			IPAddress:            ipAddr,
			Salt:                 "",
			admin:                true}
	}
	salt := user.Salt
	if org != nil {
		salt = org.Salt
	}
	return &QueryContext{
		UserIDColumn:         "user_id",
		OrganizationIDColumn: "organization_id",
		UserID:               user.ID,
		Username:             user.Username,
		OrganizationID:       user.OrganizationID,
		Salt:                 salt,
		IPAddress:            ipAddr,
		admin:                user.Admin}
}

// NewQueryContext constructor
func NewQueryContext(userID string, organizationID string, salt string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         "user_id",
		OrganizationIDColumn: "organization_id",
		UserID:               userID,
		OrganizationID:       organizationID,
		Salt:                 salt,
		admin:                false}
}

// WithUserIDColumn setter
func (qc *QueryContext) WithUserIDColumn(userIDColumn string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         userIDColumn,
		OrganizationIDColumn: qc.OrganizationIDColumn,
		UserID:               qc.UserID,
		Username:             qc.Username,
		OrganizationID:       qc.OrganizationID,
		Salt:                 qc.Salt,
		IPAddress:            qc.IPAddress,
		admin:                qc.admin,
	}
}

// WithOrganizationIDColumn setter
func (qc *QueryContext) WithOrganizationIDColumn(organizationIDColumn string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         qc.UserIDColumn,
		OrganizationIDColumn: organizationIDColumn,
		UserID:               qc.UserID,
		Username:             qc.Username,
		OrganizationID:       qc.OrganizationID,
		Salt:                 qc.Salt,
		IPAddress:            qc.IPAddress,
		admin:                qc.admin,
	}
}

// WithAdmin setter
func (qc *QueryContext) WithAdmin() *QueryContext {
	qc.admin = true
	return qc
}

// IsNull checks if user-id and org are not specified
func (qc *QueryContext) IsNull() bool {
	return qc.UserID == "" && qc.OrganizationID == ""
}

// Matches - association to org
func (qc *QueryContext) Matches(userID string, orgID string) bool {
	return qc.admin || qc.UserID == "" || qc.UserID == userID || qc.OrganizationID == orgID
}

// String textual content
func (qc *QueryContext) String() string {
	return fmt.Sprintf("[%s;%s;%v]", qc.UserID, qc.OrganizationID, qc.admin)
}

// HasOrganization - association to org
func (qc *QueryContext) HasOrganization() bool {
	return qc.OrganizationID != ""
}

// AddOrgElseUserWhere - adds user scope
func (qc *QueryContext) AddOrgElseUserWhere(db *gorm.DB) *gorm.DB {
	if qc.admin {
		return db
	}
	if qc.HasOrganization() && qc.OrganizationIDColumn != "" {
		return db.Where(qc.OrganizationIDColumn+" = ?", qc.OrganizationID)
	}
	return qc.AddUserWhere(db)
}

// AddUserWhere - adds user scope
func (qc *QueryContext) AddUserWhere(db *gorm.DB) *gorm.DB {
	if qc.admin || qc.UserID == "" || qc.UserIDColumn == "" {
		return db
	}
	return db.Where(qc.UserIDColumn+" = ?", qc.UserID)
}

// AddOrgWhere - adds user scope
func (qc *QueryContext) AddOrgWhere(db *gorm.DB) *gorm.DB {
	if qc.admin {
		return db
	}
	if qc.HasOrganization() && qc.OrganizationIDColumn != "" {
		return db.Where(qc.OrganizationIDColumn+" = ?", qc.OrganizationID)
	}
	return db
}

// AddUserWhereSQL - adds user scope
func (qc *QueryContext) AddUserWhereSQL() (string, string) {
	if qc.admin || qc.UserID == "" || qc.UserIDColumn == "" {
		return "1 = ?", "1"
	}
	return qc.UserIDColumn + " = ?", qc.UserID
}

// AddOrgWhereSQL - adds user scope
func (qc *QueryContext) AddOrgWhereSQL() (string, string) {
	if qc.admin || qc.OrganizationID == "" || qc.OrganizationIDColumn == "" {
		return "1 = ?", "1"
	}
	return qc.OrganizationIDColumn + " = ?", qc.OrganizationID
}

// AddOrgUserWhereSQL - adds user scope
func (qc *QueryContext) AddOrgUserWhereSQL() (string, string) {
	if qc.admin {
		return "1 = ?", "1"
	}
	if qc.OrganizationID != "" && qc.OrganizationIDColumn != "" {
		return qc.AddOrgWhereSQL()
	}
	return qc.AddUserWhereSQL()
}

// Admin - flag
func (qc *QueryContext) Admin() bool {
	return qc.admin
}
