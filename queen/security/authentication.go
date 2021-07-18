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
func BuildToken(user *common.User, secret string, age time.Duration) (strToken string, expiration time.Time, err error) {
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
		Admin:        user.Admin,
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

// ParseToken parses a JWT and returns Claims object
func ParseToken(tokenString string, secret string) (*web.JwtClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if token.Valid {
		if claims, ok := token.Claims.(*web.JwtClaims); ok {
			return claims, nil
		}
		if claimsMap, ok := token.Claims.(jwt.MapClaims); ok {
			if claimsMap["user_id"] == nil ||
				claimsMap["username"] == nil ||
				claimsMap["name"] == nil ||
				claimsMap["org_id"] == nil ||
				claimsMap["bundle_id"] == nil ||
				claimsMap["picture_url"] == nil ||
				claimsMap["admin"] == nil {
				return nil, fmt.Errorf("invalid token %v", claimsMap)
			}
			claims := &web.JwtClaims{
				UserID:       claimsMap["user_id"].(string),
				UserName:     claimsMap["username"].(string),
				Name:         claimsMap["name"].(string),
				OrgID:        claimsMap["org_id"].(string),
				BundleID:     claimsMap["bundle_id"].(string),
				PictureURL:   claimsMap["picture_url"].(string),
				AuthProvider: claimsMap["auth_provider"].(string),
				Admin:        claimsMap["admin"].(bool),
			}
			return claims, nil
		}
		return nil, fmt.Errorf("unknown claims for token %v", token.Claims)
	} else if ve, ok := err.(*jwt.ValidationError); ok {
		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, err
		} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
			// Token is either expired or not active yet
			return nil, err
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
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
		return nil, fmt.Errorf("unable to get the public key. Error: %s", err.Error())
	}

	return pkey, nil
}
