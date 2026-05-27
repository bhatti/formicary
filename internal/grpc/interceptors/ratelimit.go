// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"context"
	"math"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RateLimit returns a unary interceptor that enforces a per-IP token-bucket
// rate limit matching the existing tollbooth setup in web_server.go.
func RateLimit(ratePerSecond float64) grpc.UnaryServerInterceptor {
	lmt := newLimiter(ratePerSecond)
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ip := peerAddr(ctx)
		if ip != "" {
			if lmt.LimitReached(ip) {
				return nil, status.Errorf(codes.ResourceExhausted,
					"rate limit exceeded (%.0f req/s)", ratePerSecond)
			}
		}
		return handler(ctx, req)
	}
}

// RateLimitStream returns a stream interceptor with the same rate limit.
func RateLimitStream(ratePerSecond float64) grpc.StreamServerInterceptor {
	lmt := newLimiter(ratePerSecond)
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ip := peerAddr(ss.Context())
		if ip != "" && lmt.LimitReached(ip) {
			return status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded (%.0f req/s)", ratePerSecond)
		}
		return handler(srv, ss)
	}
}

func newLimiter(ratePerSecond float64) *limiter.Limiter {
	rate := math.Max(ratePerSecond, 1)
	lmt := tollbooth.NewLimiter(rate, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	})
	lmt.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
	return lmt
}
