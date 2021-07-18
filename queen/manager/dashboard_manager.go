package manager

import (
	"fmt"
	"github.com/karlseguin/ccache/v2"
	"plexobject.com/formicary/internal/health"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
	"sort"
	"time"
)

// DashboardManager for managing artifacts
type DashboardManager struct {
	serverCfg         *config.ServerConfig
	repositoryFactory *repository.Factory
	jobStatsRegistry  *stats.JobStatsRegistry
	resourceManager   resource.Manager
	heathMonitor      *health.Monitor
	cache             *ccache.Cache
}

// HealthStatusResponse response
type HealthStatusResponse struct {
	OverallStatus   *health.ServiceStatus
	ServiceStatuses []*health.ServiceStatus
}

// NewDashboardManager manages stats
func NewDashboardManager(
	serverCfg *config.ServerConfig,
	repositoryFactory *repository.Factory,
	jobStatsRegistry *stats.JobStatsRegistry,
	resourceManager resource.Manager,
	heathMonitor *health.Monitor,
) *DashboardManager {
	var cache = ccache.New(ccache.Configure().MaxSize(2000).ItemsToPrune(200))
	return &DashboardManager{
		serverCfg:         serverCfg,
		repositoryFactory: repositoryFactory,
		jobStatsRegistry:  jobStatsRegistry,
		resourceManager:   resourceManager,
		heathMonitor:      heathMonitor,
		cache:             cache,
	}
}

// GetCPUResources for last n days/weeks/months and beginning of time to current
func (s *DashboardManager) GetCPUResources(
	qc *common.QueryContext,
	days int,
	weeks int,
	months int) ([]types.ResourceUsage, error) {
	ranges := BuildRanges(time.Now(), days, weeks, months)
	return s.repositoryFactory.JobExecutionRepository.GetResourceUsage(qc, ranges)
}

// GetStorageResources for last n days/weeks/months and beginning of time to current
func (s *DashboardManager) GetStorageResources(
	qc *common.QueryContext,
	days int,
	weeks int,
	months int) ([]types.ResourceUsage, error) {
	ranges := BuildRanges(time.Now(), days, weeks, months)
	return s.repositoryFactory.ArtifactRepository.GetResourceUsage(qc, ranges)
}

func addMonthYear(now time.Time, months int) time.Time {
	return time.Date(now.Year(), now.Month()+time.Month(months), 1, 0, 0, 0, 0, now.Location())
}

// BuildRanges builds ranges
func BuildRanges(now time.Time, days int, weeks int, months int) (ranges []types.DateRange) {
	ranges = make([]types.DateRange, days+weeks+months+1)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for i := 0; i < days; i++ {
		start := startOfDay.Add(time.Duration(i*24*-1) * time.Hour)
		ranges[days-i-1] = types.DateRange{
			StartDate: start,
			EndDate:   start.Add(24 * time.Hour).Add(-1 * time.Nanosecond),
		}
	}
	for i := 0; i < weeks; i++ {
		start := startOfDay.Add(time.Duration((i+1)*24*7*-1) * time.Hour)
		ranges[weeks+days-i-1] = types.DateRange{
			StartDate: start,
			EndDate:   start.Add(24 * time.Hour*7).Add(-1 * time.Nanosecond),
		}
	}
	daysInMonths := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := 0; i < months; i++ {
		if i == 0 {
			ranges[months+weeks+days-i-1] = types.DateRange{
				StartDate: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
				EndDate:   now,
			}
		} else {
			from := addMonthYear(now, i * -1)
			daysInMonth := daysInMonths[from.Month()-1]
			if from.Month() == 2 {
				daysInMonth++
			}
			ranges[months+weeks+days-i-1] = types.DateRange{
				StartDate: from,
				EndDate:   from.Add(time.Hour * time.Duration(24*daysInMonth)).Add(time.Nanosecond*-1),
			}
		}
	}
	ranges[days+weeks+months] = types.DateRange{
		StartDate: time.Unix(0, 0),
		EndDate:   startOfDay.Add(24 * time.Hour),
	}
	return
}

// GetHealthStatuses health status
func (s *DashboardManager) GetHealthStatuses() HealthStatusResponse {
	var resp HealthStatusResponse
	resp.OverallStatus, resp.ServiceStatuses = s.heathMonitor.GetAllStatuses()
	return resp
}

// CountContainerEvents returns count of events by methods
func (s *DashboardManager) CountContainerEvents() map[common.TaskMethod]int {
	return s.resourceManager.CountContainerEvents()
}

// AntRegistrations returns ant registrations
func (s *DashboardManager) AntRegistrations() []*common.AntRegistration {
	return s.resourceManager.Registrations()
}

// GetJobStats returns job stats
func (s *DashboardManager) GetJobStats(qc *common.QueryContext) []*stats.JobStats {
	return s.jobStatsRegistry.GetStats(qc, 0, 500)
}

// GetJobTypes - finds job types
func (s *DashboardManager) GetJobTypes(
	qc *common.QueryContext,
) ([]types.JobTypeCronTrigger, error) {
	key := fmt.Sprintf("GetJobTypes:%s", qc.String())
	item, err := s.cache.Fetch(key,
		s.serverCfg.Jobs.DBObjectCache, func() (interface{}, error) {
			return s.repositoryFactory.JobDefinitionRepository.GetJobTypesAndCronTrigger(qc)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]types.JobTypeCronTrigger), nil
}

// RunningWaitingDoneJobsCount - finds running/waiting/done job counts
func (s *DashboardManager) RunningWaitingDoneJobsCount(
	qc *common.QueryContext,
	start time.Time,
	end time.Time,
	stat []*stats.JobStats) (running int64, waiting int64, done int64, succeeded int64, failed int64, err error) {
	var counts []*types.JobCounts
	counts, err = s.JobCounts(qc, start, end)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	completedByType := make(map[string]int64)
	failedByType := make(map[string]int64)
	for _, c := range counts {
		if c.JobState.Completed() {
			succeeded += c.Counts
			completedByType[c.GetUserJobTypeKey()] += c.Counts
		} else if c.JobState.Failed() {
			failed += c.Counts
			failedByType[c.GetUserJobTypeKey()] += c.Counts
		}
		if c.JobState.Running() {
			running += c.Counts
		} else if c.JobState.Waiting() {
			waiting += c.Counts
		} else if c.JobState.Done() {
			done += c.Counts
		}
	}
	if stat != nil {
		for _, st := range stat {
			if completedByType[st.JobKey.GetUserJobTypeKey()] > 0 {
				st.SucceededJobs = completedByType[st.JobKey.GetUserJobTypeKey()]
			}
			if failedByType[st.JobKey.GetUserJobTypeKey()] > 0 {
				st.FailedJobs = failedByType[st.JobKey.GetUserJobTypeKey()]
			}
			if st.SucceededJobs+st.FailedJobs > 0 {
				st.SucceededJobsPercentages = st.SucceededJobs * 100 / (st.SucceededJobs + st.FailedJobs)
			}
		}
	}
	return
}

// JobCounts - finds job counts
func (s *DashboardManager) JobCounts(
	qc *common.QueryContext,
	start time.Time,
	end time.Time) ([]*types.JobCounts, error) {
	key := fmt.Sprintf("JobCounts:%s:%s:%s", qc, start.Format("Jan _2"), end.Format("Jan _2"))
	item, err := s.cache.Fetch(key,
		s.serverCfg.Jobs.DBObjectCache, func() (interface{}, error) {
			return s.repositoryFactory.JobRequestRepository.JobCounts(qc, start, end)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]*types.JobCounts), nil
}

// OrgCounts - finds org counts
func (s *DashboardManager) OrgCounts() (int64, error) {
	item, err := s.cache.Fetch("OrgCounts",
		s.serverCfg.Jobs.DBObjectCache*2, func() (interface{}, error) {
			params := make(map[string]interface{})
			return s.repositoryFactory.OrgRepository.Count(
				common.NewQueryContext("", "", ""),
				params)
		})
	if err != nil {
		return 0, err
	}
	return item.Value().(int64), nil
}

// JobDefinitionCounts - finds job-definition counts
func (s *DashboardManager) JobDefinitionCounts(
	qc *common.QueryContext,
) (int64, error) {
	key := fmt.Sprintf("JobDefinitionCounts:%s", qc.String())
	item, err := s.cache.Fetch(key,
		s.serverCfg.Jobs.DBObjectCache, func() (interface{}, error) {
			params := make(map[string]interface{})
			return s.repositoryFactory.JobDefinitionRepository.Count(qc, params)
		})
	if err != nil {
		return 0, err
	}
	return item.Value().(int64), nil
}

// UserCounts - finds user counts
func (s *DashboardManager) UserCounts(
	qc *common.QueryContext,
) (int64, error) {
	key := fmt.Sprintf("UserCounts:%s", qc.String())
	item, err := s.cache.Fetch(key,
		s.serverCfg.Jobs.DBObjectCache*2, func() (interface{}, error) {
			params := make(map[string]interface{})
			return s.repositoryFactory.UserRepository.Count(qc, params)
		})
	if err != nil {
		return 0, err
	}
	return item.Value().(int64), nil
}

// JobCountsByDays - finds job counts by days
func (s *DashboardManager) JobCountsByDays(
	qc *common.QueryContext,
	limit int,
) ([]*types.JobCountsByDay, error) {
	key := fmt.Sprintf("JobCountsByDays:%s:%d", qc, limit)
	item, err := s.cache.Fetch(key,
		s.serverCfg.Jobs.DBObjectCache, func() (interface{}, error) {
			counts, err := s.repositoryFactory.JobRequestRepository.JobCountsByDays(qc, limit)
			if err != nil {
				return nil, err
			}
			countsByDay := make(map[string]*types.JobCountsByDay)
			for _, c := range counts {
				d := countsByDay[c.Day]
				if d == nil {
					d = &types.JobCountsByDay{
						Day: c.Day,
					}
				}
				if c.JobState.Completed() {
					d.SucceededCounts += c.Counts
				} else if c.JobState.Failed() {
					d.FailedCounts += c.Counts
				}
				countsByDay[c.Day] = d
			}
			res := make([]*types.JobCountsByDay, len(countsByDay))
			i := 0
			for _, v := range countsByDay {
				res[i] = v
				i++
			}
			sort.Slice(res, func(i, j int) bool { return res[i].Day < res[j].Day })
			return res, nil
		})
	if err != nil {
		return nil, err
	}
	return item.Value().([]*types.JobCountsByDay), nil
}
