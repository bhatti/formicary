// SPDX-License-Identifier: AGPL-3.0-or-later
// Context keys and helpers for propagating user/query-context through gRPC requests.

package interceptors

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

type contextKey int

const (
	contextKeyUser contextKey = iota
	contextKeyQueryContext
)

// WithUser stores an authenticated User in the context.
func WithUser(ctx context.Context, user *types.User) context.Context {
	return context.WithValue(ctx, contextKeyUser, user)
}

// UserFromContext retrieves the authenticated User, or nil if absent.
func UserFromContext(ctx context.Context) *types.User {
	u, _ := ctx.Value(contextKeyUser).(*types.User)
	return u
}

// WithQueryContext stores a QueryContext in the context.
func WithQueryContext(ctx context.Context, qc *types.QueryContext) context.Context {
	return context.WithValue(ctx, contextKeyQueryContext, qc)
}

// QueryContextFromContext retrieves the QueryContext, or nil if absent.
func QueryContextFromContext(ctx context.Context) *types.QueryContext {
	qc, _ := ctx.Value(contextKeyQueryContext).(*types.QueryContext)
	return qc
}

// BuildQueryContext constructs a QueryContext from the authenticated user and
// the peer address embedded in ctx by gRPC.
func BuildQueryContext(ctx context.Context) *types.QueryContext {
	user := UserFromContext(ctx)
	return types.NewQueryContext(user, peerAddr(ctx))
}

// NewUserFromClaims converts JWT claims to a lightweight User for auth checks.
// The user is not DB-enriched; auth interceptor enriches it separately.
func NewUserFromClaims(claims *web.JwtClaims) *types.User {
	roles := acl.NewRoles("")
	if claims.Admin {
		roles = acl.NewRolesWithAdmin()
	}
	// NewUser(orgID, username, name, email, roles)
	u := types.NewUser(claims.OrgID, claims.UserName, claims.Name, "", roles)
	u.ID = claims.UserID
	u.BundleID = claims.BundleID
	return u
}

// anonAdminUser returns a minimal admin-role User used when auth is disabled.
// This ensures the Authorization interceptor doesn't block requests in dev mode.
func anonAdminUser() *types.User {
	u := types.NewUser("", "anonymous", "anonymous", "", acl.NewRolesWithAdmin())
	return u
}

func peerAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		if host, _, err := net.SplitHostPort(p.Addr.String()); err == nil {
			return host
		}
		return p.Addr.String()
	}
	return ""
}
