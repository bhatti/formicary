package types

import (
	"plexobject.com/formicary/internal/acl"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldFailEmailVerificationValidationWithoutEmail(t *testing.T) {
	user := common.NewUser("", "username", "", "email@formicary.io", acl.NewRoles(""))
	user.ID = "blah"
	emailVerification := NewEmailVerification("", user)
	require.Error(t, emailVerification.Validate())
	require.Contains(t, emailVerification.Validate().Error(), "email is not valid")
	require.Contains(t, emailVerification.String(), "blah")
}

func Test_ShouldFailEmailVerificationValidationWithBadEmail(t *testing.T) {
	user := common.NewUser("", "username", "", "email@formicary.io", acl.NewRoles(""))
	user.ID = "blah"
	user.OrganizationID = "blah"
	emailVerification := NewEmailVerification("bademail", user)
	emailVerification.EmailCode = ""
	emailVerification.ExpiresAt = time.Now().Add(-1 * time.Minute)
	require.Error(t, emailVerification.Validate())
	require.Contains(t, emailVerification.Validate().Error(), "email is not valid")
}

func Test_ShouldFailEmailVerificationValidationWithoutUserID(t *testing.T) {
	user := common.NewUser("org", "username", "", "email@formicary.io", acl.NewRoles(""))
	emailVerification := NewEmailVerification("good@formicary.io", user)
	require.Error(t, emailVerification.Validate())
	require.Contains(t, emailVerification.Validate().Error(), "user-id is not specified")
}

func Test_ShouldVerifyEmailVerificationValidation(t *testing.T) {
	user := common.NewUser("org", "username", "", "email@formicary.io", acl.NewRoles(""))
	user.ID = "blah"
	emailVerification := NewEmailVerification("good@formicary.io", user)
	require.NoError(t, emailVerification.Validate())
	require.Equal(t, "formicary_email_verifications", emailVerification.TableName())
}
