package types

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"testing"
)

func Test_ShouldVerifyUserTable(t *testing.T) {
	u := NewUser("org", "username", "name", false)
	require.Equal(t, "formicary_users", u.TableName())
}

func Test_ShouldStringifyUser(t *testing.T) {
	u := NewUser("org", "username@gmail.com", "name", false)
	err := u.AfterLoad()
	require.NoError(t, err)
	require.NotEqual(t, "", u.String())
	require.NoError(t, u.ValidateBeforeSave())
	require.True(t, u.UsesCommonEmail())
	require.True(t, u.HasOrganization())
}

func Test_ShouldVerifyEqualForUser(t *testing.T) {
	u1 := NewUser("org1", "username", "name", false)
	u2 := NewUser("org1", "username", "name", false)
	u3 := NewUser("org2", "username", "name", false)
	require.NoError(t, u1.Equals(u2))
	require.Error(t, u1.Equals(u3))
	require.Error(t, u1.Equals(nil))
}

// Verify permissions
func Test_ShouldVerifyUserPermissions(t *testing.T) {
	u := NewUser("org", "username", "name", false)
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.False(t, u.HasPermission(acl.User, acl.Create))
	require.Equal(t, 18, len(u.PermissionList()))
}

// Verify permissions for admin
func Test_ShouldVerifyUserPermissionsForAdmin(t *testing.T) {
	u := NewUser("org", "username@email.com", "name", true)
	require.True(t, u.HasPermission(acl.JobRequest, acl.Upload))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.True(t, u.HasPermission(acl.User, acl.Create))
}
