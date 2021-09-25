package types

import (
	"plexobject.com/formicary/internal/acl"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldFailUserInvitationValidationWithoutEmail(t *testing.T) {
	user := common.NewUser("", "username", "", "mail@formicary.io", acl.NewRoles(""))
	user.Organization = common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
	require.Contains(t, invitation.String(), "blah")
}

func Test_ShouldFailUserInvitationValidationWithBadEmail(t *testing.T) {
	user := common.NewUser("", "username", "", "mail@formicary.io", acl.NewRoles(""))
	user.Organization = common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("bad-email", user)
	invitation.InvitationCode = ""
	invitation.ExpiresAt = time.Now().Add(-1 * time.Minute)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
}

func Test_ShouldFailUserInvitationValidationWithoutUserID(t *testing.T) {
	user := common.NewUser("org", "username", "", "mail@formicary.io", acl.NewRoles(""))
	user.Organization = common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "invited-by-user is not specified")
}

func Test_ShouldFailUserInvitationValidationWithoutOrganization(t *testing.T) {
	user := common.NewUser("", "username", "", "mail@formicary.io", acl.NewRoles(""))
	user.Organization = common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user.ID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "org-unit is not specified")
}

func Test_ShouldVerifyUserInvitationValidation(t *testing.T) {
	user := common.NewUser("org", "username", "", "mail@formicary.io", acl.NewRoles(""))
	user.Organization = common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user.ID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user)
	require.NoError(t, invitation.Validate())
	require.Equal(t, "formicary_user_invitations", invitation.TableName())
}

func Test_ShouldCreateRandomString(t *testing.T) {
	r := randomString(20)
	require.Equal(t, 40, len(r))
}
