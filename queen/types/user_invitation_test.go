package types

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldFailUserInvitationValidationWithoutEmail(t *testing.T) {
	user := common.NewUser("", "username", "", false)
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
	require.Contains(t, invitation.String(), "blah")
}

func Test_ShouldFailUserInvitationValidationWithBadEmail(t *testing.T) {
	user := common.NewUser("", "username", "", false)
	user.ID = "blah"
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("bademail", user)
	invitation.InvitationCode = ""
	invitation.ExpiresAt = time.Now().Add(-1 * time.Minute)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "email is not valid")
}

func Test_ShouldFailUserInvitationValidationWithoutUserID(t *testing.T) {
	user := common.NewUser("org", "username", "", false)
	user.OrganizationID = "blah"
	invitation := NewUserInvitation("good@email.com", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "invited-by-user is not specified")
}

func Test_ShouldFailUserInvitationValidationWithoutOrganization(t *testing.T) {
	user := common.NewUser("", "username", "", false)
	user.ID = "blah"
	invitation := NewUserInvitation("good@email.com", user)
	require.Error(t, invitation.Validate())
	require.Contains(t, invitation.Validate().Error(), "org is not specified")
}

func Test_ShouldVerifyUserInvitationValidation(t *testing.T) {
	user := common.NewUser("org", "username", "", false)
	user.ID = "blah"
	invitation := NewUserInvitation("good@email.com", user)
	require.NoError(t, invitation.Validate())
	require.Equal(t, "formicary_user_invitations", invitation.TableName())
}
