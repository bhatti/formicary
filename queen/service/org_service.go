// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

// OrgService implements svcpb.OrganizationServiceServer.
type OrgService struct {
	svcpb.UnimplementedOrganizationServiceServer
	userManager         *manager.UserManager
	orgConfigRepository repository.OrganizationConfigRepository
}

// NewOrgService creates an OrgService.
func NewOrgService(
	userManager *manager.UserManager,
	orgConfigRepository repository.OrganizationConfigRepository,
) *OrgService {
	return &OrgService{
		userManager:         userManager,
		orgConfigRepository: orgConfigRepository,
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

func (s *OrgService) QueryOrgConfigs(ctx context.Context, req *svcpb.QueryOrgConfigsRequest) (*svcpb.QueryOrgConfigsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	recs, total, err := s.orgConfigRepository.Query(qc, params, int(req.Page), pageSize(req.PageSize), nil)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryOrgConfigsResponse{
		Records:      toProtoOrgConfigs(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
	}, nil
}

func (s *OrgService) SaveOrgConfig(ctx context.Context, req *svcpb.SaveOrgConfigRequest) (*svcpb.SaveOrgConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg := fromProtoOrgConfig(req.Config)
	if cfg == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	cfg.OrganizationID = req.OrganizationId
	if req.ConfigId != "" {
		cfg.ID = req.ConfigId
	}
	saved, err := s.orgConfigRepository.Save(qc, cfg)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveOrgConfigResponse{Config: toProtoOrgConfig(saved)}, nil
}

func (s *OrgService) DeleteOrgConfig(ctx context.Context, req *svcpb.DeleteOrgConfigRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.orgConfigRepository.Delete(qc, req.ConfigId); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}
