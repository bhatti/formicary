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
)

// JobDefinitionService implements svcpb.JobDefinitionServiceServer.
type JobDefinitionService struct {
	svcpb.UnimplementedJobDefinitionServiceServer
	jobManager *manager.JobManager
}

// NewJobDefinitionService creates a JobDefinitionService backed by jobManager.
func NewJobDefinitionService(jobManager *manager.JobManager) *JobDefinitionService {
	return &JobDefinitionService{jobManager: jobManager}
}

func (s *JobDefinitionService) QueryJobDefinitions(ctx context.Context, req *svcpb.QueryJobDefinitionsRequest) (*svcpb.QueryJobDefinitionsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	if req.Platform != "" {
		params["platform"] = req.Platform
	}
	if req.Tags != "" {
		params["tags"] = req.Tags
	}
	params["public_plugin"] = req.PublicPlugin
	recs, total, err := s.jobManager.QueryJobDefinitions(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryJobDefinitionsResponse{
		Records:      toProtoJobDefinitions(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *JobDefinitionService) QueryPlugins(ctx context.Context, req *svcpb.QueryJobDefinitionsRequest) (*svcpb.QueryJobDefinitionsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{"public_plugin": true}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	if req.Platform != "" {
		params["platform"] = req.Platform
	}
	if req.Tags != "" {
		params["tags"] = req.Tags
	}
	recs, total, err := s.jobManager.QueryJobDefinitions(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryJobDefinitionsResponse{
		Records:      toProtoJobDefinitions(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *JobDefinitionService) GetJobDefinition(ctx context.Context, req *svcpb.GetJobDefinitionRequest) (*svcpb.GetJobDefinitionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	jd, err := s.jobManager.GetJobDefinition(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetJobDefinitionResponse{JobDefinition: toProtoJobDefinition(jd)}, nil
}

func (s *JobDefinitionService) GetJobDefinitionYAML(ctx context.Context, req *svcpb.GetJobDefinitionByTypeRequest) (*svcpb.GetJobDefinitionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	yamlBytes, err := s.jobManager.GetYamlJobDefinitionByType(qc, req.JobType)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	// Return YAML as raw_yaml field on the definition wrapper
	jd, err := s.jobManager.GetJobDefinitionByType(qc, req.JobType, "")
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	p := toProtoJobDefinition(jd)
	p.RawYaml = string(yamlBytes)
	return &svcpb.GetJobDefinitionResponse{JobDefinition: p}, nil
}

func (s *JobDefinitionService) GetJobDefinitionMermaid(ctx context.Context, req *svcpb.GetJobDefinitionMermaidRequest) (*svcpb.GetJobDefinitionMermaidResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	mermaid, err := s.jobManager.GetMermaidConfigForJobDefinition(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetJobDefinitionMermaidResponse{Mermaid: mermaid}, nil
}

func (s *JobDefinitionService) GetJobDefinitionStats(ctx context.Context, req *svcpb.QueryJobDefinitionsRequest) (*svcpb.GetJobDefinitionStatsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	recs, _, err := s.jobManager.QueryJobDefinitions(qc, params, 0, 1000, nil)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	stats := make([]*svcpb.JobDefinitionStat, 0, len(recs))
	for _, jd := range recs {
		stats = append(stats, &svcpb.JobDefinitionStat{
			JobType: jd.JobType,
		})
	}
	return &svcpb.GetJobDefinitionStatsResponse{Stats: stats}, nil
}

func (s *JobDefinitionService) CreateJobDefinition(ctx context.Context, req *svcpb.CreateJobDefinitionRequest) (*svcpb.CreateJobDefinitionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if req.JobDefinition == nil {
		return nil, status.Error(codes.InvalidArgument, "job_definition is required")
	}
	jd := fromProtoJobDefinition(req.JobDefinition)
	saved, err := s.jobManager.SaveJobDefinition(qc, jd)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.CreateJobDefinitionResponse{JobDefinition: toProtoJobDefinition(saved)}, nil
}

func (s *JobDefinitionService) UpdateJobDefinition(ctx context.Context, req *svcpb.UpdateJobDefinitionRequest) (*svcpb.UpdateJobDefinitionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if req.JobDefinition == nil {
		return nil, status.Error(codes.InvalidArgument, "job_definition is required")
	}
	jd := fromProtoJobDefinition(req.JobDefinition)
	jd.ID = req.Id
	saved, err := s.jobManager.SaveJobDefinition(qc, jd)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.UpdateJobDefinitionResponse{JobDefinition: toProtoJobDefinition(saved)}, nil
}

func (s *JobDefinitionService) DeleteJobDefinition(ctx context.Context, req *svcpb.DeleteJobDefinitionRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.DeleteJobDefinition(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobDefinitionService) DisableJobDefinition(ctx context.Context, req *svcpb.DisableJobDefinitionRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.DisableJobDefinition(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobDefinitionService) EnableJobDefinition(ctx context.Context, req *svcpb.EnableJobDefinitionRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.EnableJobDefinition(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobDefinitionService) UpdateConcurrency(ctx context.Context, req *svcpb.UpdateConcurrencyRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.SetJobDefinitionMaxConcurrency(qc, req.Id, int(req.Concurrency)); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// pageSize returns defaultPageSize if the requested size is ≤ 0.
func pageSize(requested int32) int {
	if requested <= 0 {
		return 200
	}
	if requested > 1000 {
		return 1000
	}
	return int(requested)
}

// effectivePageSize returns the clamped page size as int32 for response fields.
func effectivePageSize(requested int32) int32 {
	return int32(pageSize(requested))
}
