// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	commonTypes "plexobject.com/formicary/internal/types"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// OrgService implements svcpb.OrganizationServiceServer.
type OrgService struct {
	svcpb.UnimplementedOrganizationServiceServer
	userManager           *manager.UserManager
	configRepository      repository.ConfigRepository
	auditRecordRepository repository.AuditRecordRepository
}

// NewOrgService creates an OrgService.
func NewOrgService(
	userManager *manager.UserManager,
	configRepository repository.ConfigRepository,
	auditRecordRepository repository.AuditRecordRepository,
) *OrgService {
	return &OrgService{
		userManager:           userManager,
		configRepository:      configRepository,
		auditRecordRepository: auditRecordRepository,
	}
}

func (s *OrgService) QueryOrgs(ctx context.Context, req *svcpb.QueryOrgsRequest) (*svcpb.QueryOrgsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.OrgUnit != "" {
		params["org_unit"] = req.OrgUnit
	}
	if req.BundleId != "" {
		params["bundle_id"] = req.BundleId
	}
	recs, total, err := s.userManager.QueryOrgs(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryOrgsResponse{
		Records:      toProtoOrgs(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *OrgService) GetOrg(ctx context.Context, req *svcpb.GetOrgRequest) (*svcpb.GetOrgResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	org, err := s.userManager.GetOrganization(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetOrgResponse{Organization: toProtoOrg(org)}, nil
}

func (s *OrgService) CreateOrg(ctx context.Context, req *svcpb.CreateOrgRequest) (*svcpb.CreateOrgResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	org := fromProtoOrg(req.Organization)
	saved, err := s.userManager.CreateOrg(qc, org)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.CreateOrgResponse{Organization: toProtoOrg(saved)}, nil
}

func (s *OrgService) UpdateOrg(ctx context.Context, req *svcpb.UpdateOrgRequest) (*svcpb.UpdateOrgResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	org := fromProtoOrg(req.Organization)
	org.ID = req.Id
	saved, err := s.userManager.UpdateOrg(qc, org)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.UpdateOrgResponse{Organization: toProtoOrg(saved)}, nil
}

func (s *OrgService) DeleteOrg(ctx context.Context, req *svcpb.DeleteOrgRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.userManager.DeleteOrganization(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- Org config RPCs -------------------------------------------------------

func (s *OrgService) QueryOrgConfigs(ctx context.Context, req *svcpb.QueryOrgConfigsRequest) (*svcpb.QueryOrgConfigsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	// Non-admins may only query their own org's configs.
	orgID := req.OrganizationId
	if !qc.IsAdmin() {
		orgID = qc.GetOrganizationID()
	}
	recs, total, err := s.configRepository.QueryOrgConfigs(qc, orgID, int(req.Page), pageSize(req.PageSize))
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryOrgConfigsResponse{
		Records:      toProtoConfigsMasked(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
	}, nil
}

func (s *OrgService) GetOrgConfig(ctx context.Context, req *svcpb.GetOrgConfigRequest) (*svcpb.GetOrgConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	// scopedOrgDB/strictScopedDB inside Get enforces tenant isolation.
	cfg, err := s.configRepository.Get(qc, req.ConfigId)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetOrgConfigResponse{Config: toProtoConfigMasked(cfg)}, nil
}

func (s *OrgService) RevealOrgConfig(ctx context.Context, req *svcpb.RevealOrgConfigRequest) (*svcpb.RevealOrgConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg, err := s.configRepository.Get(qc, req.ConfigId)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	_, _ = s.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return &svcpb.RevealOrgConfigResponse{Config: toProtoConfig(cfg)}, nil
}

func (s *OrgService) SaveOrgConfig(ctx context.Context, req *svcpb.SaveOrgConfigRequest) (*svcpb.SaveOrgConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg := fromProtoConfig(req.Config)
	if cfg == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	// Non-admins may only write to their own org — ignore req.OrganizationId.
	orgID := req.OrganizationId
	if !qc.IsAdmin() {
		orgID = qc.GetOrganizationID()
	}
	cfg.ConfigurableID = orgID
	cfg.ConfigurableType = commonTypes.ConfigurableTypeOrg
	if req.ConfigId != "" {
		cfg.ID = req.ConfigId
	}
	if cfg.Secret && cfg.Value == "****" && cfg.ID != "" {
		existing, getErr := s.configRepository.Get(qc, cfg.ID)
		if getErr != nil {
			return nil, interceptors.MapDomainError(getErr)
		}
		cfg.Value = existing.Value
	}
	saved, err := s.configRepository.Save(qc, cfg)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveOrgConfigResponse{Config: toProtoConfigMasked(saved)}, nil
}

func (s *OrgService) DeleteOrgConfig(ctx context.Context, req *svcpb.DeleteOrgConfigRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.configRepository.Delete(qc, req.ConfigId); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- User config RPCs -------------------------------------------------------

func (s *OrgService) QueryUserConfigs(ctx context.Context, req *svcpb.QueryUserConfigsRequest) (*svcpb.QueryUserConfigsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	recs, total, err := s.configRepository.QueryUserConfigs(qc, qc.GetUserID(), int(req.Page), pageSize(req.PageSize))
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryUserConfigsResponse{
		Records:      toProtoConfigsMasked(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
	}, nil
}

func (s *OrgService) GetUserConfig(ctx context.Context, req *svcpb.GetUserConfigRequest) (*svcpb.GetUserConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg, err := s.configRepository.Get(qc, req.ConfigId)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetUserConfigResponse{Config: toProtoConfigMasked(cfg)}, nil
}

func (s *OrgService) RevealUserConfig(ctx context.Context, req *svcpb.RevealUserConfigRequest) (*svcpb.RevealUserConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg, err := s.configRepository.Get(qc, req.ConfigId)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	_, _ = s.auditRecordRepository.Save(types.NewAuditRecordFromConfig(cfg, qc))
	return &svcpb.RevealUserConfigResponse{Config: toProtoConfig(cfg)}, nil
}

func (s *OrgService) SaveUserConfig(ctx context.Context, req *svcpb.SaveUserConfigRequest) (*svcpb.SaveUserConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg := fromProtoConfig(req.Config)
	if cfg == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	cfg.ConfigurableID = qc.GetUserID()
	cfg.ConfigurableType = commonTypes.ConfigurableTypeUser
	if req.ConfigId != "" {
		cfg.ID = req.ConfigId
	}
	if cfg.Secret && cfg.Value == "****" && cfg.ID != "" {
		existing, getErr := s.configRepository.Get(qc, cfg.ID)
		if getErr != nil {
			return nil, interceptors.MapDomainError(getErr)
		}
		cfg.Value = existing.Value
	}
	saved, err := s.configRepository.Save(qc, cfg)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveUserConfigResponse{Config: toProtoConfigMasked(saved)}, nil
}

func (s *OrgService) DeleteUserConfig(ctx context.Context, req *svcpb.DeleteUserConfigRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.configRepository.Delete(qc, req.ConfigId); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}
