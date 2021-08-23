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
	// Pause action
	Pause = 16384
	// Unpause action
	Unpause = 32768
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

// New Constructor
func New(resource Resource, actions int) *Permission {
	return &Permission{Resource: resource, Actions: actions}
}

// Has checks permission
func (p *Permission) Has(action int) bool {
	return p.WildAll() || p.Actions&action == action
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
	if p.Actions&Pause == Pause {
		sb.WriteString("Pause ")
	}
	if p.Actions&Unpause == Unpause {
		sb.WriteString("Unpause ")
	}
	if p.Actions&Metrics == Metrics {
		sb.WriteString("Metrics ")
	}
	if p.Actions&Subscribe == Subscribe {
		sb.WriteString("Subscribe ")
	}
	if p.WildAll() {
		sb.WriteString("*")
	}
	return strings.TrimSpace(sb.String())
}

// LongString to string
func (p *Permission) LongString() string {
	return fmt.Sprintf("%s: %s", p.Resource, p.LongAction())
}

// Marshal permissions to string
func Marshal(perms []*Permission) string {
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

// Unmarshal converts string to permissions
func Unmarshal(s string) []*Permission {
	res := make([]*Permission, 0)
	if s == "" {
		return res
	}
	words := strings.Split(s, ";")
	for _, w := range words {
		resourceAction := strings.Split(w, "=")
		if len(resourceAction) == 2 {
			if action, err := strconv.Atoi(strings.TrimSpace(resourceAction[1])); err == nil {
				resource := Resource(strings.TrimSpace(resourceAction[0]))
				res = append(res, New(resource, action))
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
	perms := Unmarshal(str)
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
	return Marshal(perms)
}

// String to string
func (p *Permissions) String() string {
	return p.Marshal()
}

// DefaultPermissionsString string permissions
func DefaultPermissionsString() string {
	return Marshal(DefaultPermissions())
}

// DefaultPermissions default permissions
func DefaultPermissions() []*Permission {
	return []*Permission{
		New(Audit, None),
		New(Websocket, Subscribe),
		New(Dashboard, View),
		New(JobRequest, View|Execute|Submit|Cancel|Restart),
		New(JobDefinition, Create|Read|Update|Delete|Query|Pause|Unpause|Metrics),
		New(JobResource, Create|Read|Update|Delete|Query|Pause|Unpause),
		New(User, Read|Update|Delete|Login|Logout|Query|Signup),
		New(Organization, Read|Update|Delete|Invite),
		New(OrgConfig, Create|Read|Update|Delete|Query),
		New(Artifact, Upload|Read|Query|Delete),
		New(ErrorCode, None),
		New(SystemConfig, None),
		New(AntExecutor, None),
		New(Container, None),
		New(Health, None),
		New(Profile, None),
		New(Subscription, None),
		New(TermsService, View|Read),
		New(PrivacyPolicies, View|Read),
	}
}

// AdminPermissions admin permissions
func AdminPermissions() []*Permission {
	return []*Permission{
		New(Audit, Query|View|Read),
		New(AntExecutor, Query|View|Read),
		New(Container, Query|View|Read|Delete),
		New(ErrorCode, Query|View|Read|Create|Update|Delete),
		New(Websocket, Subscribe),
		New(Dashboard, View),
		New(JobRequest, View|Execute|Submit|Cancel|Restart|Metrics),
		New(JobDefinition, Create|Read|Update|Delete|Query|Pause|Unpause|Metrics),
		New(JobResource, Create|Read|Update|Delete|Query|Pause|Unpause),
		New(User, Read|Update|Delete|Login|Logout|Query|Signup),
		New(Organization, Read|Update|Delete|Invite),
		New(OrgConfig, Create|Read|Update|Delete|Query),
		New(Artifact, Upload|Read|Query|Delete),
		New(Subscription, Create|Read|Update|Delete|Query),
		New(TermsService, View|Read),
		New(PrivacyPolicies, View|Read),
		New(ErrorCode, Create|Read|Update|Delete|Query),
		New(SystemConfig, Create|Read|Update|Delete|Query),
		New(AntExecutor, Create|Read|Update|Delete|Query),
		New(Container, Create|Read|Update|Delete|Query),
		New(Health, Create|Read|Update|Delete|Query),
		New(Profile, Create|Read|Update|Delete|Query),
	}
}
