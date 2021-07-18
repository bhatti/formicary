package web

import "github.com/dgrijalva/jwt-go"

// JwtClaims claims within JWT token
type JwtClaims struct {
	UserID       string `json:"user_id"`
	UserName     string `json:"username"`
	Name         string `json:"name"`
	OrgID        string `json:"org_id"`
	BundleID     string `json:"bundle_id"`
	PictureURL   string `json:"picture_url"`
	AuthProvider string `json:"auth_provider"`
	Admin        bool   `json:"admin"`
	jwt.StandardClaims
}
