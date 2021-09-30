package types

import (
	"fmt"
	"gorm.io/gorm"
	"plexobject.com/formicary/internal/acl"
)

// QueryContext context for user/org scope so that you cannot accidentally leak any private data
type QueryContext struct {
	UserIDColumn         string
	OrganizationIDColumn string
	User                 *User
	IPAddress            string
}

// NewQueryContext constructor
func NewQueryContext(user *User, ipAddr string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         "user_id",
		OrganizationIDColumn: "organization_id",
		IPAddress:            ipAddr,
		User:                 user,
	}
}

// NewQueryContextFromIDs constructor
func NewQueryContextFromIDs(userID string, orgID string) *QueryContext {
	user := NewUser(orgID, "", "", "", acl.NewRoles(""))
	user.ID = userID
	if orgID != "" {
		user.Organization = NewOrganization("", "", "")
		user.Organization.ID = orgID
	}
	return NewQueryContext(user, "")
}

// WithUserIDColumn setter
func (qc *QueryContext) WithUserIDColumn(userIDColumn string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         userIDColumn,
		OrganizationIDColumn: qc.OrganizationIDColumn,
		User:                 qc.User,
		IPAddress:            qc.IPAddress,
	}
}

// WithOrganizationIDColumn setter
func (qc *QueryContext) WithOrganizationIDColumn(organizationIDColumn string) *QueryContext {
	return &QueryContext{
		UserIDColumn:         qc.UserIDColumn,
		OrganizationIDColumn: organizationIDColumn,
		User:                 qc.User,
		IPAddress:            qc.IPAddress,
	}
}

// WithAdmin setter
func (qc *QueryContext) WithAdmin() *QueryContext {
	if qc.User != nil {
		return &QueryContext{
			UserIDColumn:         qc.UserIDColumn,
			OrganizationIDColumn: qc.OrganizationIDColumn,
			User:                 NewUser("", "", "", "", acl.NewRoles("Admin[]")),
			IPAddress:            qc.IPAddress,
		}
	}
	return qc
}

// WithoutAdmin setter
func (qc *QueryContext) WithoutAdmin() *QueryContext {
	if qc.User != nil {
		return &QueryContext{
			UserIDColumn:         qc.UserIDColumn,
			OrganizationIDColumn: qc.OrganizationIDColumn,
			User:                 NewUser("", "", "", "", acl.NewRoles("")),
			IPAddress:            qc.IPAddress,
		}
	}
	return qc
}

// IsNull checks if user-id and org are not specified
func (qc *QueryContext) IsNull() bool {
	return qc.User == nil && qc.User.OrganizationID == ""
}

// Matches - association to org
func (qc *QueryContext) Matches(userID string, orgID string, readonly bool) bool {
	if qc.IsAdmin() || qc.User == nil || (qc.IsReadAdmin() && readonly) {
		return true
	}
	if qc.User.HasOrganization() || orgID != "" {
		return qc.User.OrganizationID == orgID
	}
	return qc.User.ID == userID
}

// String textual content
func (qc *QueryContext) String() string {
	return fmt.Sprintf("[%s]", qc.User)
}

// HasOrganization - association to org
func (qc *QueryContext) HasOrganization() bool {
	return qc.User != nil && qc.User.Organization != nil
}

// GetOrganizationID - id of org
func (qc *QueryContext) GetOrganizationID() string {
	if qc.User != nil {
		return qc.User.OrganizationID
	}
	return ""
}

// GetUserID - id of user
func (qc *QueryContext) GetUserID() string {
	if qc.User != nil {
		return qc.User.ID
	}
	return ""
}

// GetUsername - username of user
func (qc *QueryContext) GetUsername() string {
	if qc.User != nil {
		return qc.User.Username
	}
	return ""
}

// AddOrgElseUserWhere - adds user scope
func (qc *QueryContext) AddOrgElseUserWhere(db *gorm.DB, readonly bool) *gorm.DB {
	if qc.IsAdmin() || (readonly && qc.IsReadAdmin()) {
		return db
	}
	if qc.HasOrganization() && qc.OrganizationIDColumn != "" {
		return db.Where(qc.OrganizationIDColumn+" = ?",
			qc.User.OrganizationID)
	}
	return qc.AddUserWhere(db, readonly)
}

// AddUserWhere - adds user scope
func (qc *QueryContext) AddUserWhere(db *gorm.DB, readonly bool) *gorm.DB {
	if qc.IsAdmin() || qc.User == nil || qc.UserIDColumn == "" || (readonly && qc.IsReadAdmin()) {
		return db
	}
	return db.Where(qc.UserIDColumn+" = ?", qc.User.ID)
}

// AddOrgWhere - adds user scope
func (qc *QueryContext) AddOrgWhere(db *gorm.DB, readonly bool) *gorm.DB {
	if qc.IsAdmin() || (readonly && qc.IsReadAdmin()) {
		return db
	}
	if qc.HasOrganization() && qc.OrganizationIDColumn != "" {
		return db.Where(qc.OrganizationIDColumn+" = ?",
			qc.User.OrganizationID)
	}
	return db
}

// AddUserWhereSQL - adds user scope
func (qc *QueryContext) AddUserWhereSQL(readonly bool) (string, string) {
	if qc.IsAdmin() ||
		qc.User == nil ||
		qc.UserIDColumn == "" ||
		(readonly && qc.IsReadAdmin()) {
		return "'1' = ?", "1"
	}
	return qc.UserIDColumn + " = ?", qc.User.ID
}

// AddOrgWhereSQL - adds user scope
func (qc *QueryContext) AddOrgWhereSQL(readonly bool) (string, string) {
	if qc.IsAdmin() ||
		qc.User == nil ||
		qc.User.Organization == nil ||
		qc.OrganizationIDColumn == "" ||
		(readonly && qc.IsReadAdmin()) {
		return "'1' = ?", "1"
	}
	return qc.OrganizationIDColumn + " = ?", qc.User.OrganizationID
}

// AddOrgUserWhereSQL - adds user scope
func (qc *QueryContext) AddOrgUserWhereSQL(readonly bool) (string, string) {
	if qc.IsAdmin() || (readonly && qc.IsReadAdmin()){
		return "'1' = ?", "1"
	}
	if qc.User != nil && qc.User.Organization != nil && qc.OrganizationIDColumn != "" {
		return qc.AddOrgWhereSQL(readonly)
	}
	return qc.AddUserWhereSQL(readonly)
}

// IsAdmin - flag
func (qc *QueryContext) IsAdmin() bool {
	if qc.User != nil {
		return qc.User.IsAdmin()
	}
	return true
}

// IsReadAdmin - flag
func (qc *QueryContext) IsReadAdmin() bool {
	if qc.User != nil {
		return qc.User.IsReadAdmin()
	}
	return true
}

// GetSalt getter
func (qc *QueryContext) GetSalt() string {
	if qc.User != nil {
		if qc.User.Organization != nil {
			return qc.User.Organization.Salt
		}
		return qc.User.Salt
	}
	return ""
}
