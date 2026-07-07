package security

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// BuildToken builds a signed JWT. tokenType must be web.TokenTypeSession or web.TokenTypeAPI.
// Session tokens are issued at OAuth login; API tokens are issued via the API-token management UI.
// The WebSocket ant endpoint rejects session tokens — ants must use API tokens.
func BuildToken(
	user *common.User,
	secret string,
	age time.Duration,
	tokenType string) (strToken string, expiration time.Time, err error) {
	if user == nil {
		err = common.NewPermissionError("user is not specified for building token")
		return
	}
	if tokenType != web.TokenTypeSession && tokenType != web.TokenTypeAPI {
		err = common.NewValidationError("token_type must be 'session' or 'api'")
		return
	}
	expiration = time.Now().Add(age)
	claims := &web.JwtClaims{
		UserID:          user.ID,
		UserName:        user.Username,
		Name:            user.Name,
		OrgID:           user.OrganizationID,
		BundleID:        user.BundleID,
		PictureURL:      user.PictureURL,
		AuthProvider:    user.AuthProvider,
		Admin:           user.IsAdmin(),
		SerializedRoles: user.SerializedRoles,
		SerializedPerms: user.SerializedPerms,
		TokenType:       tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
		},
	}

	logrus.WithFields(logrus.Fields{
		"Component":      "BuildToken",
		"UserID":         user.ID,
		"Name":           user.Name,
		"UserName":       user.Username,
		"OrganizationID": user.OrganizationID,
		"BundleID":       user.BundleID,
		"PictureURL":     user.PictureURL,
		"AuthProvider":   user.AuthProvider,
	}).Infof("logged in")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	strToken, err = token.SignedString([]byte(secret))
	return
}

func getKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	// See https://developers.google.com/identity/sign-in/web/backend-auth
	// This user-defined KeyFunc verifies tokens issued by Google Sign-In.
	keySet, err := jwk.Fetch(ctx, "https://www.googleapis.com/oauth2/v3/certs")
	if err != nil {
		return nil, err
	}

	keyID, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("expecting JWT header to have a key ID in the kid field")
	}

	key, found := keySet.LookupKeyID(keyID)

	if !found {
		return nil, fmt.Errorf("unable to find key %q", keyID)
	}

	var pkey interface{}
	if err := key.Raw(&pkey); err != nil {
		return nil, fmt.Errorf("unable to get the public key due to %w", err)
	}

	return pkey, nil
}
