package types

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"testing"
)

// Verify permissions
func Test_ShouldVerifyPermissions(t *testing.T) {
	u := types.NewUser("org", "username", "name", false)
	require.True(t, u.HasPermission(acl.JobRequest, acl.Submit))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.False(t, u.HasPermission(acl.User, acl.Create))
}

// Verify permissions for admin
func Test_ShouldVerifyPermissionsForAdmin(t *testing.T) {
	u := types.NewUser("org", "username", "name", true)
	require.True(t, u.HasPermission(acl.JobRequest, acl.Upload))
	require.True(t, u.HasPermission(acl.JobRequest, acl.Execute))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Create))
	require.True(t, u.HasPermission(acl.JobDefinition, acl.Read))
	require.True(t, u.HasPermission(acl.Artifact, acl.View))
	require.True(t, u.HasPermission(acl.User, acl.Create))
}
