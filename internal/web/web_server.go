package web

import (
	"fmt"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"math"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/types"
	"strings"
	"time"
)

// HandlerFunc defines a function to serve HTTP requests.
type HandlerFunc func(WebContext) error

// WrapHandler wraps `http.Handler` into `echo.HandlerFunc`.
func WrapHandler(h http.Handler) HandlerFunc {
	return func(c WebContext) error {
		h.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

// Server defines methods for binding http methods
type Server interface {
	GET(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route
	AddMiddleware(m echo.MiddlewareFunc)
	Start(address string)
	Stop()
}

// DefaultWebServer defines default web server
type DefaultWebServer struct {
	e              *echo.Echo
	apiGroup       *echo.Group
	dashboardGroup *echo.Group
	authEnabled    bool
}

// NewDefaultWebServer creates new instance of web server
func NewDefaultWebServer(commonCfg *types.CommonConfig) (Server, error) {
	ws := &DefaultWebServer{e: echo.New(), authEnabled: commonCfg.Auth.Enabled}
	ws.e.Static("/", "public/assets")
	ws.e.Static("/docs", "public/docs")
	ws.e.File("/favicon.ico", "public/assets/images/favicon.ico") // https://favicon.io/emoji-favicons/sparkle
	defaultLoggerConfig := middleware.LoggerConfig{
		Skipper: middleware.DefaultSkipper,
		Format: `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
	}
	ws.e.Use(middleware.LoggerWithConfig(defaultLoggerConfig))
	ws.e.Use(middleware.Recover())
	ws.e.HTTPErrorHandler = func(err error, c echo.Context) {
		err = mapAPIErrors(err)
		ws.e.DefaultHTTPErrorHandler(err, c)
	}
	if commonCfg.Auth.Enabled {
		// TODO add check for revoked token **** MUST ****
		apiConfig := middleware.JWTConfig{
			Claims:       &JwtClaims{},
			SigningKey:   []byte(commonCfg.Auth.JWTSecret),
			TokenLookup:  "header:Authorization",
			ErrorHandler: mapAuthErrors,
		}

		dashboardConfig := middleware.JWTConfig{
			Claims:      &JwtClaims{},
			SigningKey:  []byte(commonCfg.Auth.JWTSecret),
			TokenLookup: "cookie:" + commonCfg.Auth.CookieName,
			ErrorHandlerWithContext: func(err error, c echo.Context) error {
				// Redirects to the login form.
				authCookie, _ := c.Cookie(commonCfg.Auth.CookieName)
				logrus.WithFields(logrus.Fields{
					"Component":  "DefaultWebServer",
					"AuthCookie": authCookie,
				}).Warn("redirecting to login")
				return c.Redirect(http.StatusTemporaryRedirect, "/login")
			},
		}

		ws.apiGroup = ws.e.Group("/api")
		ws.apiGroup.Use(middleware.JWTWithConfig(apiConfig))
		ws.apiGroup.Use(rateLimitMiddleware(commonCfg.RateLimitPerSecond))

		ws.dashboardGroup = ws.e.Group("/dashboard")
		ws.dashboardGroup.Use(middleware.JWTWithConfig(dashboardConfig))
		ws.dashboardGroup.Use(rateLimitMiddleware(commonCfg.RateLimitPerSecond * 2))
	}

	//CORS
	ws.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		//AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))
	var err error
	ws.e.Renderer, err = NewTemplateRenderer("public/views", ".html", commonCfg.Development)
	if err != nil {
		return nil, err
	}
	ws.e.HideBanner = true
	return ws, nil
}

// AddMiddleware adds middleware
func (w *DefaultWebServer) AddMiddleware(m echo.MiddlewareFunc) {
	w.e.Use(m)
}

func (w *DefaultWebServer) checkPermission(c echo.Context, perm *acl.Permission) error {
	if perm == nil || !w.authEnabled {
		return nil
	}
	if IsWhiteListURL(c.Path(), c.Request().Method) {
		return nil
	}
	user := GetDBUserFromSession(c)
	if user == nil {
		return &echo.HTTPError{
			Code:    http.StatusUnauthorized,
			Message: fmt.Sprintf("authentication required for accessing %s %s", c.Request().Method, c.Path()),
		}
	}
	if !user.HasPermission(perm.Resource, perm.Actions) {
		return &echo.HTTPError{
			Code:    http.StatusUnauthorized,
			Message: fmt.Sprintf("permission '%s' required for accessing %s %s", perm.LongString(), c.Request().Method, c.Path()),
		}
	}
	return nil
}

// GET calls HTTP GET method
func (w *DefaultWebServer) GET(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route {
	if w.apiGroup != nil && strings.HasPrefix(path, "/api") {
		return w.apiGroup.GET(path[4:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else if w.dashboardGroup != nil && strings.HasPrefix(path, "/dashboard") {
		return w.dashboardGroup.GET(path[10:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else {
		return w.e.GET(path, func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	}
}

// POST calls HTTP POST method
func (w *DefaultWebServer) POST(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route {
	if w.apiGroup != nil && strings.HasPrefix(path, "/api") {
		return w.apiGroup.POST(path[4:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else if w.dashboardGroup != nil && strings.HasPrefix(path, "/dashboard") {
		return w.dashboardGroup.POST(path[10:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else {
		return w.e.POST(path, func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	}
}

// PUT calls HTTP PUT method
func (w *DefaultWebServer) PUT(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route {
	if w.apiGroup != nil && strings.HasPrefix(path, "/api") {
		return w.apiGroup.PUT(path[4:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else if w.dashboardGroup != nil && strings.HasPrefix(path, "/dashboard") {
		return w.dashboardGroup.PUT(path[10:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else {
		return w.e.PUT(path, func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	}
}

// DELETE calls HTTP DELETE method
func (w *DefaultWebServer) DELETE(path string, h HandlerFunc, perm *acl.Permission, m ...echo.MiddlewareFunc) *echo.Route {
	if w.apiGroup != nil && strings.HasPrefix(path, "/api") {
		return w.apiGroup.DELETE(path[4:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else if w.dashboardGroup != nil && strings.HasPrefix(path, "/dashboard") {
		return w.dashboardGroup.DELETE(path[10:], func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	} else {
		return w.e.DELETE(path, func(context echo.Context) error {
			if err := w.checkPermission(context, perm); err != nil {
				return err
			}
			return h(context)
		}, m...)
	}
}

// Start - starts web server
func (w *DefaultWebServer) Start(address string) {
	w.e.Logger.Fatal(w.e.Start(address))
}

// Stop - stops web server
func (w *DefaultWebServer) Stop() {
	_ = w.e.Close()
}

func rateLimitMiddleware(rate float64) echo.MiddlewareFunc {
	// every token bucket in it will expire 1 hour after it was initially set.
	rate = math.Max(rate, 1)
	lmt := tollbooth.NewLimiter(
		rate,
		&limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	lmt.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
	lmt.SetMethods([]string{"GET", "POST", "PUT", "DELETE"})
	lmt.SetMessage(fmt.Sprintf("You have reached maximum request limit (%v).", rate))
	lmt.SetMessageContentType("text/plain; charset=utf-8")
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httpError := tollbooth.LimitByRequest(lmt, c.Response(), c.Request())
			if httpError != nil {
				return c.String(httpError.StatusCode, httpError.Message)
			}
			return next(c)
		}
	}
}

func mapAuthErrors(err error) error {
	switch err.(type) {
	case *echo.HTTPError:
		herr := err.(*echo.HTTPError)
		//debug.PrintStack()
		logrus.WithFields(logrus.Fields{
			"Component": "DefaultWebServer",
			"Error":     err,
		}).Warn("failed to authenticate api")
		return herr
	case *types.NotFoundError:
		return &echo.HTTPError{
			Code:     http.StatusNotFound,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.ConflictError:
		return &echo.HTTPError{
			Code:     http.StatusConflict,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.DuplicateError:
		return &echo.HTTPError{
			Code:     http.StatusConflict,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.ValidationError:
		return &echo.HTTPError{
			Code:     http.StatusPreconditionFailed,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.PermissionError:
		return &echo.HTTPError{
			Code:     http.StatusForbidden,
			Message:  err.Error(),
			Internal: err,
		}
	default:
		return &echo.HTTPError{
			Code:     http.StatusForbidden,
			Message:  err.Error() + " (003)",
			Internal: err,
		}
	}
}

func mapAPIErrors(err error) error {
	switch err.(type) {
	case *echo.HTTPError:
		herr := err.(*echo.HTTPError)
		//debug.PrintStack()
		logrus.WithFields(logrus.Fields{
			"Component": "DefaultWebServer",
			"Error":     err,
		}).Warn("failed to authenticate api")
		return herr
	case *types.NotFoundError:
		return &echo.HTTPError{
			Code:     http.StatusNotFound,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.ConflictError:
		return &echo.HTTPError{
			Code:     http.StatusConflict,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.DuplicateError:
		return &echo.HTTPError{
			Code:     http.StatusConflict,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.ValidationError:
		return &echo.HTTPError{
			Code:     http.StatusPreconditionFailed,
			Message:  err.Error(),
			Internal: err,
		}
	case *types.PermissionError:
		return &echo.HTTPError{
			Code:     http.StatusForbidden,
			Message:  err.Error(),
			Internal: err,
		}
	default:
		return &echo.HTTPError{
			Code:     http.StatusForbidden,
			Message:  err.Error() + " (003)",
			Internal: err,
		}
	}
}
