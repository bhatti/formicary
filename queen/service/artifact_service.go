// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	commonTypes "plexobject.com/formicary/internal/types"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoResource "plexobject.com/formicary/gen/go/formicary/v1/resource"
	"plexobject.com/formicary/queen/manager"
)

// ArtifactService implements svcpb.ArtifactServiceServer.
type ArtifactService struct {
	svcpb.UnimplementedArtifactServiceServer
	artifactManager *manager.ArtifactManager
}

// NewArtifactService creates an ArtifactService.
func NewArtifactService(artifactManager *manager.ArtifactManager) *ArtifactService {
	return &ArtifactService{artifactManager: artifactManager}
}

func (s *ArtifactService) QueryArtifacts(ctx context.Context, req *svcpb.QueryArtifactsRequest) (*svcpb.QueryArtifactsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobRequestId != "" {
		params["job_request_id"] = req.JobRequestId
	}
	if req.JobExecutionId != "" {
		params["job_execution_id"] = req.JobExecutionId
	}
	if req.TaskExecutionId != "" {
		params["task_execution_id"] = req.TaskExecutionId
	}
	if req.Kind != "" {
		params["kind"] = req.Kind
	}
	recs, total, err := s.artifactManager.QueryArtifacts(ctx, qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryArtifactsResponse{
		Records:      toProtoArtifacts(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *ArtifactService) GetArtifact(ctx context.Context, req *svcpb.GetArtifactRequest) (*svcpb.GetArtifactResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	art, err := s.artifactManager.GetArtifact(ctx, qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetArtifactResponse{Artifact: toProtoArtifact(art)}, nil
}

func (s *ArtifactService) DeleteArtifact(ctx context.Context, req *svcpb.DeleteArtifactRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.artifactManager.DeleteArtifact(ctx, qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- Artifact conversion helpers -------------------------------------------

func toProtoArtifact(a *commonTypes.Artifact) *protoResource.Artifact {
	if a == nil {
		return nil
	}
	p := &protoResource.Artifact{
		Id:                 a.ID,
		Bucket:             a.Bucket,
		Name:               a.Name,
		OrganizationId:     a.OrganizationID,
		UserId:             a.UserID,
		ArtifactGroup:      a.ArtifactGroup,
		Kind:               a.Kind,
		Etag:               a.ETag,
		ArtifactOrder:      int32(a.ArtifactOrder),
		JobRequestId:       a.JobRequestID,
		JobExecutionId:     a.JobExecutionID,
		TaskExecutionId:    a.TaskExecutionID,
		TaskType:           a.TaskType,
		Sha256:             a.SHA256,
		ContentType:        a.ContentType,
		ContentLength:      a.ContentLength,
		Permissions:        a.Permissions,
		MetadataSerialized: a.MetadataSerialized,
		TagsSerialized:     a.TagsSerialized,
		Active:             a.Active,
		Metadata:           a.Metadata,
		Tags:               a.Tags,
		Url:                a.URL,
		ExpiresAt:          timestamppb.New(a.ExpiresAt),
		CreatedAt:          timestamppb.New(a.CreatedAt),
		UpdatedAt:          timestamppb.New(a.UpdatedAt),
	}
	return p
}

func toProtoArtifacts(arts []*commonTypes.Artifact) []*protoResource.Artifact {
	out := make([]*protoResource.Artifact, 0, len(arts))
	for _, a := range arts {
		out = append(out, toProtoArtifact(a))
	}
	return out
}
