package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldFailUserTokenValidationWithoutUserID(t *testing.T) {
	token := NewUserToken("", "org-id", "tok-name")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "user-id")
}

func Test_ShouldFailUserTokenValidationWithoutTokenName(t *testing.T) {
	token := NewUserToken("user-id", "org-id", "")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "token-name")
}

func Test_ShouldFailUserTokenValidationWithoutAPIToken(t *testing.T) {
	token := NewUserToken("user-id", "org-id", "tok-name")
	require.Error(t, token.Validate())
	require.Contains(t, token.Validate().Error(), "api token")
}

func Test_ShouldFailUserTokenValidationWithoutExpiration(t *testing.T) {
	token := NewUserToken("user-id", "org-id", "tok-name")
	require.Error(t, token.Validate())
	token.APIToken = "blah"
	require.Contains(t, token.Validate().Error(), "expires-at")
}

func Test_ShouldVerifyUserTokenValidation(t *testing.T) {
	token := NewUserToken("user-id", "org-id", "tok-name")
	token.APIToken = "blah"
	token.ExpiresAt = time.Now().Add(time.Hour)
	require.NoError(t, token.Validate())
	require.Equal(t, "formicary_user_tokens", token.TableName())
}
