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
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
	"plexobject.com/formicary/queen/repository"
)

// ErrorCodeService implements svcpb.ErrorCodeServiceServer.
type ErrorCodeService struct {
	svcpb.UnimplementedErrorCodeServiceServer
	errorCodeRepository repository.ErrorCodeRepository
}

// NewErrorCodeService creates an ErrorCodeService.
func NewErrorCodeService(errorCodeRepository repository.ErrorCodeRepository) *ErrorCodeService {
	return &ErrorCodeService{errorCodeRepository: errorCodeRepository}
}

func (s *ErrorCodeService) QueryErrorCodes(ctx context.Context, req *svcpb.QueryErrorCodesRequest) (*svcpb.QueryErrorCodesResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	if req.ErrorCode != "" {
		params["error_code"] = req.ErrorCode
	}
	if req.PlatformScope != "" {
		params["platform_scope"] = req.PlatformScope
	}
	if req.TaskTypeScope != "" {
		params["task_type_scope"] = req.TaskTypeScope
	}
	recs, total, err := s.errorCodeRepository.Query(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryErrorCodesResponse{
		Records:      toProtoErrorCodes(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *ErrorCodeService) GetErrorCode(ctx context.Context, req *svcpb.GetErrorCodeRequest) (*svcpb.GetErrorCodeResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	ec, err := s.errorCodeRepository.Get(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetErrorCodeResponse{ErrorCode: toProtoErrorCode(ec)}, nil
}

func (s *ErrorCodeService) SaveErrorCode(ctx context.Context, req *svcpb.SaveErrorCodeRequest) (*svcpb.SaveErrorCodeResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	ec := fromProtoErrorCode(req.ErrorCode)
	if ec == nil {
		return nil, status.Error(codes.InvalidArgument, "error_code is required")
	}
	if req.Id != "" {
		ec.ID = req.Id
	}
	saved, err := s.errorCodeRepository.Save(qc, ec)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveErrorCodeResponse{ErrorCode: toProtoErrorCode(saved)}, nil
}

func (s *ErrorCodeService) DeleteErrorCode(ctx context.Context, req *svcpb.DeleteErrorCodeRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.errorCodeRepository.Delete(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// ---- ErrorCode conversion helpers ------------------------------------------

func toProtoErrorCode(ec *commonTypes.ErrorCode) *protoQueen.ErrorCode {
	if ec == nil {
		return nil
	}
	return &protoQueen.ErrorCode{
		Id:             ec.ID,
		Regex:          ec.Regex,
		ExitCode:       int32(ec.ExitCode),
		ErrorCode:      ec.ErrorCode,
		Description:    ec.Description,
		DisplayMessage: ec.DisplayMessage,
		DisplayCode:    ec.DisplayCode,
		JobType:        ec.JobType,
		TaskTypeScope:  ec.TaskTypeScope,
		PlatformScope:  ec.PlatformScope,
		CommandScope:   ec.CommandScope,
		UserId:         ec.UserID,
		OrganizationId: ec.OrganizationID,
		Action:         string(ec.Action),
		HardFailure:    ec.HardFailure,
		Retry:          int32(ec.Retry),
		CreatedAt:      timestamppb.New(ec.CreatedAt),
		UpdatedAt:      timestamppb.New(ec.UpdatedAt),
	}
}

func toProtoErrorCodes(ecs []*commonTypes.ErrorCode) []*protoQueen.ErrorCode {
	out := make([]*protoQueen.ErrorCode, 0, len(ecs))
	for _, ec := range ecs {
		out = append(out, toProtoErrorCode(ec))
	}
	return out
}

func fromProtoErrorCode(p *protoQueen.ErrorCode) *commonTypes.ErrorCode {
	if p == nil {
		return nil
	}
	return &commonTypes.ErrorCode{
		ID:             p.Id,
		Regex:          p.Regex,
		ExitCode:       int(p.ExitCode),
		ErrorCode:      p.ErrorCode,
		Description:    p.Description,
		DisplayMessage: p.DisplayMessage,
		DisplayCode:    p.DisplayCode,
		JobType:        p.JobType,
		TaskTypeScope:  p.TaskTypeScope,
		PlatformScope:  p.PlatformScope,
		CommandScope:   p.CommandScope,
		UserID:         p.UserId,
		OrganizationID: p.OrganizationId,
		Action:         commonTypes.ErrorCodeAction(p.Action),
		HardFailure:    p.HardFailure,
		Retry:          int(p.Retry),
	}
}
