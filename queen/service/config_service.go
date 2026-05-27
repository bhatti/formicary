// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	queenTypes "plexobject.com/formicary/queen/types"
)

// ConfigService implements svcpb.ConfigServiceServer.
type ConfigService struct {
	svcpb.UnimplementedConfigServiceServer
	sysConfigRepository     repository.SystemConfigRepository
	jobDefinitionRepository repository.JobDefinitionRepository
	jobManager              *manager.JobManager
}

// NewConfigService creates a ConfigService.
func NewConfigService(
	sysConfigRepository repository.SystemConfigRepository,
	jobDefinitionRepository repository.JobDefinitionRepository,
	jobManager *manager.JobManager,
) *ConfigService {
	return &ConfigService{
		sysConfigRepository:     sysConfigRepository,
		jobDefinitionRepository: jobDefinitionRepository,
		jobManager:              jobManager,
	}
}

func (s *ConfigService) QuerySystemConfigs(ctx context.Context, req *svcpb.QuerySystemConfigsRequest) (*svcpb.QuerySystemConfigsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.Scope != "" {
		params["scope"] = req.Scope
	}
	if req.Kind != "" {
		params["kind"] = req.Kind
	}
	if req.Name != "" {
		params["name"] = req.Name
	}
	// SystemConfigRepository.Query does not take a QueryContext.
	recs, total, err := s.sysConfigRepository.Query(params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QuerySystemConfigsResponse{
		Records:      toProtoSystemConfigs(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *ConfigService) GetSystemConfig(ctx context.Context, req *svcpb.GetSystemConfigRequest) (*svcpb.GetSystemConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg, err := s.sysConfigRepository.Get(req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetSystemConfigResponse{Config: toProtoSystemConfig(cfg)}, nil
}

func (s *ConfigService) SaveSystemConfig(ctx context.Context, req *svcpb.SaveSystemConfigRequest) (*svcpb.SaveSystemConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg := fromProtoSystemConfig(req.Config)
	if cfg == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	if req.Id != "" {
		cfg.ID = req.Id
	}
	saved, err := s.sysConfigRepository.Save(cfg)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveSystemConfigResponse{Config: toProtoSystemConfig(saved)}, nil
}

func (s *ConfigService) DeleteSystemConfig(ctx context.Context, req *svcpb.DeleteSystemConfigRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.sysConfigRepository.Delete(req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *ConfigService) QueryJobConfigs(ctx context.Context, req *svcpb.QueryJobConfigsRequest) (*svcpb.QueryJobConfigsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	jd, err := s.jobManager.GetJobDefinition(qc, req.JobId)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	total := int64(len(jd.Configs))
	ps := pageSize(req.PageSize)
	p := int(req.Page)
	start := p * ps
	end := start + ps
	if start > len(jd.Configs) {
		start = len(jd.Configs)
	}
	if end > len(jd.Configs) {
		end = len(jd.Configs)
	}
	page := jd.Configs[start:end]
	return &svcpb.QueryJobConfigsResponse{
		Records:      toProtoJobDefinitionConfigs(page),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
	}, nil
}

func (s *ConfigService) SaveJobConfig(ctx context.Context, req *svcpb.SaveJobConfigRequest) (*svcpb.SaveJobConfigResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg := fromProtoJobDefinitionConfig(req.Config)
	if cfg == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	saved, err := s.jobDefinitionRepository.SaveConfig(qc, req.JobId, cfg.Name, cfg.Value, cfg.Secret)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveJobConfigResponse{Config: toProtoJobDefinitionConfig(saved)}, nil
}

func (s *ConfigService) DeleteJobConfig(ctx context.Context, req *svcpb.DeleteJobConfigRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobDefinitionRepository.DeleteConfig(qc, req.JobId, req.ConfigId); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- System config conversion helpers -------------------------------------

func toProtoSystemConfig(c *queenTypes.SystemConfig) *protoQueen.SystemConfig {
	if c == nil {
		return nil
	}
	return &protoQueen.SystemConfig{
		Id:        c.ID,
		Scope:     c.Scope,
		Kind:      c.Kind,
		Name:      c.Name,
		Value:     c.Value,
		Secret:    c.Secret,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

func toProtoSystemConfigs(cfgs []*queenTypes.SystemConfig) []*protoQueen.SystemConfig {
	out := make([]*protoQueen.SystemConfig, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, toProtoSystemConfig(c))
	}
	return out
}

func fromProtoSystemConfig(p *protoQueen.SystemConfig) *queenTypes.SystemConfig {
	if p == nil {
		return nil
	}
	return &queenTypes.SystemConfig{
		ID:    p.Id,
		Scope: p.Scope,
		Kind:  p.Kind,
		Name:  p.Name,
		Value: p.Value,
		Secret: p.Secret,
	}
}

// toProtoJobDefinitionConfigs converts a slice of JobDefinitionConfig to proto.
func toProtoJobDefinitionConfigs(cfgs []*queenTypes.JobDefinitionConfig) []*protoQueen.JobDefinitionConfig {
	out := make([]*protoQueen.JobDefinitionConfig, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, toProtoJobDefinitionConfig(c))
	}
	return out
}
