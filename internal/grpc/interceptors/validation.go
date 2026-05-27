// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validatable is implemented by request types that carry self-validation logic.
// Our domain _ext.go files already add Validate() to all generated proto types.
type Validatable interface {
	Validate() error
}

// Validation returns a unary interceptor that calls Validate() on any request
// that implements Validatable. Requests that don't implement the interface pass through.
func Validation() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if v, ok := req.(Validatable); ok {
			if err := v.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
			}
		}
		return handler(ctx, req)
	}
}

// ValidationStream is a no-op: stream messages arrive incrementally and must be
// validated inside the handler. Present to keep the interceptor chain symmetric.
func ValidationStream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		return handler(srv, ss)
	}
}
