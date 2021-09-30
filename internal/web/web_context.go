package web

import (
	"github.com/labstack/echo/v4"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	common "plexobject.com/formicary/internal/types"
)

// WebContext interface
type WebContext interface { //nolint
	// Path Request path
	Path() string

	// Request returns `*http.Request`.
	Request() *http.Request

	// Response returns `*Response`.
	Response() *echo.Response

	// Param returns path parameter by name.
	Param(name string) string

	// QueryParams returns the query parameters as `url.Values`.
	QueryParams() url.Values

	// QueryParam returns the query param for the provided name.
	QueryParam(name string) string

	// FormParams returns the form parameters as `url.Values`.
	FormParams() (url.Values, error)

	// FormValue returns the form field value for the provided name.
	FormValue(name string) string

	// Cookie returns the named cookie provided in the request.
	Cookie(name string) (*http.Cookie, error)

	// SetCookie adds a `Set-Cookie` header in HTTP response.
	SetCookie(cookie *http.Cookie)

	// Get retrieves data from the context.
	Get(key string) interface{}

	// Set saves data in the context.
	Set(key string, val interface{})

	// Render renders a template with data and sends a text/html response with status
	// code. Renderer must be registered using `Echo.Renderer`.
	Render(code int, name string, data interface{}) error

	// String sends a string response with status code.
	String(code int, s string) error

	// JSON sends a JSON response with status code.
	JSON(code int, i interface{}) error

	// MultipartForm returns the multipart form.
	MultipartForm() (*multipart.Form, error)

	// Redirect redirects the request to a provided URL with status code.
	Redirect(code int, url string) error

	// NoContent sends a response with nobody and a status code.
	NoContent(code int) error

	// Blob sends a blob response with status code and content type.
	Blob(code int, contentType string, b []byte) error

	// Stream sends a streaming response with status code and content type.
	Stream(code int, contentType string, r io.Reader) error
	// Attachment sends a response as attachment, prompting client to save the
	// file.
	Attachment(file string, name string) error
}

// LoggedInUser constant
const LoggedInUser = "LoggedInUser"

// AppVersion constant
const AppVersion = "AppVersion"

// DBUser constant
const DBUser = "DBUser"

// DBUserOrg constant
const DBUserOrg = "DBUserOrg"

// AuthDisabled constant
const AuthDisabled = "AuthDisabled"

// RenderDBUserFromSession initializes user/admin parameters
func RenderDBUserFromSession(c WebContext, res map[string]interface{}) {
	user := GetDBLoggedUserFromSession(c)
	if c.Get(AppVersion) != nil {
		res[AppVersion] = c.Get(AppVersion)
	}
	if res["Q"] == nil {
		res["Q"] = ""
	}
	if user != nil {
		res[DBUser] = user
		res[DBUserOrg] = user.OrganizationID
		res["Admin"] = user.IsAdmin()
		res["ReadAdmin"] = user.IsReadAdmin()
	} else if c.Get(AuthDisabled) != nil {
		res[DBUserOrg] = ""
		res["Admin"] = true
		res["ReadAdmin"] = true
	}
}

// GetDBUserFromSession returns database-user from web context
func GetDBUserFromSession(c WebContext) *common.User {
	user := c.Get(DBUser)
	if user != nil {
		return user.(*common.User)
	}
	return nil
}

// GetDBLoggedUserFromSession returns logged-in user from web context
func GetDBLoggedUserFromSession(c WebContext) *common.User {
	user := GetDBUserFromSession(c)
	if user != nil {
		return user
	}
	if c.Get(LoggedInUser) != nil {
		return c.Get(LoggedInUser).(*common.User)
	}
	return nil
}

// BuildQueryContext returns query-context for scoping queries by user/org
func BuildQueryContext(c WebContext) *common.QueryContext {
	return common.NewQueryContext(
		GetDBLoggedUserFromSession(c),
		c.Request().RemoteAddr)
}

// IsWhiteListURL checks if path is white listed -- does not require authentication
func IsWhiteListURL(path string, method string) bool {
	if strings.HasPrefix(path, "/docs") && method == "GET" {
		return true
	}
	whitelistGetURLs := map[string]bool{
		"/":                     true,
		"/login":                true,
		"/logout":               true,
		"/auth/google":          true,
		"/auth/google/callback": true,
		"/auth/github":          true,
		"/auth/github/callback": true,
		"/dashboard/users/new":  true,
		"/terms_service":        true,
		"/privacy_policies":     true,
		"/favicon.ico":          true,
	}

	whitelistPostURLs := map[string]bool{
		"/dashboard/users":     true,
		"/auth/github/webhook": true,
	}
	return (whitelistGetURLs[path] && method == "GET") ||
		(whitelistPostURLs[path] && method == "POST")
}
