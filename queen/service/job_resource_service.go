// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	commonTypes "plexobject.com/formicary/internal/types"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
	"plexobject.com/formicary/queen/repository"
	queenTypes "plexobject.com/formicary/queen/types"
)

// JobResourceService implements svcpb.JobResourceServiceServer.
type JobResourceService struct {
	svcpb.UnimplementedJobResourceServiceServer
	jobResourceRepository repository.JobResourceRepository
}

// NewJobResourceService creates a JobResourceService.
func NewJobResourceService(jobResourceRepository repository.JobResourceRepository) *JobResourceService {
	return &JobResourceService{jobResourceRepository: jobResourceRepository}
}

func (s *JobResourceService) QueryJobResources(ctx context.Context, req *svcpb.QueryJobResourcesRequest) (*svcpb.QueryJobResourcesResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.ResourceType != "" {
		params["resource_type"] = req.ResourceType
	}
	if req.Platform != "" {
		params["platform"] = req.Platform
	}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	recs, total, err := s.jobResourceRepository.Query(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryJobResourcesResponse{
		Records:      toProtoJobResources(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *JobResourceService) GetJobResource(ctx context.Context, req *svcpb.GetJobResourceRequest) (*svcpb.GetJobResourceResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	r, err := s.jobResourceRepository.Get(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetJobResourceResponse{JobResource: toProtoJobResource(r)}, nil
}

func (s *JobResourceService) SaveJobResource(ctx context.Context, req *svcpb.SaveJobResourceRequest) (*svcpb.SaveJobResourceResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	r := fromProtoJobResource(req.JobResource)
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "job_resource is required")
	}
	if req.Id != "" {
		r.ID = req.Id
	}
	saved, err := s.jobResourceRepository.Save(qc, r)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveJobResourceResponse{JobResource: toProtoJobResource(saved)}, nil
}

func (s *JobResourceService) DeleteJobResource(ctx context.Context, req *svcpb.DeleteJobResourceRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobResourceRepository.Delete(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- JobResource conversion helpers ----------------------------------------

func toProtoJobResource(r *queenTypes.JobResource) *protoQueen.JobResource {
	if r == nil {
		return nil
	}
	p := &protoQueen.JobResource{
		Id:             r.ID,
		ExternalId:     r.ExternalID,
		ValidStatus:    r.ValidStatus,
		Quota:          int32(r.Quota),
		LeaseTimeoutNs: ns(r.LeaseTimeout),
		Disabled:       r.Disabled,
		OrganizationId: r.OrganizationID,
		UserId:         r.UserID,
		Active:         r.Active,
		CreatedAt:      timestamppb.New(r.CreatedAt),
		UpdatedAt:      timestamppb.New(r.UpdatedAt),
		Basic: &protoQueen.BasicResource{
			ResourceType:   r.ResourceType,
			Description:    r.Description,
			Platform:       r.Platform,
			Category:       r.Category,
			Tags:           r.Tags,
			TagsSerialized: r.TagsSerialized,
			Value:          int32(r.Value),
		},
	}
	for _, c := range r.Configs {
		p.Configs = append(p.Configs, toProtoJobResourceConfig(c))
	}
	if r.NameValueConfigs != nil {
		if b, err := json.Marshal(r.NameValueConfigs); err == nil {
			p.NameValueConfigsJson = string(b)
		}
	}
	return p
}

func toProtoJobResources(recs []*queenTypes.JobResource) []*protoQueen.JobResource {
	out := make([]*protoQueen.JobResource, 0, len(recs))
	for _, r := range recs {
		out = append(out, toProtoJobResource(r))
	}
	return out
}

func fromProtoJobResource(p *protoQueen.JobResource) *queenTypes.JobResource {
	if p == nil {
		return nil
	}
	r := &queenTypes.JobResource{
		ID:             p.Id,
		ExternalID:     p.ExternalId,
		ValidStatus:    p.ValidStatus,
		Quota:          int(p.Quota),
		LeaseTimeout:   fromNs(p.LeaseTimeoutNs),
		Disabled:       p.Disabled,
		OrganizationID: p.OrganizationId,
		UserID:         p.UserId,
		Active:         p.Active,
	}
	if p.Basic != nil {
		r.BasicResource = queenTypes.BasicResource{
			ResourceType:   p.Basic.ResourceType,
			Description:    p.Basic.Description,
			Platform:       p.Basic.Platform,
			Category:       p.Basic.Category,
			Tags:           p.Basic.Tags,
			TagsSerialized: p.Basic.TagsSerialized,
			Value:          int(p.Basic.Value),
		}
	}
	for _, c := range p.Configs {
		r.Configs = append(r.Configs, fromProtoJobResourceConfig(c))
	}
	if p.NameValueConfigsJson != "" {
		_ = json.Unmarshal([]byte(p.NameValueConfigsJson), &r.NameValueConfigs)
	}
	return r
}

func toProtoJobResourceConfig(c *queenTypes.JobResourceConfig) *protoQueen.JobResourceConfig {
	if c == nil {
		return nil
	}
	return &protoQueen.JobResourceConfig{
		Id:            c.ID,
		JobResourceId: c.JobResourceID,
		Name:          c.Name,
		Kind:          c.Kind,
		Value:         c.Value,
		Secret:        c.Secret,
		CreatedAt:     timestamppb.New(c.CreatedAt),
		UpdatedAt:     timestamppb.New(c.UpdatedAt),
	}
}

func fromProtoJobResourceConfig(p *protoQueen.JobResourceConfig) *queenTypes.JobResourceConfig {
	if p == nil {
		return nil
	}
	return &queenTypes.JobResourceConfig{
		ID:            p.Id,
		JobResourceID: p.JobResourceId,
		NameTypeValue: commonTypes.NameTypeValue{
			Name:   p.Name,
			Kind:   p.Kind,
			Value:  p.Value,
			Secret: p.Secret,
		},
	}
}
