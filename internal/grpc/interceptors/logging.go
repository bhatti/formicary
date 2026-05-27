// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Logging returns a unary interceptor that logs each request with method,
// duration, status code, and user ID.
func Logging(logger *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		entry := logger.WithFields(logrus.Fields{
			"Component": "gRPC",
			"Method":    info.FullMethod,
			"Code":      code.String(),
			"LatencyMs": time.Since(start).Milliseconds(),
		})
		if user := UserFromContext(ctx); user != nil {
			entry = entry.WithField("UserID", user.ID)
		}
		if err != nil {
			entry.WithError(err).Warn("gRPC request failed")
		} else {
			entry.Debug("gRPC request completed")
		}
		return resp, err
	}
}

// LoggingStream returns a stream interceptor that logs stream lifecycle.
func LoggingStream(logger *logrus.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		logger.WithFields(logrus.Fields{
			"Component": "gRPC",
			"Method":    info.FullMethod,
			"Code":      code.String(),
			"LatencyMs": time.Since(start).Milliseconds(),
		}).Debug("gRPC stream completed")
		return err
	}
}
