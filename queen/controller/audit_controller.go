package controller

import (
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// AuditController structure
type AuditController struct {
	auditRecordRepository repository.AuditRecordRepository
	webserver             web.Server
}

// NewAuditController instantiates controller for updating audits
func NewAuditController(
	auditRecordRepository repository.AuditRecordRepository,
	webserver web.Server) *AuditController {
	c := &AuditController{
		auditRecordRepository: auditRecordRepository,
		webserver:             webserver,
	}
	webserver.GET("/api/audits", c.queryAudits, acl.New(acl.Audit, acl.Query)).Name = "query_audits"
	return c
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/audits audits queryAudits
// Queries audits within the organization that is allowed.
// responses:
//   200: auditQueryResponse
func (uc *AuditController) queryAudits(c web.WebContext) error {
	params, order, page, pageSize, _ := ParseParams(c)
	recs, total, err := uc.auditRecordRepository.Query(
		params,
		page,
		pageSize,
		order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryAudits
// The params for querying audits.
type auditQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// TargetID defines target id
	TargetID string `json:"target_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// Kind defines type of audit record
	Kind types.AuditKind `json:"kind"`
	// JobType - job-type
	JobType string `json:"job_type"`
}

// Paginated results of audits matching query
// swagger:response auditQueryResponse
type auditQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*types.AuditRecord
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}
