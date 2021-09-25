package controller

import (
	"net/http"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/resource"
)

// AntRegistrationController structure
type AntRegistrationController struct {
	resourceManager resource.Manager
	webserver       web.Server
}

// NewAntRegistrationController instantiates controller for ant registration
func NewAntRegistrationController(
	resourceManager resource.Manager,
	webserver web.Server) *AntRegistrationController {
	wrc := &AntRegistrationController{
		resourceManager: resourceManager,
		webserver:       webserver,
	}
	webserver.GET("/api/ants", wrc.queryAntRegistrations, acl.NewPermission(acl.AntExecutor, acl.Query)).Name = "query_ants"
	webserver.GET("/api/ants/:id", wrc.getAntRegistration, acl.NewPermission(acl.AntExecutor, acl.View)).Name = "get_ant"
	return wrc
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/ants ant-registrations queryAntRegistrations
// Queries ant registration.
// `This requires admin access`
// responses:
//   200: antRegistrationsQueryResponse
func (wrc *AntRegistrationController) queryAntRegistrations(c web.WebContext) error {
	recs := wrc.resourceManager.Registrations()
	return c.JSON(http.StatusOK, recs)
}

// swagger:route GET /api/ants/{id} ant-registrations getAntRegistration
// Retrieves ant-registration by its id.
// `This requires admin access`
// responses:
//   200: antRegistrationResponse
func (wrc *AntRegistrationController) getAntRegistration(c web.WebContext) error {
	rec := wrc.resourceManager.Registration(c.Param("id"))
	if rec == nil {
		return c.String(http.StatusNotFound, "could not find ant registration")
	}
	return c.JSON(http.StatusOK, rec)
}

// ********************************* Swagger types ***********************************

// The parameter for querying ant registration
// swagger:parameters queryAntRegistrations
type antQueryParams struct {
}

// Paginated results of ant-registrations matching query
// swagger:response antRegistrationsQueryResponse
type antRegistrationsQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []common.AntRegistration
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// The parameter for finding ant registration by id
// swagger:parameters getAntRegistration
type antIDParams struct {
	// in:path
	ID string `json:"id"`
}

// Ant Registration body
// swagger:response antRegistrationResponse
type antRegistrationResponseBody struct {
	// in:body
	Body common.AntRegistration
}
