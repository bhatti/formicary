package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

// ErrorCodeController structure
type ErrorCodeController struct {
	errorCodeRepository repository.ErrorCodeRepository
	webserver           web.Server
}

// NewErrorCodeController instantiates controller for updating error-codes
func NewErrorCodeController(
	repo repository.ErrorCodeRepository,
	webserver web.Server) *ErrorCodeController {
	ecCtrl := &ErrorCodeController{
		errorCodeRepository: repo,
		webserver:           webserver,
	}
	webserver.GET("/api/errors", ecCtrl.queryErrorCodes, acl.NewPermission(acl.ErrorCode, acl.Query)).Name = "query_errors"
	webserver.GET("/api/errors/:id", ecCtrl.getErrorCode, acl.NewPermission(acl.ErrorCode, acl.View)).Name = "get_error"
	webserver.POST("/api/errors", ecCtrl.postErrorCode, acl.NewPermission(acl.ErrorCode, acl.Create)).Name = "create_error"
	webserver.PUT("/api/errors/:id", ecCtrl.putErrorCode, acl.NewPermission(acl.ErrorCode, acl.Update)).Name = "update_error"
	webserver.DELETE("/api/errors/:id", ecCtrl.deleteErrorCode, acl.NewPermission(acl.ErrorCode, acl.Delete)).Name = "delete_error"
	return ecCtrl
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/errors error-codes queryErrorCodes
// Queries error-codes by type, regex.
// `This requires admin access`
// responses:
//   200: errorCodesQueryResponse
func (ecCtrl *ErrorCodeController) queryErrorCodes(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	recs, err := ecCtrl.errorCodeRepository.GetAll(qc)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, int64(len(recs)), 0, len(recs)))
}

// swagger:route POST /api/errors error-codes postErrorCode
// Creates new error code based on request body.
// `This requires admin access`
// responses:
//   200: errorCodeResponse
func (ecCtrl *ErrorCodeController) postErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	now := time.Now()
	ec := common.NewErrorCode("", "", "", "")
	err := json.NewDecoder(c.Request().Body).Decode(ec)
	if err != nil {
		return err
	}
	saved, err := ecCtrl.errorCodeRepository.Save(qc, ec)
	if err != nil {
		return err
	}
	status := 0
	if saved.CreatedAt.Unix() >= now.Unix() {
		status = http.StatusCreated
	} else {
		status = http.StatusOK
	}
	return c.JSON(status, saved)
}

// swagger:route PUT /api/errors error-codes putErrorCode
// Updates new error code based on request body.
// `This requires admin access`
// responses:
//   200: errorCodeResponse
func (ecCtrl *ErrorCodeController) putErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	ec := common.NewErrorCode("", "", "", "")
	err := json.NewDecoder(c.Request().Body).Decode(ec)
	if err != nil {
		return err
	}
	saved, err := ecCtrl.errorCodeRepository.Save(qc, ec)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/errors/{id} error-codes getErrorCode
// Finds error code by id.
// `This requires admin access`
// responses:
//   200: errorCodeResponse
func (ecCtrl *ErrorCodeController) getErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	ec, err := ecCtrl.errorCodeRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, ec)
}

// swagger:route DELETE /api/errors/{id} error-codes deleteErrorCode
// Deletes error code by id.
// `This requires admin access`
// responses:
//   200: emptyResponse
func (ecCtrl *ErrorCodeController) deleteErrorCode(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	err := ecCtrl.errorCodeRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters queryErrorCodes
// The params for querying error-codes
type errorCodesQueryParams struct {
	// in:query
	// Regex matches error-code
	Regex string `json:"regex"`
	// ExitCode defines exit-code for error
	ExitCode int `json:"exit_code"`
	// ErrorCode defines error code
	ErrorCode string `json:"error_code"`
	// JobType defines type for the job
	JobType string `json:"job_type"`
	// TaskTypeScope only applies error code for task_type
	TaskTypeScope string `json:"task_type_scope"`
	// PlatformScope only applies error code for platform
	PlatformScope string `json:"platform_scope"`
	// HardFailure determines if this error can be retried or is hard failure
	HardFailure bool `json:"hard_failure"`
}

// Query results of error-codes
// swagger:response errorCodesQueryResponse
type errorCodesQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []common.ErrorCode
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters postErrorCode putErrorCode
// The params for error-code
type errorCodeParams struct {
	// in:body
	Body common.ErrorCode
}

// ErrorCode body for update
// swagger:response errorCodeResponse
type errorCodeResponseBody struct {
	// in:body
	Body common.ErrorCode
}

// swagger:parameters deleteErrorCode getErrorCode
// The parameters for finding error-code by id
type errorCoderIDParams struct {
	// in:path
	ID string `json:"id"`
}
