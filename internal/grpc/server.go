// SPDX-License-Identifier: AGPL-3.0-or-later
// gRPC server factory: builds a grpc.Server with the standard interceptor chain.

package grpc

import (
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/grpc/interceptors"
)

// ServerConfig holds the parameters needed to build a gRPC server.
type ServerConfig struct {
	// JWTSecret is the HMAC secret used to validate bearer tokens.
	// If empty, authentication is disabled (dev/test only).
	JWTSecret string

	// CookieName is the session cookie name set after OAuth login (e.g. "formicary-session").
	// The gateway forwards the Cookie header; the auth interceptor parses it so dashboard
	// users authenticated via OAuth can also call /api/v1/* without a separate Bearer token.
	// Leave empty to disable cookie-based auth.
	CookieName string

	// RateLimitPerSecond is the per-IP request rate cap. Values ≤ 0 disable rate limiting.
	RateLimitPerSecond float64

	// RequestTimeout is the maximum duration for a unary RPC. 0 disables the timeout.
	// Recommended: 30s for production. The caller's deadline takes precedence if shorter.
	RequestTimeout time.Duration

	// MethodPermissions maps full gRPC method names to the ACL permission required.
	// Methods absent from the map are allowed without an ACL check (auth still applies).
	MethodPermissions map[string]*acl.Permission

	// UserLoader enriches JWT-claim users with full DB state.
	// If nil the interceptor falls back to claim-only users.
	UserLoader interceptors.UserLoader

	// SkipAuthMethods lists full method names that bypass JWT validation entirely.
	// Useful for health checks and unauthenticated RPCs.
	SkipAuthMethods []string

	// Logger is used by the logging interceptor. Defaults to logrus.StandardLogger().
	Logger *logrus.Logger
}

// NewServer constructs a grpc.Server with the standard interceptor stack:
//
//	Recovery → Logging → Timeout → RateLimit → Auth → Authorization → Validation
//
// The order ensures panics are caught first, then logged, then deadline-bounded,
// then rate-limited, then authenticated, then authorized, then validated.
func NewServer(cfg ServerConfig) *grpc.Server {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	unary := []grpc.UnaryServerInterceptor{
		interceptors.Recovery(),
		interceptors.Logging(logger),
	}
	stream := []grpc.StreamServerInterceptor{
		interceptors.RecoveryStream(),
		interceptors.LoggingStream(logger),
	}

	if cfg.RequestTimeout > 0 {
		unary = append(unary, interceptors.Timeout(cfg.RequestTimeout))
	}

	if cfg.RateLimitPerSecond > 0 {
		unary = append(unary, interceptors.RateLimit(cfg.RateLimitPerSecond))
		stream = append(stream, interceptors.RateLimitStream(cfg.RateLimitPerSecond))
	}

	unary = append(unary,
		interceptors.Auth(cfg.JWTSecret, cfg.CookieName, cfg.UserLoader, cfg.SkipAuthMethods...),
	)
	stream = append(stream,
		interceptors.AuthStream(cfg.JWTSecret, cfg.CookieName, cfg.UserLoader, cfg.SkipAuthMethods...),
	)

	if len(cfg.MethodPermissions) > 0 {
		unary = append(unary, interceptors.Authorization(cfg.MethodPermissions))
		stream = append(stream, interceptors.AuthorizationStream(cfg.MethodPermissions))
	}

	unary = append(unary, interceptors.Validation())
	stream = append(stream, interceptors.ValidationStream())

	return grpc.NewServer(
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.MaxRecvMsgSize(16*1024*1024),  // 16 MiB
		grpc.MaxSendMsgSize(16*1024*1024),  // 16 MiB
	)
}
