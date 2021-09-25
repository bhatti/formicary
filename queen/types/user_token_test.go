package types

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldFailUserTokenValidationWithoutUserID(t *testing.T) {
	token := NewUserToken(&common.User{ID: "", OrganizationID: "456"}, "tok-name")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "user-id")
}

func Test_ShouldFailUserTokenValidationWithoutTokenName(t *testing.T) {
	token := NewUserToken(&common.User{ID: "123", OrganizationID: "456"}, "")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "token-name")
}

func Test_ShouldFailUserTokenValidationWithoutAPIToken(t *testing.T) {
	token := NewUserToken(&common.User{ID: "123", OrganizationID: "456"}, "tok-name")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "api token")
}

func Test_ShouldFailUserTokenValidationWithoutExpiration(t *testing.T) {
	token := NewUserToken(&common.User{ID: "123", OrganizationID: "456"}, "tok-name")
	require.Error(t, token.Validate())
	token.APIToken = "blah"
	require.Contains(t, token.Validate().Error(), "expires-at")
}

func Test_ShouldVerifyUserTokenValidation(t *testing.T) {
	token := NewUserToken(&common.User{ID: "123", OrganizationID: "456"}, "tok-name")
	token.APIToken = "blah"
	token.ExpiresAt = time.Now().Add(time.Hour)
	require.NoError(t, token.Validate())
	require.Equal(t, "formicary_user_tokens", token.TableName())
}
