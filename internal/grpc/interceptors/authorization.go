// SPDX-License-Identifier: AGPL-3.0-or-later

package interceptors

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"plexobject.com/formicary/internal/acl"
)

// Authorization returns a unary interceptor that checks ACL permissions.
// methodPermissions maps full gRPC method names to the required permission.
// Methods not in the map are allowed through (auth still required via Auth interceptor).
// Admins bypass all resource-level ACL checks.
func Authorization(methodPermissions map[string]*acl.Permission) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		perm, ok := methodPermissions[info.FullMethod]
		if !ok || perm == nil {
			return handler(ctx, req)
		}
		user := UserFromContext(ctx)
		if user == nil {
			return nil, status.Errorf(codes.Unauthenticated, "no authenticated user")
		}
		// Admins pass all ACL checks.
		if user.IsAdmin() {
			return handler(ctx, req)
		}
		// Read-only actions: read-admins and users with explicit permission both pass.
		if perm.ReadOnly() {
			if !user.IsReadAdmin() && !user.HasPermission(perm.Resource, perm.Actions) {
				return nil, status.Errorf(codes.PermissionDenied,
					"read permission required: %s", perm.String())
			}
			return handler(ctx, req)
		}
		if !user.HasPermission(perm.Resource, perm.Actions) {
			return nil, status.Errorf(codes.PermissionDenied,
				"permission denied: %s", perm.String())
		}
		return handler(ctx, req)
	}
}

// AuthorizationStream returns a stream interceptor for ACL permission checks.
func AuthorizationStream(methodPermissions map[string]*acl.Permission) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		perm, ok := methodPermissions[info.FullMethod]
		if !ok || perm == nil {
			return handler(srv, ss)
		}
		user := UserFromContext(ss.Context())
		if user == nil {
			return status.Errorf(codes.Unauthenticated, "no authenticated user")
		}
		if user.IsAdmin() {
			return handler(srv, ss)
		}
		if perm.ReadOnly() {
			if !user.IsReadAdmin() && !user.HasPermission(perm.Resource, perm.Actions) {
				return status.Errorf(codes.PermissionDenied,
					"read permission required: %s", perm.String())
			}
			return handler(srv, ss)
		}
		if !user.HasPermission(perm.Resource, perm.Actions) {
			return status.Errorf(codes.PermissionDenied,
				"permission denied: %s", perm.String())
		}
		return handler(srv, ss)
	}
}
