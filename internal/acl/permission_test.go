package acl

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Verify permission
func Test_ShouldMatchPermission(t *testing.T) {
	p := New(JobRequest, Read|Update)

	require.True(t, p.Has(Read))
	require.True(t, p.Has(Update))
	require.False(t, p.Has(Delete))
}

// Verify marshal
func Test_ShouldMarshalPermissions(t *testing.T) {
	ser := Marshal([]*Permission{
		New(JobRequest, Submit|Execute),
		New(JobDefinition, Create|Read|Update),
		New(JobResource, Delete|View|Write),
		New(User, View|Write),
		New(Organization, View|Write),
		New(SystemConfig, View|Write),
		New(OrgConfig, View|Write),
		New(ErrorCode, View|Write),
		New(Artifact, View|Write),
	})
	perms := Unmarshal(ser)
	require.Equal(t, 9, len(perms))
	require.Equal(t, JobRequest, perms[0].Resource)
	require.True(t, perms[0].Has(Submit))
	require.True(t, perms[0].Has(Execute))

	require.Equal(t, JobDefinition, perms[1].Resource)
	require.True(t, perms[1].Has(Read))
	require.True(t, perms[1].Has(Create))

	require.Equal(t, JobResource, perms[2].Resource)
	require.True(t, perms[2].Has(Delete))
}

// Verify permissions
func Test_ShouldPermissions(t *testing.T) {
	ser := DefaultPermissionsString()
	perms := NewPermissions(ser)
	require.True(t, perms.Has(JobRequest, Submit))
	require.True(t, perms.Has(JobRequest, Execute))
	require.True(t, perms.Has(JobDefinition, Create))
	require.True(t, perms.Has(JobDefinition, Read))
	require.True(t, perms.Has(Artifact, Read))
	require.True(t, perms.Has(Artifact, View))
	require.True(t, perms.Has(Artifact, Upload))
}

// Verify wild permissions
func Test_ShouldWildPermissions(t *testing.T) {
	perms := NewPermissions("*=-1")
	require.True(t, perms.Has(JobRequest, Submit))
	require.True(t, perms.Has(JobRequest, Execute))
	require.True(t, perms.Has(JobDefinition, Create))
	require.True(t, perms.Has(JobDefinition, Read))
	require.True(t, perms.Has(Artifact, View))
	require.True(t, perms.Has(Artifact, View))
	require.True(t, perms.Has(AntExecutor, Query))
}
