package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
)

func Test_ShouldVerifyUserTable(t *testing.T) {
	u := NewUser("org", "username", "name", "email", acl.NewRoles(""))
	require.Equal(t, "formicary_users", u.TableName())
}

func Test_ShouldStringifyUser(t *testing.T) {
	u := NewUser("org", "username@gmail.com", "name", "", acl.NewRoles(""))
	u.Organization = &Organization{ID: "org"}
	err := u.AfterLoad()
	require.NoError(t, err)
	require.NotEqual(t, "", u.String())
	require.NoError(t, u.ValidateBeforeSave())
	require.True(t, u.UsesCommonEmail())
	require.True(t, u.HasOrganization())
}

func Test_ShouldVerifyEqualForUser(t *testing.T) {
	u1 := NewUser("org1", "username", "name", "", acl.NewRoles(""))
	u2 := NewUser("org1", "username", "name", "", acl.NewRoles(""))
	u3 := NewUser("org2", "username", "name", "", acl.NewRoles(""))
	require.NoError(t, u1.Equals(u2))
	require.Error(t, u1.Equals(u3))
	require.Error(t, u1.Equals(nil))
}

// Verify permissions
func Test_ShouldVerifyUserPermissions(t *testing.T) {
	u := NewUser("org", "username", "name", "", acl.NewRoles(""))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.False(t, u.HasPermission(acl.User, acl.Create))
	require.Equal(t, 24, len(u.PermissionList()))
}

// Verify permissions for admin
func Test_ShouldVerifyUserPermissionsForAdmin(t *testing.T) {
	u := NewUser("org", "username@formicary.io", "name", "", acl.NewRoles("Admin[]"))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Upload))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.True(t, u.HasPermission(acl.User, acl.Create))
}

// HasOrganization is true only when both OrganizationID and Organization are set
// (i.e. the org is persisted and loaded). OrgUnit is transient and does not count.
func Test_ShouldVerifyHasOrganization(t *testing.T) {
	// no org — individual account
	u := NewUser("", "username", "name", "email@example.com", acl.NewRoles(""))
	require.False(t, u.HasOrganization())
	require.False(t, u.HasOrganizationOrInvitationCode())

	// OrganizationID set but org not loaded — not considered "has org"
	u2 := NewUser("org-123", "username2", "name", "email2@example.com", acl.NewRoles(""))
	require.False(t, u2.HasOrganization())

	// Both ID and loaded org — fully linked
	u2.Organization = &Organization{ID: "org-123"}
	require.True(t, u2.HasOrganization())

	// OrgUnit alone (signup transient) does NOT make HasOrganization true
	u3 := NewUser("", "username3", "name", "email3@example.com", acl.NewRoles(""))
	u3.OrgUnit = "mycompany"
	require.False(t, u3.HasOrganization())
	// but it does make HasOrganizationOrInvitationCode true
	require.True(t, u3.HasOrganizationOrInvitationCode())

	// InvitationCode alone
	u4 := NewUser("", "username4", "name", "email4@example.com", acl.NewRoles(""))
	u4.InvitationCode = "invite-abc"
	require.False(t, u4.HasOrganization())
	require.True(t, u4.HasOrganizationOrInvitationCode())
}

// New users must get default permissions including Dashboard: View.
func Test_ShouldHaveDefaultPermissionsOnNewUser(t *testing.T) {
	u := NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	require.True(t, u.HasPermission(acl.Dashboard, acl.View), "new user must have Dashboard: View")
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit))
	require.True(t, u.HasPermission(acl.Websocket, acl.Subscribe))
}

// Permissions are derived from the User role — Dashboard is always present without any additive perms.
func Test_ShouldHaveDefaultPermissionsFromRole(t *testing.T) {
	u := NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	require.True(t, u.HasPermission(acl.Dashboard, acl.View), "User role must include Dashboard: View")
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit), "User role must include JobRequest: Submit")
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create), "User role must include JobDefinition: Create")
}

// AdditivePerms grants extra actions on top of the role baseline.
func Test_ShouldMergeAdditivePermsWithRoleBaseline(t *testing.T) {
	extra := acl.MarshalPermissions([]*acl.Permission{
		acl.NewPermission(acl.Report, acl.View|acl.Read),
	})
	u := &User{AdditivePerms: extra}
	// Role baseline (no role = User) gives Dashboard etc.; additive grants Report.
	require.True(t, u.HasPermission(acl.Dashboard, acl.View), "role baseline must apply")
	require.True(t, u.HasPermission(acl.Report, acl.View), "additive perm must be merged")
}

// ResetPermissionsCache clears cached perms — effective set must remain unchanged.
func Test_ShouldNotChangePersWhenCacheReset(t *testing.T) {
	u := NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	before := u.EffectivePermsString()
	u.ResetPermissionsCache()
	require.Equal(t, before, u.EffectivePermsString(), "cache reset must be idempotent")
}
