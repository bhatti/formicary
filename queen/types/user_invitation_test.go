package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldFailUserInvitationValidationWithoutEmail(t *testing.T) {
	org := common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user := common.NewUser("", "username", "", "mail@formicary.io", false)
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("", user, org)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
	require.Contains(t, invitation.String(), "blah")
}

func Test_ShouldFailUserInvitationValidationWithBadEmail(t *testing.T) {
	org := common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user := common.NewUser("", "username", "", "mail@formicary.io", false)
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("bad-email", user, org)
	invitation.InvitationCode = ""
	invitation.ExpiresAt = time.Now().Add(-1 * time.Minute)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
}

func Test_ShouldFailUserInvitationValidationWithoutUserID(t *testing.T) {
	org := common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user := common.NewUser("org", "username", "", "mail@formicary.io", false)
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user, org)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "invited-by-user is not specified")
}

func Test_ShouldFailUserInvitationValidationWithoutOrganization(t *testing.T) {
	org := common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user := common.NewUser("", "username", "", "mail@formicary.io", false)
	user.ID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user, org)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "org is not specified")
}

func Test_ShouldVerifyUserInvitationValidation(t *testing.T) {
	org := common.NewOrganization("owner-user-id", "my-unit", "my-bundle")
	user := common.NewUser("org", "username", "", "mail@formicary.io", false)
	user.ID = "blah"
	invitation := NewUserInvitation("good@formicary.io", user, org)
	require.NoError(t, invitation.Validate())
	require.Equal(t, "formicary_user_invitations", invitation.TableName())
}

func Test_ShouldCreateRandomString(t *testing.T) {
	r := randomString(20)
	require.Equal(t, 40, len(r))
}
