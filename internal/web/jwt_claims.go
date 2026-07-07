package web

import "github.com/golang-jwt/jwt/v5"

// Token type constants distinguish API tokens (used by ants/scripts) from browser session tokens.
// The WebSocket ant endpoint requires TokenTypeAPI; session tokens are rejected.
const (
	TokenTypeSession = "session" // browser OAuth session — dashboard/API access only
	TokenTypeAPI     = "api"     // long-lived API token — ant workers and programmatic clients
)

// JwtClaims claims within JWT token
type JwtClaims struct {
	UserID          string `json:"user_id"`
	UserName        string `json:"username"`
	Name            string `json:"name"`
	OrgID           string `json:"org_id"`
	BundleID        string `json:"bundle_id"`
	PictureURL      string `json:"picture_url"`
	AuthProvider    string `json:"auth_provider"`
	Admin           bool   `json:"admin"`
	SerializedRoles string `json:"serialized_roles"`
	SerializedPerms string `json:"serialized_perms"`
	// TokenType distinguishes "session" (browser) from "api" (ant/script) tokens.
	// The WebSocket ant endpoint rejects session tokens.
	TokenType       string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}
