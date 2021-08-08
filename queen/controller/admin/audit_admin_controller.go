package admin

import (
	"fmt"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
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
	webserver.GET("/dashboard/audits", ctr.queryAudits, acl.New(acl.User, acl.Query)).Name = "query_admin_audits"
	return ctr
}

// ********************************* HTTP Handlers ***********************************
// queryAudits - queries audit
func (c *AuditAdminController) queryAudits(ctx web.WebContext) error {
	params, order, page, pageSize, q := controller.ParseParams(ctx)
	audits, total, err := c.auditRecordRepository.Query(
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/audits?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{"Audits": audits,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          "",
	}
	web.RenderDBUserFromSession(ctx, res)
	return ctx.Render(http.StatusOK, "audits/index", res)
}
