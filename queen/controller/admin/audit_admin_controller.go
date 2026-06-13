package admin

import (
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// AuditAdminController structure
type AuditAdminController struct {
	auditRecordRepository repository.AuditRecordRepository
	jobRequestRepository  repository.JobRequestRepository
	webserver             web.Server
}

// NewAuditAdminController admin dashboard for managing audits
func NewAuditAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	jobRequestRepository repository.JobRequestRepository,
	webserver web.Server) *AuditAdminController {
	ctr := &AuditAdminController{
		auditRecordRepository: auditRecordRepository,
		jobRequestRepository:  jobRequestRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/audits", ctr.queryAudits, acl.NewPermission(acl.User, acl.Query)).Name = "query_admin_audits"
	webserver.GET("/dashboard/audits/job_submissions", ctr.jobSubmissionsReport, acl.NewPermission(acl.User, acl.Query)).Name = "admin_job_submissions_report"
	return ctr
}

// ********************************* HTTP Handlers ***********************************
// jobSubmissionsReport - shows jobs submitted aggregated by username
func (c *AuditAdminController) jobSubmissionsReport(ctx web.APIContext) error {
	params, order, page, pageSize, _, _ := controller.ParseParams(ctx)
	qc := web.BuildQueryContext(ctx)
	// Non-admin users can only see their own organization's data
	if !qc.IsAdmin() {
		params["organization_id"] = qc.GetOrganizationID()
	}
	recs, total, err := c.jobRequestRepository.QueryJobSubmissions(params, page, pageSize, order)
	if err != nil {
		return err
	}
	// Build baseURL after org enforcement so pagination links preserve the filter
	baseURL := controller.BuildBaseURL("/dashboard/audits/job_submissions", params, pageSize)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	orgID, _ := params["organization_id"].(string)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"OrgID":      orgID,
		"FromDate":   ctx.QueryParam("from"),
		"ToDate":     ctx.QueryParam("to"),
		"IsAdmin":    qc.IsAdmin(),
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "audits/job_submissions", res)
}

// queryAudits - queries audit
func (c *AuditAdminController) queryAudits(ctx web.APIContext) error {
	params, order, page, pageSize, _, qs := controller.ParseParams(ctx)
	qc := web.BuildQueryContext(ctx)
	// Non-admin users can only see their own organization's data
	if !qc.IsAdmin() {
		params["organization_id"] = qc.GetOrganizationID()
	}
	var kind string
	if params["kind"] != nil {
		kind = params["kind"].(string)
	}
	recs, total, err := c.auditRecordRepository.Query(
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	// Build baseURL after org enforcement so pagination links preserve the filter
	baseURL := controller.BuildBaseURL("/dashboard/audits", params, pageSize)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
		"Kind":       kind,
	}
	if kinds, err := c.auditRecordRepository.GetKinds(); err == nil {
		kinds = append([]types.AuditKind{""}, kinds...)
		res["Kinds"] = kinds
	} else {
		res["Kinds"] = []types.AuditKind{""}
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "audits/index", res)
}
