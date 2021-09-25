package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/resource"
)

// ExecutionContainerAdminController structure
type ExecutionContainerAdminController struct {
	resourceManager resource.Manager
	webserver       web.Server
}

// NewExecutionContainerAdminController admin dashboard for managing error-codes -- only admin can access it
func NewExecutionContainerAdminController(
	resourceManager resource.Manager,
	webserver web.Server) *ExecutionContainerAdminController {
	eca := &ExecutionContainerAdminController{
		resourceManager: resourceManager,
		webserver:       webserver,
	}
	webserver.GET("/dashboard/executors", eca.queryExecutionContainers, acl.NewPermission(acl.Container, acl.Query)).Name = "query_admin_executors"
	webserver.POST("/dashboard/executors/:id/delete", eca.deleteExecutionContainer, acl.NewPermission(acl.Container, acl.Delete)).Name = "delete_admin_executor"
	return eca
}

// ********************************* HTTP Handlers ***********************************
// queryExecutionContainers - queries error-code
func (eca *ExecutionContainerAdminController) queryExecutionContainers(c web.WebContext) error {
	_, order, page, pageSize, q, qs := controller.ParseParams(c)
	sortField := ""
	if len(order) > 0 {
		sortField = order[0]
	}
	recs, total := eca.resourceManager.GetContainerEvents(page*pageSize, pageSize, sortField)
	baseURL := fmt.Sprintf("/dashboard/executors?%s", q)
	pagination := controller.Pagination(page, pageSize, int64(total), baseURL)
	res := map[string]interface{}{"Executors": recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "executors/index", res)
}

// deleteExecutionContainer - deletes error-code by id
func (eca *ExecutionContainerAdminController) deleteExecutionContainer(c web.WebContext) error {
	id := c.FormValue("id")
	if id == "" {
		return fmt.Errorf("failed to find container id")
	}

	antID := c.FormValue("antID")
	if antID == "" {
		return fmt.Errorf("antID is not specified in form")
	}
	method := c.FormValue("method")
	if method == "" {
		return fmt.Errorf("failed to find method")
	}
	err := eca.resourceManager.TerminateContainer(context.Background(), id, antID, types.TaskMethod(method))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobLauncher",
			"ID":        id,
			"AntID":     antID,
			"Method":    method,
			"Error":     err,
		}).Warn("failed to terminate container")
	}
	return c.Redirect(http.StatusFound, "/dashboard/executors")
}
