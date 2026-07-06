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

// BackfillDefaultPermissions adds missing entries without touching existing ones.
func Test_ShouldBackfillDefaultPermissions(t *testing.T) {
	// Simulate a user created before Dashboard was added — they have some perms but not Dashboard.
	permsWithoutDashboard := acl.MarshalPermissions([]*acl.Permission{
		acl.NewPermission(acl.JobRequest, acl.View|acl.Submit),
		acl.NewPermission(acl.JobDefinition, acl.Create|acl.Read),
	})
	u := &User{SerializedPerms: permsWithoutDashboard}

	require.False(t, u.HasPermission(acl.Dashboard, acl.View), "should be missing before backfill")

	u.BackfillDefaultPermissions()

	require.True(t, u.HasPermission(acl.Dashboard, acl.View), "should have Dashboard after backfill")
	// Existing entries must be preserved unchanged.
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit), "existing perm must survive backfill")
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create), "existing perm must survive backfill")
}

// BackfillDefaultPermissions is idempotent when all defaults already present.
func Test_ShouldNotChangePersWhenBackfillNotNeeded(t *testing.T) {
	u := NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	original := u.SerializedPerms
	u.BackfillDefaultPermissions()
	require.Equal(t, original, u.SerializedPerms, "backfill must be idempotent when all defaults present")
}
