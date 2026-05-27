// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	"plexobject.com/formicary/queen/manager"
)

// AdminService implements svcpb.AdminServiceServer.
type AdminService struct {
	svcpb.UnimplementedAdminServiceServer
	dashboardManager *manager.DashboardManager
	userManager      *manager.UserManager
}

// NewAdminService creates an AdminService.
func NewAdminService(dashboardManager *manager.DashboardManager, userManager *manager.UserManager) *AdminService {
	return &AdminService{
		dashboardManager: dashboardManager,
		userManager:      userManager,
	}
}

func (s *AdminService) GetDashboardStats(ctx context.Context, _ *svcpb.DashboardStatsRequest) (*svcpb.DashboardStatsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}

	totalOrgs, err := s.dashboardManager.OrgCounts()
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	totalUsers, err := s.dashboardManager.UserCounts(qc)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	totalJobDefs, err := s.dashboardManager.JobDefinitionCounts(qc)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	totalPlugins, err := s.dashboardManager.PluginCounts()
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}

	antRegs := s.dashboardManager.AntRegistrations()
	totalAnts := int64(len(antRegs))

	return &svcpb.DashboardStatsResponse{
		TotalOrgs:             totalOrgs,
		TotalUsers:            totalUsers,
		TotalAntRegistrations: totalAnts,
		TotalJobDefinitions:   totalJobDefs,
		TotalPlugins:          totalPlugins,
	}, nil
}
