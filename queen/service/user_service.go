// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	commonTypes "plexobject.com/formicary/internal/types"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoUser "plexobject.com/formicary/gen/go/formicary/v1/user"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/security"
)

// UserService implements svcpb.UserServiceServer.
type UserService struct {
	svcpb.UnimplementedUserServiceServer
	userManager    *manager.UserManager
	userRepository repository.UserRepository
	cfg            *config.ServerConfig
}

// NewUserService creates a UserService.
func NewUserService(
	userManager *manager.UserManager,
	userRepository repository.UserRepository,
	cfg *config.ServerConfig,
) *UserService {
	return &UserService{
		userManager:    userManager,
		userRepository: userRepository,
		cfg:            cfg,
	}
}

// Login issues a JWT token for an already-authenticated caller.
// It requires a valid session: the caller must already have a JWT (from OAuth or an
// API token) in their Authorization header or session cookie. The interceptor validates
// it before this handler runs. This endpoint simply re-issues a fresh token and returns
// the caller's full profile — it is NOT an unauthenticated password login.
func (s *UserService) Login(ctx context.Context, _ *svcpb.LoginRequest) (*svcpb.LoginResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no authenticated session")
	}
	dbUser, err := s.userRepository.GetByUsername(qc, qc.GetUsername())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}
	if !dbUser.Active {
		return nil, status.Error(codes.PermissionDenied, "account is inactive")
	}
	if dbUser.Locked {
		return nil, status.Error(codes.PermissionDenied, "account is locked")
	}
	tokenAge := s.cfg.Common.Auth.TokenMaxAge
	if tokenAge <= 0 {
		tokenAge = 30 * 24 * time.Hour
	}
	token, _, err := security.BuildToken(dbUser, s.cfg.Common.Auth.JWTSecret, tokenAge)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.LoginResponse{
		Token: token,
		User:  toProtoUser(dbUser),
	}, nil
}

func (s *UserService) GetProfile(ctx context.Context, _ *emptypb.Empty) (*svcpb.GetUserResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	u, err := s.userManager.GetUser(qc, qc.GetUserID())
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetUserResponse{User: toProtoUser(u)}, nil
}

func (s *UserService) QueryUsers(ctx context.Context, req *svcpb.QueryUsersRequest) (*svcpb.QueryUsersResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.Username != "" {
		params["username"] = req.Username
	}
	if req.Email != "" {
		params["email"] = req.Email
	}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	recs, total, err := s.userManager.QueryUsers(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryUsersResponse{
		Records:      toProtoUsers(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *svcpb.GetUserRequest) (*svcpb.GetUserResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	u, err := s.userManager.GetUser(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetUserResponse{User: toProtoUser(u)}, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *svcpb.CreateUserRequest) (*svcpb.CreateUserResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	u := fromProtoUser(req.User)
	saved, err := s.userManager.CreateUser(qc, u)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.CreateUserResponse{User: toProtoUser(saved)}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *svcpb.UpdateUserRequest) (*svcpb.UpdateUserResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	u := fromProtoUser(req.User)
	u.ID = req.Id
	saved, err := s.userManager.UpdateUser(qc, u)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.UpdateUserResponse{User: toProtoUser(saved)}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *svcpb.DeleteUserRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.userManager.DeleteUser(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- Org conversion helpers ------------------------------------------------

func toProtoOrg(o *commonTypes.Organization) *protoUser.Organization {
	if o == nil {
		return nil
	}
	p := &protoUser.Organization{
		Id:             o.ID,
		ParentId:       o.ParentID,
		OwnerUserId:    o.OwnerUserID,
		BundleId:       o.BundleID,
		OrgUnit:        o.OrgUnit,
		Salt:           o.Salt,
		MaxConcurrency: int32(o.MaxConcurrency),
		StickyMessage:  o.StickyMessage,
		LicensePolicy:  o.LicensePolicy,
		IsPersonal:     o.IsPersonal,
		Active:         o.Active,
		CreatedAt:      timestamppb.New(o.CreatedAt),
		UpdatedAt:      timestamppb.New(o.UpdatedAt),
	}
	for _, c := range o.Configs {
		p.Configs = append(p.Configs, toProtoConfigMasked(c))
	}
	return p
}

func toProtoOrgs(orgs []*commonTypes.Organization) []*protoUser.Organization {
	out := make([]*protoUser.Organization, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, toProtoOrg(o))
	}
	return out
}

func fromProtoOrg(p *protoUser.Organization) *commonTypes.Organization {
	if p == nil {
		return nil
	}
	o := &commonTypes.Organization{
		ID:             p.Id,
		ParentID:       p.ParentId,
		OwnerUserID:    p.OwnerUserId,
		BundleID:       p.BundleId,
		OrgUnit:        p.OrgUnit,
		Salt:           p.Salt,
		MaxConcurrency: int(p.MaxConcurrency),
		StickyMessage:  p.StickyMessage,
		LicensePolicy:  p.LicensePolicy,
		IsPersonal:     p.IsPersonal,
		Active:         p.Active,
	}
	for _, c := range p.Configs {
		o.Configs = append(o.Configs, fromProtoConfig(c))
	}
	return o
}

func toProtoConfig(c *commonTypes.Config) *protoUser.Config {
	if c == nil {
		return nil
	}
	return &protoUser.Config{
		Id:               c.ID,
		ConfigurableId:   c.ConfigurableID,
		ConfigurableType: string(c.ConfigurableType),
		Name:             c.Name,
		Kind:             c.Kind,
		Value:            c.Value,
		Secret:           c.Secret,
		CreatedAt:        timestamppb.New(c.CreatedAt),
		UpdatedAt:        timestamppb.New(c.UpdatedAt),
	}
}

// toProtoConfigMasked returns the proto with the value redacted for secret configs.
func toProtoConfigMasked(c *commonTypes.Config) *protoUser.Config {
	p := toProtoConfig(c)
	if p != nil && p.Secret {
		p.Value = "****"
	}
	return p
}

func toProtoConfigsMasked(cfgs []*commonTypes.Config) []*protoUser.Config {
	out := make([]*protoUser.Config, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, toProtoConfigMasked(c))
	}
	return out
}

func fromProtoConfig(p *protoUser.Config) *commonTypes.Config {
	if p == nil {
		return nil
	}
	return &commonTypes.Config{
		ID:               p.Id,
		ConfigurableID:   p.ConfigurableId,
		ConfigurableType: commonTypes.ConfigurableType(p.ConfigurableType),
		NameTypeValue: commonTypes.NameTypeValue{
			Name:   p.Name,
			Kind:   p.Kind,
			Value:  p.Value,
			Secret: p.Secret,
		},
	}
}
