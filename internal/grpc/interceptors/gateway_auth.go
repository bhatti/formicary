// SPDX-License-Identifier: AGPL-3.0-or-later
// HTTP middleware that replicates Auth interceptor logic for grpc-gateway in-process calls.
//
// When grpc-gateway uses RegisterHandlerServer (in-process), it bypasses the gRPC interceptor
// chain entirely. This middleware wraps the gateway HTTP handler and performs the same auth
// logic — injecting User and QueryContext into the request context — so service handlers
// can call QueryContextFromContext(ctx) successfully.

package interceptors

import (
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// GatewayAuthMiddleware returns an HTTP middleware that authenticates requests arriving
// via grpc-gateway (which bypasses gRPC interceptors when using RegisterHandlerServer).
//
// Logic mirrors Auth() interceptor:
//   - Empty secret → anonymous admin (auth disabled, dev/test).
//   - Authorization: Bearer/Token header → JWT validation.
//   - Cookie fallback → named session cookie (OAuth dashboard users).
//
// skipPaths lists exact URL paths that bypass auth (e.g. "/api/v1/health", "/api/v1/ping").
func GatewayAuthMiddleware(
	secret, cookieName string,
	loader UserLoader,
	skipPaths ...string,
) func(http.Handler) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for whitelisted paths.
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Auth disabled — inject anonymous admin context.
			if secret == "" {
				u := anonAdminUser()
				ctx = WithUser(ctx, u)
				ctx = WithQueryContext(ctx, types.NewQueryContext(u, r.RemoteAddr))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Extract token from Authorization header or session cookie.
			tokenStr, err := extractTokenFromHTTP(r, cookieName)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"authentication required","code":"UNAUTHENTICATED"}`))
				return
			}

			claims, err := web.ParseToken(tokenStr, secret)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid token","code":"UNAUTHENTICATED"}`))
				return
			}

			user := NewUserFromClaims(claims)
			if loader != nil {
				dbUser, dbErr := loader.GetUserByUsername(ctx, claims.UserName)
				if dbErr != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "GatewayAuthMiddleware",
						"Username":  claims.UserName,
						"Error":     dbErr,
					}).Warn("failed to load DB user; falling back to JWT claims")
				} else if dbUser != nil {
					if dbUser.Locked {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"error":"account is locked","code":"PERMISSION_DENIED"}`))
						return
					}
					user = dbUser
				}
			}

			ctx = WithUser(ctx, user)
			ctx = WithQueryContext(ctx, types.NewQueryContext(user, r.RemoteAddr))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractTokenFromHTTP extracts the raw JWT from an HTTP request:
//  1. Authorization: Bearer/Token header.
//  2. Named session cookie (OAuth dashboard users).
func extractTokenFromHTTP(r *http.Request, cookieName string) (string, error) {
	if authHdr := r.Header.Get("Authorization"); authHdr != "" {
		upper := strings.ToUpper(authHdr)
		var tok string
		switch {
		case strings.HasPrefix(upper, "BEARER "):
			tok = strings.TrimSpace(authHdr[7:])
		case strings.HasPrefix(upper, "TOKEN "):
			tok = strings.TrimSpace(authHdr[6:])
		default:
			tok = strings.TrimSpace(authHdr)
		}
		if tok != "" {
			return tok, nil
		}
	}

	if cookieName != "" {
		if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
			return c.Value, nil
		}
	}

	return "", errUnauthenticated
}

var errUnauthenticated = &authError{"authentication required"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
