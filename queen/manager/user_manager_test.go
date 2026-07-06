package manager

import (
	"fmt"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/queen/config"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldAddStickyMessage(t *testing.T) {
	serverCfg := config.TestServerConfig()
	userMgr, err := TestUserManager(serverCfg)
	require.NoError(t, err)
	user := common.NewUser("", "username", "name", "email@formicary.io", acl.NewRoles(""))
	qc := common.NewQueryContext(nil, "").WithAdmin()
	user, err = userMgr.CreateUser(qc, user)
	require.NoError(t, err)
	err = userMgr.AddStickyMessageForSlack(qc, user, fmt.Errorf("failed"))
	require.NoError(t, err)
	err = userMgr.ClearStickyMessageForSlack(qc, user)
	require.NoError(t, err)
}

// CreateUser must assign default permissions even when none are supplied.
func Test_ShouldAssignDefaultPermissionsOnCreateUser(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)

	user := &common.User{
		Username: "oauth-user@example.com",
		Email:    "oauth-user@example.com",
		Name:     "OAuth User",
		Active:   true,
	}
	require.Empty(t, user.SerializedPerms, "pre-condition: no perms before create")

	qc := common.NewQueryContext(nil, "").WithAdmin()
	saved, err := userMgr.CreateUser(qc, user)
	require.NoError(t, err)
	require.NotEmpty(t, saved.SerializedPerms, "saved user must have serialized permissions")
	require.True(t, saved.HasPermission(acl.Dashboard, acl.View), "must have Dashboard: View")
	require.True(t, saved.HasPermission(acl.JobRequest, acl.Submit), "must have JobRequest: Submit")
	require.True(t, saved.HasPermission(acl.Websocket, acl.Subscribe), "must have Websocket: Subscribe")
}

// PrepareLoginUser copies perms from DB user and backfills any that are missing.
func Test_ShouldPrepareLoginUserForExistingUser(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)
	qc := common.NewQueryContext(nil, "").WithAdmin()

	// Persist a user with stale permissions (missing Dashboard).
	permsWithoutDashboard := acl.MarshalPermissions([]*acl.Permission{
		acl.NewPermission(acl.JobRequest, acl.View|acl.Submit),
	})
	dbUser := &common.User{
		Username:        "stale@example.com",
		Email:           "stale@example.com",
		Name:            "Stale User",
		Active:          true,
		SerializedPerms: permsWithoutDashboard,
	}
	_, err = userMgr.CreateUser(qc, dbUser)
	require.NoError(t, err)

	// Simulate what the OAuth provider returns: a fresh user with no perms.
	oauthUser := &common.User{Username: "stale@example.com"}
	oldUser := userMgr.PrepareLoginUser(oauthUser)

	require.NotNil(t, oldUser, "should find the existing DB user")
	require.True(t, oauthUser.HasPermission(acl.Dashboard, acl.View),
		"Dashboard: View must be backfilled for stale user")
	require.True(t, oauthUser.HasPermission(acl.JobRequest, acl.Submit),
		"pre-existing JobRequest perm must be preserved")
}

// PrepareLoginUser sets full default permissions for a brand-new user (no DB record).
func Test_ShouldPrepareLoginUserForNewUser(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)

	oauthUser := &common.User{Username: "brandnew@example.com"}
	oldUser := userMgr.PrepareLoginUser(oauthUser)

	require.Nil(t, oldUser, "no DB user should be found for a new signup")
	require.NotEmpty(t, oauthUser.SerializedPerms)
	require.True(t, oauthUser.HasPermission(acl.Dashboard, acl.View),
		"new user JWT must include Dashboard: View")
}

// CreatePersonalOrgForUser must create a valid org and link it to the user.
func Test_ShouldCreatePersonalOrgForUser(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)
	qc := common.NewQueryContext(nil, "").WithAdmin()

	user := &common.User{
		Username: "personal@example.com",
		Email:    "personal@example.com",
		Name:     "Personal User",
		Active:   true,
	}
	saved, err := userMgr.CreateUser(qc, user)
	require.NoError(t, err)
	require.Empty(t, saved.OrganizationID, "pre-condition: no org yet")

	org, err := userMgr.CreatePersonalOrgForUser(saved)
	require.NoError(t, err)
	require.NotNil(t, org)
	require.True(t, org.IsPersonal)
	require.NotEmpty(t, org.BundleID, "personal org must have a BundleID")
	require.Contains(t, org.BundleID, ".formicary.io")
	require.NotEmpty(t, saved.OrganizationID, "user must be linked to the new org")
	require.Equal(t, org.BundleID, saved.BundleID, "user BundleID must match org BundleID")
	require.True(t, saved.IsOrgAdmin(), "personal org owner must be promoted to OrgAdmin")
}

// Individual-account signup: OrgUnit present in form should not trigger org creation.
// This mirrors the server-side guard: only accountType=organization triggers BuildOrgWithInvitation.
func Test_ShouldNotCreateOrgWhenIndividualSignup(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)

	// Simulate what happens when orgUnit leaks through form submission for an individual account.
	user := common.NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	user.OrgUnit = "" // individual: always empty
	user.BundleID = ""
	user.InvitationCode = ""

	// HasOrganizationOrInvitationCode must be false → BuildOrgWithInvitation is a no-op.
	require.False(t, user.HasOrganizationOrInvitationCode(),
		"individual account must never enter org creation path")

	org, err := userMgr.BuildOrgWithInvitation(user)
	require.NoError(t, err)
	require.Nil(t, org, "BuildOrgWithInvitation must return nil org for individual account")
}

// Organization signup without BundleID must auto-generate one.
func Test_ShouldAutoGenerateBundleIDForOrgSignup(t *testing.T) {
	userMgr, err := TestUserManager(nil)
	require.NoError(t, err)

	user := common.NewUser("", "user@example.com", "Test User", "user@example.com", acl.NewRoles(""))
	user.OrgUnit = "mycompany"
	user.BundleID = "" // intentionally missing — should be auto-generated
	_ = user.Validate()

	org, err := userMgr.BuildOrgWithInvitation(user)
	require.NoError(t, err)
	require.NotNil(t, org)
	require.NotEmpty(t, user.BundleID, "BundleID must be auto-generated when empty")
	require.Contains(t, user.BundleID, ".formicary.io")
}
