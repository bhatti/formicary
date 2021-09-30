package admin

import (
	"encoding/json"
	"net/http"
	"plexobject.com/formicary/internal/utils"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"

	"plexobject.com/formicary/internal/web"
)

// DashboardAdminController structure
type DashboardAdminController struct {
	dashboardStats *manager.DashboardManager
	webserver      web.Server
}

// NewDashboardAdminController admin dashboard
func NewDashboardAdminController(
	dashboardStats *manager.DashboardManager,
	webserver web.Server) *DashboardAdminController {
	jraCtr := &DashboardAdminController{
		dashboardStats: dashboardStats,
		webserver:      webserver,
	}
	webserver.GET("/dashboard", jraCtr.dashboard, acl.NewPermission(acl.Dashboard, acl.View)).Name = "admin_dashboard"

	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// dashboard -
func (ctr *DashboardAdminController) dashboard(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	start := utils.ParseStartDateTime(c.QueryParam("start"))
	end := utils.ParseEndDateTime(c.QueryParam("end"))

	res := make(map[string]interface{})
	jobStats := ctr.dashboardStats.GetJobStats(qc)
	res["JobStats"] = jobStats

	healthStatusResponse := ctr.dashboardStats.GetHealthStatuses()
	res["Health"] = healthStatusResponse

	// RBAC assumes empty org is admin
	if qc.IsAdmin() {
		ants := ctr.dashboardStats.AntRegistrations()
		res["AntRegistrations"] = ants
		res["AntRegistrationsCount"] = len(ants)
	} else {
		res["AntRegistrations"] = make([]*common.AntRegistration, 0)
		res["AntRegistrationsCount"] = 0
	}
	res["JobCounts"] = make([]*types.JobCounts, 0)
	res["OrgCounts"] = 0
	res["UserCounts"] = 0
	res["JobDefinitionCounts"] = 0
	res["PluginCounts"] = 0
	res["RunningJobsCount"] = 0
	res["WaitingJobsCount"] = 0
	res["DoneJobsCount"] = 0
	res["SucceededJobsCount"] = 0
	res["SuccessPercentage"] = 0
	res["FailedJobsCount"] = 0
	if counts, err := ctr.dashboardStats.JobCounts(
		qc,
		start,
		end); err == nil {
		res["JobCounts"] = counts
		running, waiting, done, succeeded, failed, _ := ctr.dashboardStats.RunningWaitingDoneJobsCount(
			qc,
			start,
			end,
			jobStats,
		)
		res["RunningJobsCount"] = running
		res["WaitingJobsCount"] = waiting
		res["DoneJobsCount"] = done
		res["SucceededJobsCount"] = succeeded
		res["FailedJobsCount"] = failed
		if succeeded+failed > 0 {
			res["SuccessPercentage"] = succeeded * 100 / (succeeded + failed)
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "DashboardAdminController",
			"Org":       qc,
			"Error":     err,
		}).Warnf("failed to get job counts")
	}

	jobDefinitionsCount := 0
	if total, err := ctr.dashboardStats.JobDefinitionCounts(qc); err == nil {
		res["JobDefinitionCounts"] = total
		jobDefinitionsCount = int(total)
	}

	pluginsCount := 0
	if total, err := ctr.dashboardStats.PluginCounts(); err == nil {
		res["PluginCounts"] = total
		pluginsCount = int(total)
	}

	if total, err := ctr.dashboardStats.UserCounts(qc); err == nil {
		res["UserCounts"] = total
	}

	if qc.IsAdmin() {
		if total, err := ctr.dashboardStats.OrgCounts(); err == nil {
			res["OrgCounts"] = total
		}
	}

	jobCountsByDays := make(map[string]interface{})
	// TODO limit jobDefinitionsCount+pluginsCount
	if counts, err := ctr.dashboardStats.JobCountsByDays(
		qc,
		(jobDefinitionsCount+pluginsCount)*3*30); err == nil {
		labels := make([]string, len(counts))
		succeededData := make([]int64, len(counts))
		failedData := make([]int64, len(counts))
		for i, c := range counts {
			labels[i] = c.Day
			succeededData[i] = c.SucceededCounts
			failedData[i] = c.FailedCounts
		}
		jobCountsByDays["labels"] = labels
		jobCountsByDays["datasets"] = []interface{}{
			map[string]interface{}{"data": succeededData, "label": "Succeeded", "backgroundColor": "#198754", "fill": false},
			map[string]interface{}{"data": failedData, "label": "Failed", "backgroundColor": "#dc3545", "fill": false},
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "DashboardAdminController",
			"Org":       qc,
			"Error":     err,
		}).Warnf("failed to get job counts by days")
	}
	if b, err := json.Marshal(jobCountsByDays); err == nil {
		res["JobCountsByDays"] = string(b)
	} else {
		res["JobCountsByDays"] = "{}"
	}

	containerCounts := ctr.dashboardStats.CountContainerEvents()
	containerCountsMap := make(map[string]interface{})
	{
		defaultColors := []string{"#6c757d", "#ffc107", "#0dcaf0", "#198754", "#dc3545", "#f8f9fa"}
		labels := make([]string, len(containerCounts))
		i := 0
		counts := make([]int, len(containerCounts))
		colors := make([]string, len(containerCounts))
		totalExecutors := 0
		for method, count := range containerCounts {
			labels[i] = string(method)
			counts[i] = count
			colors[i] = defaultColors[i%len(defaultColors)]
			totalExecutors += count
			i++
		}
		res["TotalExecutors"] = totalExecutors
		containerCountsMap["labels"] = labels
		containerCountsMap["datasets"] = []interface{}{
			map[string]interface{}{"data": counts, "label": "Executors", "backgroundColor": colors},
		}
		if b, err := json.Marshal(containerCountsMap); err == nil {
			res["ContainerCounts"] = string(b)
		} else {
			res["ContainerCounts"] = "{}"
		}
	}
	usageDays := 10
	ranges := manager.BuildRanges(time.Now(), usageDays, 0, 0, false)
	user := web.GetDBLoggedUserFromSession(c)
	res["ArtifactCounts"] = 0
	if numArtifacts, err := ctr.dashboardStats.GetStorageCount(qc); err == nil {
		res["ArtifactCounts"] = numArtifacts
	}
	if user != nil && user.Subscription != nil {
		ranges = append(ranges, types.DateRange{StartDate: user.Subscription.StartedAt, EndDate: user.Subscription.EndedAt})
	}
	if cpuUsage, err := ctr.dashboardStats.GetCPUResources(
		qc, ranges); err == nil {
		labels := make([]string, usageDays)
		cpuData := make([]int64, usageDays)
		storageData := make([]int64, usageDays)
		for i := 0; i < usageDays; i++ {
			labels[i] = cpuUsage[i].StartDate.Format("Jan _2")
			cpuData[i] = cpuUsage[i].Value / 60
		}
		if user != nil && user.Subscription != nil {
			res["SubscriptionCPUUsage"] = cpuUsage[len(cpuUsage)-1].ValueString()
		}
		if storageUsage, err := ctr.dashboardStats.GetStorageResources(
			qc, ranges); err == nil {
			for i := 0; i < usageDays; i++ {
				storageData[i] = storageUsage[i].Value / 1000 / 1000
			}
			if user != nil && user.Subscription != nil {
				res["SubscriptionDiskUsage"] = storageUsage[len(storageUsage)-1].ValueString()
			}
		}

		usageMap := make(map[string]interface{})
		usageMap["labels"] = labels
		usageMap["datasets"] = []interface{}{
			map[string]interface{}{"data": cpuData, "label": "CPU Usage (Minutes)", "backgroundColor": "#99d9df", "fill": false},
			map[string]interface{}{"data": storageData, "label": "Disk Usage (MB)", "backgroundColor": "#e3c598", "fill": false},
		}
		if b, err := json.Marshal(usageMap); err == nil {
			res["ResourcesUsage"] = string(b)
		} else {
			res["ResourcesUsage"] = "{}"
		}
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "dashboard/dashboard", res)
}
