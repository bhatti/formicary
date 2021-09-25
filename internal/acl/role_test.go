package acl

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify role
func Test_ShouldMatchRole(t *testing.T) {
	roles := NewRoles("")
	roles.AddRole(Admin)
	require.True(t, roles.IsAdmin())
	require.True(t, roles.IsReadAdmin())
}

// Verify scope
func Test_ShouldMatchScope(t *testing.T) {
	role := NewRole("role", "a", "b", "c")
	require.True(t, role.MatchesScope("a", "b", "c"))
	require.True(t, role.MatchesScope("1", "b", "a", "c", "d"))
	require.False(t, role.MatchesScope("1", "a", "b"))
	require.False(t, role.MatchesScope())
}

// Verify marshal
func Test_ShouldMarshalRolesRoles(t *testing.T) {
	ser := MarshalRoles([]*Role{
		NewRole("manager", "product"),
		NewRole("supervisor", "ops"),
		NewRole("engineer", "product"),
	})
	roles := UnmarshalRoles(ser)
	require.Equal(t, 3, len(roles))
}

// Compare roles
func Test_ShouldCompareRoles(t *testing.T) {
	roles := NewRolesWithAdmin()
	roles.AddRole(ReadAdmin)
	roles.AddRole("manager", "product")
	ser := roles.String()
	t.Logf("serialized-roles: %s", ser)
	loaded := NewRoles(ser)
	for _, r := range roles.lookup {
		require.True(t, loaded.HasRole(r.RoleType, r.Scope...))
	}
}
