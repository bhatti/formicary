package web

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/acl"
)

// ********************************* STUB METHODS For Web server ***********************************
type stubWebServer struct {
}

type stubContext struct {
	request  *http.Request
	response *echo.Response
	Result   interface{}
	Params   map[string]string
	Context  map[string]interface{}
}

func (c *stubContext) SetResponse(r *echo.Response) {
	c.response = r
}

func (c *stubContext) SetLogger(_ echo.Logger) {
}

// NewStubContext - creates stubbed server
func NewStubContext(req *http.Request) *stubContext {
	return &stubContext{
		request: req,
		Params:  make(map[string]string),
		Context: make(map[string]interface{}),
	}
}

// NewStubWebServer creates stubbed web server
func NewStubWebServer() Server {
	return &stubWebServer{}
}

// AddMiddleware middleware
func (w *stubWebServer) AddMiddleware(_ echo.MiddlewareFunc) {
}

func (w *stubWebServer) GET(string, HandlerFunc, *acl.Permission, ...echo.MiddlewareFunc) *echo.Route {
	return &echo.Route{}
}

func (w *stubWebServer) POST(string, HandlerFunc, *acl.Permission, ...echo.MiddlewareFunc) *echo.Route {
	return &echo.Route{}
}

func (w *stubWebServer) PUT(string, HandlerFunc, *acl.Permission, ...echo.MiddlewareFunc) *echo.Route {
	return &echo.Route{}
}

func (w *stubWebServer) DELETE(string, HandlerFunc, *acl.Permission, ...echo.MiddlewareFunc) *echo.Route {
	return &echo.Route{}
}

func (w *stubWebServer) Start(string) {
}

func (w *stubWebServer) Stop() {
}

// ********************************* STUB METHODS For Echo Context ***********************************

// Request getter
func (c *stubContext) Request() *http.Request {
	return c.request
}

// SetRequest setter
func (c *stubContext) SetRequest(r *http.Request) {
	c.request = r
}

// Response getter
func (c *stubContext) Response() *echo.Response {
	return c.response
}

// IsTLS getter
func (c *stubContext) IsTLS() bool {
	return true
}

// IsWebSocket getter
func (c *stubContext) IsWebSocket() bool {
	return false
}

// Scheme getter
func (c *stubContext) Scheme() string {
	return "https"
}

func (c *stubContext) RealIP() string {
	return "127.0.0.1"
}

func (c *stubContext) Path() string {
	return ""
}

func (c *stubContext) SetPath(string) {
}

func (c *stubContext) Param(name string) string {
	return c.Params[name]
}

func (c *stubContext) ParamNames() []string {
	return make([]string, 0)
}

func (c *stubContext) SetParamNames(...string) {
}

func (c *stubContext) ParamValues() []string {
	return make([]string, 0)
}

func (c *stubContext) SetParamValues(...string) {
}

func (c *stubContext) QueryParam(name string) string {
	return c.Params[name]
}

func (c *stubContext) QueryParams() url.Values {
	return c.request.URL.Query()
}

func (c *stubContext) QueryString() string {
	return ""
}

func (c *stubContext) FormValue(name string) string {
	return c.Params[name]
}

func (c *stubContext) FormParams() (url.Values, error) {
	return c.request.Form, nil
}

func (c *stubContext) FormFile(string) (*multipart.FileHeader, error) {
	return nil, nil
}

func (c *stubContext) MultipartForm() (*multipart.Form, error) {
	return nil, nil
}

func (c *stubContext) Cookie(name string) (*http.Cookie, error) {
	return c.request.Cookie(name)
}

func (c *stubContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response(), cookie)
}

func (c *stubContext) Cookies() []*http.Cookie {
	return c.request.Cookies()
}

func (c *stubContext) Get(name string) interface{} {
	return c.Context[name]
}

func (c *stubContext) Set(name string, value interface{}) {
	c.Context[name] = value
}

func (c *stubContext) Bind(interface{}) error {
	return nil
}

func (c *stubContext) Validate(interface{}) error {
	return nil
}

func (c *stubContext) Render(code int, t string, r interface{}) (err error) {
	c.Result = r
	if code >= 300 {
		return fmt.Errorf("%d - %s", code, t)
	}
	return nil
}

func (c *stubContext) HTML(code int, t string) (err error) {
	c.Result = t
	if code >= 300 {
		return fmt.Errorf("%d - %s", code, t)
	}
	return nil
}

func (c *stubContext) HTMLBlob(code int, b []byte) (err error) {
	c.Result = b
	if code >= 300 {
		return fmt.Errorf("%d - %d", code, len(b))
	}
	return nil
}

func (c *stubContext) String(code int, s string) (err error) {
	c.Result = s
	if code >= 300 {
		return fmt.Errorf("%d - %s", code, s)
	}
	return nil
}

func (c *stubContext) JSON(code int, j interface{}) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) JSONPretty(code int, j interface{}, _ string) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) JSONBlob(code int, b []byte) (err error) {
	c.Result = b
	if code >= 300 {
		return fmt.Errorf("%d - %d", code, len(b))
	}
	return nil
}

func (c *stubContext) JSONP(code int, j string, _ interface{}) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %s", code, j)
	}
	return nil
}

func (c *stubContext) JSONPBlob(code int, j string, _ []byte) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %s", code, j)
	}
	return nil
}

func (c *stubContext) XML(code int, j interface{}) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) XMLPretty(code int, j interface{}, _ string) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) XMLBlob(code int, j []byte) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) Blob(code int, j string, _ []byte) (err error) {
	c.Result = j
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) Stream(code int, j string, r io.Reader) (err error) {
	c.Result = r
	if code >= 300 {
		return fmt.Errorf("%d - %v", code, j)
	}
	return nil
}

func (c *stubContext) File(string) (err error) {
	return
}

func (c *stubContext) Attachment(string, string) error {
	return nil
}

func (c *stubContext) Inline(string, string) error {
	return nil
}

func (c *stubContext) NoContent(code int) error {
	if code >= 300 {
		return fmt.Errorf("%d", code)
	}
	return nil
}

func (c *stubContext) Redirect(code int, s string) error {
	if code >= 300 {
		return fmt.Errorf("%d u %s", code, s)
	}
	return nil
}

func (c *stubContext) Error(err error) {
	c.Result = err
}

func (c *stubContext) Echo() *echo.Echo {
	return nil
}

func (c *stubContext) Handler() echo.HandlerFunc {
	return nil
}

func (c *stubContext) SetHandler(echo.HandlerFunc) {
}

func (c *stubContext) Logger() echo.Logger {
	return nil
}

func (c *stubContext) Reset(*http.Request, http.ResponseWriter) {
}
