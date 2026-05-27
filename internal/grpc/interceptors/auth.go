// SPDX-License-Identifier: AGPL-3.0-or-later
// Auth interceptor: validates JWT from gRPC metadata, populates User in context.
//
// Token extraction order:
//  1. Authorization: Bearer <token> header (API clients, curl, SDKs).
//  2. Session cookie (dashboard users authenticated via OAuth — cookie forwarded
//     from HTTP by the grpc-gateway incomingHeaderMatcher).

package interceptors

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// UserLoader loads and enriches a User from the database by username.
// Implementations call the user repository and merge org/subscription data.
type UserLoader interface {
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
}

// Auth returns a unary interceptor that:
//  1. Extracts the JWT from the "authorization" header (Bearer) or session cookie.
//  2. Validates it with the provided HMAC secret.
//  3. Optionally loads the full DB user via UserLoader.
//  4. Attaches the user to the context.
//
// cookieName is the session cookie set after OAuth login (e.g. "formicary-session").
// Pass an empty string to disable cookie fallback.
// Methods listed in skipMethods bypass authentication entirely (e.g. health checks).
func Auth(secret, cookieName string, loader UserLoader, skipMethods ...string) grpc.UnaryServerInterceptor {
	skip := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = true
	}
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}
		// Auth disabled (secret empty) — allow all requests as anonymous admin.
		// This only happens in dev/test; production always has a non-empty secret.
		if secret == "" {
			ctx = WithUser(ctx, anonAdminUser())
			ctx = WithQueryContext(ctx, types.NewQueryContext(UserFromContext(ctx), peerAddr(ctx)))
			return handler(ctx, req)
		}
		tokenStr, err := extractToken(ctx, cookieName)
		if err != nil {
			return nil, err
		}
		claims, err := web.ParseToken(tokenStr, secret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		user := NewUserFromClaims(claims)

		// Enrich from DB when a loader is available.
		if loader != nil {
			dbUser, dbErr := loader.GetUserByUsername(ctx, claims.UserName)
			if dbErr != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "AuthInterceptor",
					"Username":  claims.UserName,
					"Error":     dbErr,
				}).Warn("failed to load DB user; falling back to JWT claims")
			} else if dbUser != nil {
				if dbUser.Locked {
					return nil, status.Errorf(codes.PermissionDenied, "account is locked")
				}
				user = dbUser
			}
		}
		ctx = WithUser(ctx, user)
		ctx = WithQueryContext(ctx, types.NewQueryContext(user, peerAddr(ctx)))
		return handler(ctx, req)
	}
}

// AuthStream returns a stream interceptor counterpart to Auth.
func AuthStream(secret, cookieName string, loader UserLoader, skipMethods ...string) grpc.StreamServerInterceptor {
	skip := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = true
	}
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if skip[info.FullMethod] {
			return handler(srv, ss)
		}
		if secret == "" {
			u := anonAdminUser()
			wrapped := &wrappedStream{
				ServerStream: ss,
				ctx: WithQueryContext(
					WithUser(ss.Context(), u),
					types.NewQueryContext(u, peerAddr(ss.Context())),
				),
			}
			return handler(srv, wrapped)
		}
		tokenStr, err := extractToken(ss.Context(), cookieName)
		if err != nil {
			return err
		}
		claims, err := web.ParseToken(tokenStr, secret)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		user := NewUserFromClaims(claims)
		if loader != nil {
			if dbUser, dbErr := loader.GetUserByUsername(ss.Context(), claims.UserName); dbErr == nil && dbUser != nil {
				if dbUser.Locked {
					return status.Errorf(codes.PermissionDenied, "account is locked")
				}
				user = dbUser
			}
		}
		wrapped := &wrappedStream{
			ServerStream: ss,
			ctx: WithQueryContext(
				WithUser(ss.Context(), user),
				types.NewQueryContext(user, peerAddr(ss.Context())),
			),
		}
		return handler(srv, wrapped)
	}
}

// extractToken returns the raw JWT string by checking, in order:
//  1. Authorization: Bearer <token> or Token <token> metadata header.
//  2. The named session cookie forwarded from HTTP by the gateway.
func extractToken(ctx context.Context, cookieName string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "no metadata in request")
	}

	// 1. Authorization header (Bearer or Token prefix).
	if values := md.Get("authorization"); len(values) > 0 {
		token := values[0]
		upper := strings.ToUpper(token)
		switch {
		case strings.HasPrefix(upper, "BEARER "):
			token = token[7:]
		case strings.HasPrefix(upper, "TOKEN "):
			token = token[6:]
		}
		if token != "" {
			return token, nil
		}
	}

	// 2. Session cookie (OAuth-authenticated dashboard users).
	// The gateway forwards the raw Cookie header; parse it manually so we don't
	// depend on net/http.Cookie (which requires a full http.Request).
	if cookieName != "" {
		if cookies := md.Get("cookie"); len(cookies) > 0 {
			for _, header := range cookies {
				for _, part := range strings.Split(header, ";") {
					kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
					if len(kv) == 2 && kv[0] == cookieName && kv[1] != "" {
						return kv[1], nil
					}
				}
			}
		}
	}

	return "", status.Error(codes.Unauthenticated,
		"authentication required: provide Authorization: Bearer <token> header or a valid session cookie")
}

// wrappedStream overrides Context() on a ServerStream to carry enriched context values.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
