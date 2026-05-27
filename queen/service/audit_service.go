// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
	"plexobject.com/formicary/queen/repository"
	queenTypes "plexobject.com/formicary/queen/types"
)

// AuditService implements svcpb.AuditServiceServer.
type AuditService struct {
	svcpb.UnimplementedAuditServiceServer
	auditRepository repository.AuditRecordRepository
}

// NewAuditService creates an AuditService.
func NewAuditService(auditRepository repository.AuditRecordRepository) *AuditService {
	return &AuditService{auditRepository: auditRepository}
}

func (s *AuditService) QueryAuditRecords(ctx context.Context, req *svcpb.QueryAuditRecordsRequest) (*svcpb.QueryAuditRecordsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.UserId != "" {
		params["user_id"] = req.UserId
	}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	if req.Kind != "" {
		params["kind"] = req.Kind
	}
	// AuditRecordRepository.Query does not take a QueryContext — it's a global-admin operation.
	recs, total, err := s.auditRepository.Query(params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryAuditRecordsResponse{
		Records:      toProtoAuditRecords(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

// ---- Audit conversion helpers ----------------------------------------------

func toProtoAuditRecord(r *queenTypes.AuditRecord) *protoQueen.AuditRecord {
	if r == nil {
		return nil
	}
	return &protoQueen.AuditRecord{
		Id:             r.ID,
		TargetId:       r.TargetID,
		UserId:         r.UserID,
		OrganizationId: r.OrganizationID,
		Kind:           string(r.Kind),
		JobType:        r.JobType,
		RemoteIp:       r.RemoteIP,
		Url:            r.URL,
		Error:          r.Error,
		Message:        r.Message,
		CreatedAt:      timestamppb.New(r.CreatedAt),
	}
}

func toProtoAuditRecords(recs []*queenTypes.AuditRecord) []*protoQueen.AuditRecord {
	out := make([]*protoQueen.AuditRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, toProtoAuditRecord(r))
	}
	return out
}
