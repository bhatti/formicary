package acl

import (
	"fmt"
	"strconv"
	"strings"
)

// Resource type
type Resource string

const (
	// Audit resource
	Audit Resource = "Audit"
	// Dashboard resource
	Dashboard Resource = "Dashboard"
	// JobRequest resource
	JobRequest Resource = "JobRequest"
	// JobDefinition resource
	JobDefinition Resource = "JobDefinition"
	// JobResource resource
	JobResource Resource = "JobResource"
	// User resource
	User Resource = "User"
	// Organization resource
	Organization Resource = "Organization"
	// SystemConfig resource
	SystemConfig Resource = "SystemConfig"
	// OrgConfig resource
	OrgConfig Resource = "OrgConfig"
	// ErrorCode resource
	ErrorCode Resource = "ErrorCode"
	// Artifact resource
	Artifact Resource = "Artifact"
	// AntExecutor resource
	AntExecutor Resource = "AntExecutor"
	// Container resource
	Container Resource = "Container"
	// Websocket resource
	Websocket Resource = "Websocket"
	// Health resource
	Health Resource = "Health"
	// Profile resource
	Profile Resource = "Profile"
	// Subscription resource
	Subscription Resource = "Subscription"
	// TermsService resource
	TermsService Resource = "TermsService"
	// PrivacyPolicies resource
	PrivacyPolicies Resource = "PrivacyPolicies"
	// EmailVerification resource
	EmailVerification Resource = "EmailVerification"
	// UserInvitation resource
	UserInvitation Resource = "UserInvitation"
	// Report resource
	Report Resource = "Report"
)

const (
	// None action
	None = 0
	// Execute action
	Execute = 1
	// Create action
	Create = 2
	// Read action
	Read = 4
	// View action
	View = 4
	// Query action
	Query = 8
	// Write action
	Write = 16
	// Update action
	Update = 16
	// Verify action
	Verify = 16
	// Delete action
	Delete = 32
	// Submit action
	Submit = 64
	// Cancel action
	Cancel = 128
	// Restart action
	Restart = 256
	// Trigger action
	Trigger = 256
	// Invite action
	Invite = 512
	// Upload action
	Upload = 1024
	// Login action
	Login = 2048
	// Logout action
	Logout = 4096
	// Signup action
	Signup = 8192
	// Disable action
	Disable = 16384
	// Enable action
	Enable = 32768
	// Metrics action
	Metrics = 65536
	// Subscribe action
	Subscribe = 131072
	// All action
	All = 1024 * 1024 * 1024
)

// Permission ACL
type Permission struct {
	Resource Resource `json:"resource"`
	Actions  int      `json:"action"`
}

// NewPermission Constructor
func NewPermission(resource Resource, actions int) *Permission {
	return &Permission{Resource: resource, Actions: actions}
}

// Has checks permission
func (p *Permission) Has(action int) bool {
	return p.WildAll() || p.Actions&action == action
}

// ReadOnly checks read-only access
func (p *Permission) ReadOnly() bool {
	return p.Has(None) ||
		p.Has(Read) ||
		p.Has(View) ||
		p.Has(Query) ||
		p.Has(Verify) ||
		p.Has(Metrics) ||
		p.Has(Subscribe)
}

// WildAll checks all access
func (p *Permission) WildAll() bool {
	return p.Actions == All || p.Actions < 0
}

// String to string
func (p *Permission) String() string {
	return fmt.Sprintf("%s=%d", p.Resource, p.Actions)
}

// NotEmpty permission
func (p *Permission) NotEmpty() bool {
	return p.Actions != 0
}

// LongAction to string
func (p *Permission) LongAction() string {
	if p.WildAll() {
		return "*"
	}
	sb := strings.Builder{}
	if p.Actions&Execute == Execute {
		sb.WriteString("Execute ")
	}
	if p.Actions&Create == Create {
		sb.WriteString("Create ")
	}
	if p.Actions&Read == Read {
		sb.WriteString("Read ")
	}
	if p.Actions&View == View {
		sb.WriteString("View ")
	}
	if p.Actions&Query == Query {
		sb.WriteString("Query ")
	}
	if p.Actions&Write == Write {
		sb.WriteString("Write ")
	}
	if p.Actions&Verify == Verify {
		sb.WriteString("Verify ")
	}
	if p.Actions&Update == Update {
		sb.WriteString("Update ")
	}
	if p.Actions&Delete == Delete {
		sb.WriteString("Delete ")
	}
	if p.Actions&Submit == Submit {
		sb.WriteString("Submit ")
	}
	if p.Actions&Cancel == Cancel {
		sb.WriteString("Cancel ")
	}
	if p.Actions&Restart == Restart {
		sb.WriteString("Restart ")
	}
	if p.Actions&Trigger == Trigger {
		sb.WriteString("Trigger ")
	}
	if p.Actions&Invite == Invite {
		sb.WriteString("Invite ")
	}
	if p.Actions&Upload == Upload {
		sb.WriteString("Upload ")
	}
	if p.Actions&Login == Login {
		sb.WriteString("Login ")
	}
	if p.Actions&Logout == Logout {
		sb.WriteString("Logout ")
	}
	if p.Actions&Signup == Signup {
		sb.WriteString("Signup ")
	}
	if p.Actions&Disable == Disable {
		sb.WriteString("Disable ")
	}
	if p.Actions&Enable == Enable {
		sb.WriteString("Enable ")
	}
	if p.Actions&Metrics == Metrics {
		sb.WriteString("Metrics ")
	}
	if p.Actions&Subscribe == Subscribe {
		sb.WriteString("Subscribe ")
	}
	return strings.TrimSpace(sb.String())
}

// LongString to string
func (p *Permission) LongString() string {
	return fmt.Sprintf("%s: %s", p.Resource, p.LongAction())
}

// MarshalPermissions permissions to string
func MarshalPermissions(perms []*Permission) string {
	if len(perms) == 0 {
		return ""
	}
	sb := strings.Builder{}
	for _, p := range perms {
		if sb.Len() > 0 {
			sb.WriteString(";")
		}
		sb.WriteString(p.String())
	}
	return sb.String()
}

// UnmarshalPermissions converts string to permissions
func UnmarshalPermissions(s string) []*Permission {
	res := make([]*Permission, 0)
	if s == "" {
		return res
	}
	lines := strings.Split(s, ";")
	for _, line := range lines {
		resourceAction := strings.Split(line, "=")
		if len(resourceAction) == 2 {
			if action, err := strconv.Atoi(strings.TrimSpace(resourceAction[1])); err == nil {
				resource := Resource(strings.TrimSpace(resourceAction[0]))
				res = append(res, NewPermission(resource, action))
			}
		}
	}
	return res
}

// Permissions store
type Permissions struct {
	lookup      map[Resource]*Permission
	hasWildcard bool
}

// NewPermissions Constructor
func NewPermissions(str string) *Permissions {
	perms := UnmarshalPermissions(str)
	lookup := make(map[Resource]*Permission)
	hasWildcard := false
	for _, p := range perms {
		lookup[p.Resource] = p
		if p.Resource == "*" && (p.Actions < 0 || p.Actions == All) {
			hasWildcard = true
		}
	}
	return &Permissions{lookup: lookup, hasWildcard: hasWildcard}
}

// Has checks permission
func (p *Permissions) Has(resource Resource, action int) bool {
	if p.hasWildcard {
		return true
	}
	matched := p.lookup[resource]
	if matched == nil {
		return false
	}
	return matched.Has(action)
}

// Marshal permissions to string
func (p *Permissions) Marshal() string {
	perms := make([]*Permission, len(p.lookup))
	i := 0
	for _, p := range p.lookup {
		perms[i] = p
		i++
	}
	return MarshalPermissions(perms)
}

// String to string
func (p *Permissions) String() string {
	return p.Marshal()
}

// DefaultPermissionsString string permissions
func DefaultPermissionsString() string {
	return MarshalPermissions(DefaultPermissions())
}

// DefaultPermissions default permissions
func DefaultPermissions() []*Permission {
	return []*Permission{
		NewPermission(Audit, None),
		NewPermission(Websocket, Subscribe),
		NewPermission(Dashboard, View),
		NewPermission(JobRequest, View|Execute|Submit|Cancel|Restart),
		NewPermission(JobDefinition, Create|Read|Update|Delete|Query|Disable|Enable|Metrics),
		NewPermission(JobResource, Create|Read|Update|Delete|Query|Disable|Enable),
		NewPermission(User, Read|Update|Delete|Login|Logout|Query|Signup),
		NewPermission(Organization, Read|Update|Delete|Invite),
		NewPermission(OrgConfig, Create|Read|Update|Delete|Query),
		NewPermission(Artifact, Upload|Read|Query|Delete),
		NewPermission(ErrorCode, Query|View|Read|Create|Update|Delete),
		NewPermission(SystemConfig, None),
		NewPermission(AntExecutor, None),
		NewPermission(Container, None),
		NewPermission(Health, None),
		NewPermission(Profile, None),
		NewPermission(Subscription, None),
		NewPermission(TermsService, View|Read),
		NewPermission(PrivacyPolicies, View|Read),
		NewPermission(EmailVerification, Create|View|Read|Verify|Query),
		NewPermission(UserInvitation, Create|View|Update|Read|Query|Invite),
		NewPermission(Report, None),
	}
}

// AdminPermissions admin permissions
func AdminPermissions() []*Permission {
	return []*Permission{
		NewPermission(Audit, Query|View|Read),
		NewPermission(AntExecutor, Query|View|Read),
		NewPermission(Container, Query|View|Read|Delete),
		NewPermission(ErrorCode, Query|View|Read|Create|Update|Delete),
		NewPermission(Websocket, Subscribe),
		NewPermission(Dashboard, View),
		NewPermission(JobRequest, View|Execute|Submit|Cancel|Restart|Metrics),
		NewPermission(JobDefinition, Create|Read|Update|Delete|Query|Disable|Enable|Metrics),
		NewPermission(JobResource, Create|Read|Update|Delete|Query|Disable|Enable),
		NewPermission(User, Read|Update|Delete|Login|Logout|Query|Signup),
		NewPermission(Organization, Read|Update|Delete|Invite),
		NewPermission(OrgConfig, Create|Read|Update|Delete|Query),
		NewPermission(Artifact, Upload|Read|Query|Delete),
		NewPermission(Subscription, Create|Read|Update|Delete|Query),
		NewPermission(TermsService, View|Read),
		NewPermission(PrivacyPolicies, View|Read),
		NewPermission(ErrorCode, Create|Read|Update|Delete|Query),
		NewPermission(SystemConfig, Create|Read|Update|Delete|Query),
		NewPermission(AntExecutor, Create|Read|Update|Delete|Query),
		NewPermission(Container, Create|Read|Update|Delete|Query),
		NewPermission(Health, Create|Read|Update|Delete|Query),
		NewPermission(Profile, Create|Read|Update|Delete|Query),
		NewPermission(EmailVerification, Create|Read|Update|Delete|Query),
		NewPermission(UserInvitation, Create|View|Update|Read|Query|Invite),
		NewPermission(Report, View|Read|Query),
	}
}
