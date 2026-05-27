// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	"plexobject.com/formicary/queen/manager"
)

// HealthService implements svcpb.HealthServiceServer.
type HealthService struct {
	svcpb.UnimplementedHealthServiceServer
	dashboardManager *manager.DashboardManager
}

// NewHealthService creates a HealthService.
func NewHealthService(dashboardManager *manager.DashboardManager) *HealthService {
	return &HealthService{dashboardManager: dashboardManager}
}

func (s *HealthService) GetHealth(_ context.Context, _ *emptypb.Empty) (*svcpb.HealthResponse, error) {
	resp := s.dashboardManager.GetHealthStatuses()

	overallStatus := "healthy"
	if resp.OverallStatus != nil && !resp.OverallStatus.Healthy() {
		overallStatus = "degraded"
	}

	components := make([]*svcpb.ComponentHealth, 0, len(resp.ServiceStatuses))
	for _, ss := range resp.ServiceStatuses {
		compStatus := "healthy"
		if !ss.Healthy() {
			compStatus = "degraded"
		}
		name := ""
		if ss.Monitored != nil {
			name = ss.Monitored.Name()
		}
		components = append(components, &svcpb.ComponentHealth{
			Name:   name,
			Status: compStatus,
		})
	}

	registeredAnts := make(map[string]int32)
	for method, count := range s.dashboardManager.CountContainerEvents() {
		registeredAnts[string(method)] = int32(count)
	}

	return &svcpb.HealthResponse{
		OverallStatus:  overallStatus,
		Components:     components,
		RegisteredAnts: registeredAnts,
	}, nil
}

func (s *HealthService) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

