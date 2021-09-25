package acl

import (
	"fmt"
	"strings"
)

// RoleType type
type RoleType string

const (
	// Admin role
	Admin RoleType = "Admin"
	// ReadAdmin role
	ReadAdmin RoleType = "ReadAdmin"
)

// Role ACL
type Role struct {
	RoleType RoleType `json:"role"`
	Scope    []string `json:"scope"`
}

// NewRole Constructor
func NewRole(roleType RoleType, scope ...string) *Role {
	return &Role{RoleType: roleType, Scope: scope}
}

// IsAdmin checks admin access
func (p *Role) IsAdmin(scope ...string) bool {
	if p.RoleType != Admin {
		return false
	}
	return p.MatchesScope(scope...)
}

// IsReadAdmin checks read admin access
func (p *Role) IsReadAdmin(scope ...string) bool {
	if p.RoleType != ReadAdmin {
		return false
	}
	return p.MatchesScope(scope...)
}

// MatchesScope checks scope access
func (p *Role) MatchesScope(scope ...string) bool {
	if len(p.Scope) == 0 {
		return true
	}
	for _, roleScope := range p.Scope {
		matched := false
		for _, pScope := range scope {
			if roleScope == pScope {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// String to string
func (p *Role) String() string {
	return fmt.Sprintf("%s%v", p.RoleType, p.Scope)
}

// MarshalRoles to string
func MarshalRoles(roles []*Role) string {
	if len(roles) == 0 {
		return ""
	}
	sb := strings.Builder{}
	for _, r := range roles {
		if sb.Len() > 0 {
			sb.WriteString(";")
		}
		sb.WriteString(r.String())
	}
	return sb.String()
}

// UnmarshalRoles converts string to roles
func UnmarshalRoles(s string) []*Role {
	res := make([]*Role, 0)
	if s == "" {
		return res
	}
	lines := strings.Split(s, ";")
	for _, line := range lines {
		parts := strings.Split(line, "[")
		if len(parts) == 2 {
			role := RoleType(strings.TrimSpace(parts[0]))
			scopeLines := strings.Split(parts[1], ",")
			scope := make([]string, 0)
			for _, scopeLine := range scopeLines {
				next := strings.TrimSpace(strings.ReplaceAll(scopeLine, "]", ""))
				if next != "" {
					scope = append(scope, next)
				}
			}
			res = append(res, NewRole(role, scope...))
		}
	}
	return res
}

// Roles store
type Roles struct {
	lookup map[RoleType]*Role
}

// NewRoles Constructor
func NewRoles(str string) *Roles {
	roles := &Roles{lookup: make(map[RoleType]*Role)}
	for _, next := range UnmarshalRoles(str) {
		roles.lookup[next.RoleType] = next
	}
	return roles
}

// NewRolesWithAdmin Constructor
func NewRolesWithAdmin() *Roles {
	roles := &Roles{lookup: make(map[RoleType]*Role)}
	roles.AddRole(Admin)
	return roles
}

// NewRolesWithReadAdmin Constructor
func NewRolesWithReadAdmin() *Roles {
	roles := &Roles{lookup: make(map[RoleType]*Role)}
	roles.AddRole(ReadAdmin)
	return roles
}

// AddRole adds role
func (r *Roles) AddRole(role RoleType, scope ...string) {
	r.lookup[role] = NewRole(role, scope...)
}

// MarshalRoles roles to string
func (r *Roles) MarshalRoles() string {
	roles := make([]*Role, 0)
	for _, next := range r.lookup {
		roles = append(roles, next)
	}
	return MarshalRoles(roles)
}

// HasRole checks role access
func (r *Roles) HasRole(roleType RoleType, scope ...string) bool {
	role := r.lookup[roleType]
	if role == nil {
		return false
	}
	if roleType == Admin || roleType == ReadAdmin {
		return true // no scope check
	}
	return role.MatchesScope(scope...)
}

// IsAdmin checks admin access
func (r *Roles) IsAdmin() bool {
	return r.HasRole(Admin)
}

// IsReadAdmin checks read admin access
func (r *Roles) IsReadAdmin() bool {
	return r.HasRole(ReadAdmin) || r.HasRole(Admin)
}

// String to string
func (r *Roles) String() string {
	return r.MarshalRoles()
}
