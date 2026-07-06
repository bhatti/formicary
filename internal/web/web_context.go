package web

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/acl"
	"strings"

	common "plexobject.com/formicary/internal/types"
)

// APIContext interface
type APIContext interface { //nolint
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

// AuthenticatedUser returns user
func AuthenticatedUser(c APIContext, cookieName string, secret string) (user *common.User, err error) {
	sessionUser := c.Get(DBUser)
	if sessionUser != nil {
		return sessionUser.(*common.User), nil
	}
	sessionUser = c.Get(LoggedInUser)
	if sessionUser != nil {
		return sessionUser.(*common.User), nil
	}
	token, err := AuthenticatedToken(c, cookieName)
	if err != nil {
		return nil, err
	}
	claims, err := parseToken(token, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to find session claims due to %w", err)
	}
	user = common.NewUser(
		claims.OrgID,
		claims.UserName,
		"",
		"",
		acl.NewRoles(claims.SerializedRoles),
	)

	user.ID = claims.UserID
	user.Name = claims.Name
	user.BundleID = claims.BundleID
	user.PictureURL = claims.PictureURL
	user.AuthProvider = claims.AuthProvider
	user.SerializedRoles = claims.SerializedRoles
	user.SerializedPerms = claims.SerializedPerms
	return user, nil
}

// AuthenticatedToken verifies token
func AuthenticatedToken(c APIContext, cookieName string) (token string, err error) {
	authCookie, err := c.Cookie(cookieName)
	if err != nil {
		tokenString := c.Request().Header.Get("Authorization")
		if tokenString == "" {
			tokenString = c.QueryParam("authorization")
		}
		if tokenString == "" {
			return "", fmt.Errorf("could not find jwt token in request headers '%s' or parameters 'authorization", cookieName)
		}
		token, err = stripTokenPrefix(tokenString)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "AuthController",
				"Error":     err,
			}).Warnf("addSessionUser failed to strip token")
			return "", err
		}
	} else {
		token = authCookie.Value
	}
	return
}

// RenderDBUserFromSession initializes user/admin parameters
func RenderDBUserFromSession(c APIContext, res map[string]interface{}) {
	user := GetDBLoggedUserFromSession(c)
	if c.Get(AppVersion) != nil {
		res[AppVersion] = c.Get(AppVersion)
	}
	if res["Q"] == nil {
		res["Q"] = ""
	}
	res["APIDocsURL"] = "https://petstore.swagger.io/?url=https://raw.githubusercontent.com/bhatti/formicary/main/public/docs/openapi.json"
	if user != nil {
		res[DBUser] = user
		res[DBUserOrg] = user.OrganizationID
		res["Admin"] = user.IsAdmin()
		res["ReadAdmin"] = user.IsReadAdmin()
		res["OrgAdmin"] = user.IsOrgAdmin()
	} else if c.Get(AuthDisabled) != nil {
		res[DBUserOrg] = ""
		res["Admin"] = true
		res["ReadAdmin"] = true
	}
}

// GetDBUserFromSession returns database-user from web context
func GetDBUserFromSession(c APIContext) *common.User {
	user := c.Get(DBUser)
	if user != nil {
		return user.(*common.User)
	}
	return nil
}

// GetDBLoggedUserFromSession returns logged-in user from web context
func GetDBLoggedUserFromSession(c APIContext) *common.User {
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
func BuildQueryContext(c APIContext) *common.QueryContext {
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

// ParseToken is the exported wrapper around parseToken for use outside this package.
func ParseToken(tokenString, secret string) (*JwtClaims, error) {
	return parseToken(tokenString, secret)
}

// parseToken parses a JWT and returns Claims object
func parseToken(tokenString string, secret string) (*JwtClaims, error) {
	claims := &JwtClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if c, ok := token.Claims.(*JwtClaims); ok {
		return c, nil
	}
	return nil, fmt.Errorf("unknown claims for token %v", token.Claims)
}

// Strips 'Token' or 'Bearer' prefix from token string
func stripTokenPrefix(tok string) (string, error) {
	// split token to 2 parts
	tokenParts := strings.Split(tok, " ")

	if len(tokenParts) < 2 {
		return tokenParts[0], nil
	}

	return tokenParts[1], nil
}
