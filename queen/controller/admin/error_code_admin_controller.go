package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/repository"
)

// ErrorCodeAdminController structure
type ErrorCodeAdminController struct {
	errorCodeRepository repository.ErrorCodeRepository
	webserver           web.Server
}

// NewErrorCodeAdminController admin dashboard for managing error-codes -- admin only
func NewErrorCodeAdminController(
	repo repository.ErrorCodeRepository,
	webserver web.Server) *ErrorCodeAdminController {
	jraCtr := &ErrorCodeAdminController{
		errorCodeRepository: repo,
		webserver:           webserver,
	}
	webserver.GET("/dashboard/errors", jraCtr.queryErrorCodes, acl.NewPermission(acl.ErrorCode, acl.View)).Name = "query_admin_error_codes"
	webserver.GET("/dashboard/errors/new", jraCtr.newErrorCode, acl.NewPermission(acl.ErrorCode, acl.View)).Name = "new_admin_error_code"
	webserver.POST("/dashboard/errors", jraCtr.createErrorCode, acl.NewPermission(acl.ErrorCode, acl.Create)).Name = "create_admin_error_code"
	webserver.POST("/dashboard/errors/:id", jraCtr.updateErrorCode, acl.NewPermission(acl.ErrorCode, acl.Update)).Name = "update_admin_error_code"
	webserver.GET("/dashboard/errors/:id", jraCtr.getErrorCode, acl.NewPermission(acl.ErrorCode, acl.View)).Name = "get_admin_error_code"
	webserver.GET("/dashboard/errors/:id/edit", jraCtr.editErrorCode, acl.NewPermission(acl.ErrorCode, acl.Update)).Name = "edit_admin_error_code"
	webserver.POST("/dashboard/errors/:id/delete", jraCtr.deleteErrorCode, acl.NewPermission(acl.ErrorCode, acl.Delete)).Name = "delete_admin_error_code"
	return jraCtr
}

// ********************************* HTTP Handlers ***********************************
// queryErrorCodes - queries error-code
func (jraCtr *ErrorCodeAdminController) queryErrorCodes(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, q, qs := controller.ParseParams(c)
	recs, total, err := jraCtr.errorCodeRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("/dashboard/errors?%s", q)
	pagination := controller.Pagination(page, pageSize, total, baseURL)
	res := map[string]interface{}{
		"Records":    recs,
		"Pagination": pagination,
		"BaseURL":    baseURL,
		"Q":          qs,
	}
	for _, rec := range recs {
		rec.CanEdit = (rec.OrganizationID != "" &&
			qc.GetOrganizationID() != "" &&
			qc.GetOrganizationID() == rec.OrganizationID) ||
			qc.GetUserID() == rec.UserID || qc.IsAdmin()
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "errors/index", res)
}

// createErrorCode - saves a new error-code
func (jraCtr *ErrorCodeAdminController) createErrorCode(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	errorCode := buildError(c)
	err = errorCode.Validate()
	if err == nil {
		errorCode, err = jraCtr.errorCodeRepository.Save(qc, errorCode)
	}
	if err != nil {
		res := map[string]interface{}{
			"Error": errorCode,
		}
		if errorCode != nil && len(errorCode.Errors) == 0 {
			errorCode.Errors = map[string]string{"Error": err.Error()}
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "errors/new", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/errors/%s", errorCode.ID))
}

// updateErrorCode - updates error-code
func (jraCtr *ErrorCodeAdminController) updateErrorCode(c web.WebContext) (err error) {
	qc := web.BuildQueryContext(c)
	errorCode := buildError(c)
	errorCode.ID = c.Param("id")
	err = errorCode.Validate()

	if err == nil {
		errorCode, err = jraCtr.errorCodeRepository.Save(qc, errorCode)
	}
	if err != nil {
		res := map[string]interface{}{
			"Error": errorCode,
		}
		if errorCode != nil && len(errorCode.Errors) == 0 {
			errorCode.Errors = map[string]string{"Error": err.Error()}
		}
		web.RenderDBUserFromSession(c, res)
		return c.Render(http.StatusOK, "errors/edit", res)
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard/errors/%s", errorCode.ID))
}

// newErrorCode - creates a new system error
func (jraCtr *ErrorCodeAdminController) newErrorCode(c web.WebContext) error {
	errorCode := common.NewErrorCode("", "", "", "")
	res := map[string]interface{}{
		"Error": errorCode,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "errors/new", res)
}

// getErrorCode - finds error-code by id
func (jraCtr *ErrorCodeAdminController) getErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	errorCode, err := jraCtr.errorCodeRepository.Get(qc, id)
	if err != nil {
		return err
	}
	res := make(map[string]interface{})
	res["Error"] = errorCode
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "errors/view", res)
}

// editErrorCode - shows error-code for edit
func (jraCtr *ErrorCodeAdminController) editErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	id := c.Param("id")
	errorCode, err := jraCtr.errorCodeRepository.Get(qc, id)
	if err != nil {
		errorCode = common.NewErrorCode("", "", "", "")
		errorCode.Errors = map[string]string{"Error": err.Error()}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "ErrorCodeAdminController",
				"Error":     err,
				"ID":        id,
			}).Debug("failed to find errorCode")
		}
	}
	res := make(map[string]interface{})
	res["Error"] = errorCode
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "errors/edit", res)
}

// deleteErrorCode - deletes error-code by id
func (jraCtr *ErrorCodeAdminController) deleteErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := jraCtr.errorCodeRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusFound, "/dashboard/errors")
}

func buildError(c web.WebContext) *common.ErrorCode {
	errorCode := common.NewErrorCode(
		c.FormValue("jobType"),
		c.FormValue("regex"),
		c.FormValue("command"),
		c.FormValue("errorCode"),
		)
	errorCode.Description = c.FormValue("description")
	errorCode.DisplayMessage = c.FormValue("displayMessage")
	errorCode.DisplayCode = c.FormValue("displayCode")
	errorCode.TaskTypeScope = c.FormValue("taskTypeScope")
	errorCode.Action = common.ErrorCodeAction(c.FormValue("action"))
	errorCode.ExitCode, _ = strconv.Atoi(c.FormValue("exitCode"))
	errorCode.Retry, _ = strconv.Atoi(c.FormValue("retry"))
	// TODO add HardFailure
	return errorCode
}
