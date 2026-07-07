package security

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

const testSecret = "test-jwt-secret-32-bytes-long!!"

func newTestUser() *common.User {
	u := common.NewUser("org1", "testuser@example.com", "Test User", "test@example.com", acl.NewRoles(""))
	u.ID = "01TEST000000000000000000001"
	return u
}

// TestBuildAndVerifyToken verifies the full sign→parse round-trip with a valid secret.
func TestBuildAndVerifyToken(t *testing.T) {
	user := newTestUser()
	tok, expiration, err := BuildToken(user, testSecret, time.Hour, web.TokenTypeSession)
	require.NoError(t, err)
	require.NotEmpty(t, tok)
	require.True(t, expiration.After(time.Now()))

	// Token must be a three-segment JWT
	parts := strings.Split(tok, ".")
	require.Equal(t, 3, len(parts), "JWT must have header.payload.signature")

	// Verify with correct secret
	claims, err := web.ParseToken(tok, testSecret)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, user.ID, claims.UserID)
	require.Equal(t, user.Username, claims.UserName)
	require.Equal(t, user.Name, claims.Name)
	require.Equal(t, user.OrganizationID, claims.OrgID)
}

// TestBuildToken_WrongSecretFails verifies that verification fails when a different secret is used.
func TestBuildToken_WrongSecretFails(t *testing.T) {
	user := newTestUser()
	tok, _, err := BuildToken(user, testSecret, time.Hour, web.TokenTypeSession)
	require.NoError(t, err)

	_, err = web.ParseToken(tok, "wrong-secret")
	require.Error(t, err, "token signed with different secret must not verify")
}

// TestBuildToken_ExpiredFails verifies that an expired token is rejected.
func TestBuildToken_ExpiredFails(t *testing.T) {
	user := newTestUser()
	tok, _, err := BuildToken(user, testSecret, -time.Second, web.TokenTypeSession) // already expired
	require.NoError(t, err)

	_, err = web.ParseToken(tok, testSecret)
	require.Error(t, err, "expired token must not verify")
}

// TestBuildToken_NilUserFails verifies that passing a nil user returns an error.
func TestBuildToken_NilUserFails(t *testing.T) {
	_, _, err := BuildToken(nil, testSecret, time.Hour, web.TokenTypeSession)
	require.Error(t, err)
}

// TestBuildToken_EmptySecretRoundTrip verifies that a token signed with an empty secret
// can be verified with the same empty secret, but not with a different secret.
func TestBuildToken_EmptySecretRoundTrip(t *testing.T) {
	user := newTestUser()
	tok, _, err := BuildToken(user, "", time.Hour, web.TokenTypeSession)
	require.NoError(t, err)

	// Empty→empty round-trip works
	_, err = web.ParseToken(tok, "")
	require.NoError(t, err)

	// Verifying with a real secret must fail
	_, err = web.ParseToken(tok, testSecret)
	require.Error(t, err, "token signed with empty secret must not verify against non-empty secret")
}

// TestParseToken_MalformedFails verifies that a garbage string is rejected.
func TestParseToken_MalformedFails(t *testing.T) {
	_, err := web.ParseToken("not.a.jwt", testSecret)
	require.Error(t, err)
}

// TestParseToken_BearerPrefixFails verifies that passing "Bearer <token>" directly fails.
// echojwt's TokenLookup must strip the prefix before calling the JWT parser.
func TestParseToken_BearerPrefixFails(t *testing.T) {
	user := newTestUser()
	tok, _, err := BuildToken(user, testSecret, time.Hour, web.TokenTypeSession)
	require.NoError(t, err)

	_, err = web.ParseToken("Bearer "+tok, testSecret)
	require.Error(t, err, "ParseToken must not accept 'Bearer ' prefix — echojwt must strip it first")
}

// TestTokenClaimsRoundTrip verifies all claim fields survive the sign→parse cycle.
func TestTokenClaimsRoundTrip(t *testing.T) {
	u := newTestUser()
	u.OrganizationID = "org-42"
	u.BundleID = "bundle-1"
	u.PictureURL = "https://example.com/pic.jpg"
	u.AuthProvider = "google"

	tok, _, err := BuildToken(u, testSecret, 24*time.Hour, web.TokenTypeSession)
	require.NoError(t, err)

	claims, err := web.ParseToken(tok, testSecret)
	require.NoError(t, err)
	require.Equal(t, u.ID, claims.UserID)
	require.Equal(t, u.Username, claims.UserName)
	require.Equal(t, u.Name, claims.Name)
	require.Equal(t, u.OrganizationID, claims.OrgID)
	require.Equal(t, u.BundleID, claims.BundleID)
	require.Equal(t, u.PictureURL, claims.PictureURL)
	require.Equal(t, u.AuthProvider, claims.AuthProvider)
	require.Equal(t, web.TokenTypeSession, claims.TokenType)
}

// TestBuildToken_InvalidTokenType verifies that an unknown token type is rejected.
func TestBuildToken_InvalidTokenType(t *testing.T) {
	_, _, err := BuildToken(newTestUser(), testSecret, time.Hour, "unknown")
	require.Error(t, err)
}

// TestBuildToken_APITokenType verifies that TokenTypeAPI is accepted and claim survives round-trip.
func TestBuildToken_APITokenType(t *testing.T) {
	tok, _, err := BuildToken(newTestUser(), testSecret, time.Hour, web.TokenTypeAPI)
	require.NoError(t, err)

	claims, err := web.ParseToken(tok, testSecret)
	require.NoError(t, err)
	require.Equal(t, web.TokenTypeAPI, claims.TokenType)
}
