// SPDX-License-Identifier: AGPL-3.0-or-later
// Timeout interceptor: enforces a per-RPC deadline on unary handlers.
// Stream RPCs are excluded — they manage their own lifetime.

package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Timeout returns a unary interceptor that enforces a maximum handler duration.
// If the caller already has a shorter deadline, that deadline takes precedence.
// Pass 0 to disable.
//
// The handler runs in the current goroutine under a derived context. When the
// deadline fires, context.WithTimeout cancels the derived context, which propagates
// cancellation to any blocking DB/queue calls inside the handler — they return
// context.DeadlineExceeded and the goroutine unwinds naturally. No goroutine leak.
func Timeout(d time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if d <= 0 {
			return handler(ctx, req)
		}
		// Respect a shorter caller-supplied deadline.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= d {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		resp, err := handler(ctx, req)
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			return nil, status.Errorf(codes.DeadlineExceeded, "request timed out after %s", d)
		}
		return resp, err
	}
}
