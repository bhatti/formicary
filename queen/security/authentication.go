package security

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// BuildToken builds JWT token
func BuildToken(
	user *common.User,
	secret string,
	age time.Duration) (strToken string, expiration time.Time, err error) {
	if user == nil {
		err = common.NewPermissionError("user is not specified for building token")
		return
	}
	expiration = time.Now().Add(age)
	claims := &web.JwtClaims{
		UserID:       user.ID,
		UserName:     user.Username,
		Name:         user.Name,
		OrgID:        user.OrganizationID,
		BundleID:     user.BundleID,
		PictureURL:   user.PictureURL,
		AuthProvider: user.AuthProvider,
		Admin:        user.IsAdmin(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiration.Unix(),
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
