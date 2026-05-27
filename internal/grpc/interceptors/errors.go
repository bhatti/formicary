// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"plexobject.com/formicary/internal/types"
)

// MapDomainError converts a domain error to the appropriate gRPC status error.
// Callers should pass the raw error returned by manager/repository methods.
func MapDomainError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *types.NotFoundError:
		return status.Errorf(codes.NotFound, "%s", err.Error())
	case *types.PermissionError:
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	case *types.QuotaExceededError:
		return status.Errorf(codes.ResourceExhausted, "%s", err.Error())
	case *types.ValidationError:
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	case *types.DuplicateError:
		return status.Errorf(codes.AlreadyExists, "%s", err.Error())
	case *types.ConflictError:
		return status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	default:
		return status.Errorf(codes.Internal, "%s", err.Error())
	}
}
