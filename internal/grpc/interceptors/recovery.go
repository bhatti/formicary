// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"context"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery returns a unary interceptor that recovers from panics and returns
// an Internal gRPC error, preventing the server from crashing.
func Recovery() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "RecoveryInterceptor",
					"Method":    info.FullMethod,
					"Panic":     r,
					"Stack":     string(debug.Stack()),
				}).Error("panic recovered in gRPC handler")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// RecoveryStream returns a stream interceptor that recovers from panics.
func RecoveryStream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "RecoveryStreamInterceptor",
					"Method":    info.FullMethod,
					"Panic":     r,
					"Stack":     string(debug.Stack()),
				}).Error("panic recovered in gRPC stream handler")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}
