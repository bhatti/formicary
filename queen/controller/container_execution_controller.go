package controller

import (
	"context"
	"fmt"
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/resource"
)

// ContainerExecutionController structure
type ContainerExecutionController struct {
	resourceManager resource.Manager
	webserver       web.Server
}

// NewContainerExecutionController instantiates controller for updating error-codes
func NewContainerExecutionController(
	resourceManager resource.Manager,
	webserver web.Server) *ContainerExecutionController {
	cec := &ContainerExecutionController{
		resourceManager: resourceManager,
		webserver:       webserver,
	}
	webserver.GET("/api/executors", cec.queryContainerExecutions, acl.NewPermission(acl.Container, acl.Query)).Name = "query_executors"
	webserver.DELETE("/api/executors/:id", cec.deleteContainerExecution, acl.NewPermission(acl.Container, acl.Delete)).Name = "delete_executor"
	return cec
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/executors container-executions queryContainerExecutions
// Queries container executions.
// `This requires admin access`
// responses:
//   200: containerExecutionsQueryResponse
func (cec *ContainerExecutionController) queryContainerExecutions(c web.WebContext) error {
	_, order, page, pageSize, _, _ := ParseParams(c)
	sortField := ""
	if len(order) > 0 {
		sortField = order[0]
	}
	recs, total := cec.resourceManager.GetContainerEvents(page*pageSize, pageSize, sortField)
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, int64(total), page, len(recs)))
}

// swagger:route GET /api/executors/{id} container-executions deleteContainerExecution
// Deletes container-executor by its id.
// `This requires admin access`
// responses:
//   200: emptyResponse
func (cec *ContainerExecutionController) deleteContainerExecution(c web.WebContext) error {
	id := c.Param("id")
	if id == "" {
		return types.NewValidationError(fmt.Errorf("failed to find container id"))
	}

	antID := c.FormValue("antID")
	if antID == "" {
		return types.NewValidationError(fmt.Errorf("antID is not specified in query params"))
	}
	method := c.FormValue("method")
	if method == "" {
		return types.NewValidationError(fmt.Errorf("failed to find method param in form data"))
	}
	err := cec.resourceManager.TerminateContainer(context.Background(), id, antID, types.TaskMethod(method))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryContainerExecutions
// The params for querying container-executions
type containerExecutionsQueryParams struct {
	// in:query
}

// Paginated results of container-executions matching query
// swagger:response containerExecutionsQueryResponse
type containerExecutionsQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []events.ContainerLifecycleEvent
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters deleteContainerExecution
// The parameters for finding container by id
type containerIDParamsBody struct {
	// in:path
	ID string `json:"id"`
}
