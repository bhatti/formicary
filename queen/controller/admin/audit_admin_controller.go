package admin

import (
	"fmt"
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
	webserver             web.Server
}

// NewAuditAdminController admin dashboard for managing audits
func NewAuditAdminController(
	auditRecordRepository repository.AuditRecordRepository,
	webserver web.Server) *AuditAdminController {
	ctr := &AuditAdminController{
		auditRecordRepository: auditRecordRepository,
		webserver:             webserver,
	}
	webserver.GET("/dashboard/audits", ctr.queryAudits, acl.NewPermission(acl.User, acl.Query)).Name = "query_admin_audits"
	return ctr
}

// ********************************* HTTP Handlers ***********************************
// queryAudits - queries audit
func (c *AuditAdminController) queryAudits(ctx web.APIContext) error {
	params, order, page, pageSize, q, qs := controller.ParseParams(ctx)
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
	baseURL := fmt.Sprintf("/dashboard/audits?%s", q)
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
