package admin

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4/middleware"

	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/auth"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/security"
)

// AuthController structure
type AuthController struct {
	commonCfg             *common.CommonConfig
	authProviders         map[string]auth.Provider
	auditRecordRepository repository.AuditRecordRepository
	userRepository        repository.UserRepository
	orgRepository         repository.OrganizationRepository
	webserver             web.Server
}

// NewAuthController for user login/logout
func NewAuthController(
	commonCfg *common.CommonConfig,
	authProviders []auth.Provider,
	auditRecordRepository repository.AuditRecordRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	webServer web.Server) *AuthController {
	ac := &AuthController{
		commonCfg:             commonCfg,
		authProviders:         make(map[string]auth.Provider),
		auditRecordRepository: auditRecordRepository,
		userRepository:        userRepository,
		orgRepository:         orgRepository,
		webserver:             webServer,
	}
	webServer.GET("/", ac.root, nil).Name = "root"
	webServer.GET("/login", ac.login, acl.NewPermission(acl.User, acl.Login)).Name = "login"
	webServer.GET("/logout", ac.logout, acl.NewPermission(acl.User, acl.Logout)).Name = "logout"
	for _, authProvider := range authProviders {
		ac.authProviders[authProvider.AuthLoginURL()] = authProvider
		ac.authProviders[authProvider.AuthLoginCallbackURL()] = authProvider
		webServer.GET(
			authProvider.AuthLoginURL(),
			ac.providerAuth,
			acl.NewPermission(acl.User, acl.Login)).Name = "provider_auth"
		webServer.GET(
			authProvider.AuthLoginCallbackURL(),
			ac.providerAuthCallback,
			acl.NewPermission(acl.User, acl.Login)).Name = "provider_auth_callback"
		if authProvider.AuthWebhookCallbackURL() != "" {
			apiConfig := middleware.JWTConfig{
				Claims:      &web.JwtClaims{},
				SigningKey:  []byte(commonCfg.Auth.JWTSecret),
				TokenLookup: "query:authorization",
			}

			webServer.POST(
				authProvider.AuthWebhookCallbackURL(),
				authProvider.AuthWebhookCallbackHandle,
				acl.NewPermission(acl.User, acl.Login),
				middleware.JWTWithConfig(apiConfig)).Name = "provider_auth_webhook_callback"
		}
	}

	if commonCfg.Auth.Enabled {
		ac.addSessionMiddleware(webServer)
	} else {
		ac.nonSessionMiddleware(webServer)
	}
	return ac
}

// ********************************* HTTP Handlers ***********************************

// root - authentication
func (ac *AuthController) root(c web.WebContext) error {
	return c.JSON(http.StatusOK, ac.commonCfg.Version)
}

// providerAuthCallback - authentication
func (ac *AuthController) providerAuthCallback(c web.WebContext) error {
	stateCookie, err := c.Cookie(ac.commonCfg.Auth.LoginStateCookieName())
	if err != nil || stateCookie.Value == "" {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Error":     err,
		}).Warnf("no state cookie configured")
		http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
		return nil
	}

	authProvider := ac.authProviders[c.Path()]
	if authProvider == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Path":      c.Path(),
			"Providers": ac.authProviders,
		}).Error("unsupported login provider callback")
		http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
		return nil
	}

	user, err := authProvider.AuthUser(stateCookie.Value, c)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Error":     err,
		}).Error("failed to handle callback for login")
		http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
		return err
	}
	cookie, oldUser, err := ac.postLogin(user)
	if err != nil {
		return err
	}

	if cookie != nil {
		c.SetCookie(cookie)
		c.SetCookie(ac.commonCfg.Auth.ExpiredCookie(ac.commonCfg.Auth.LoginStateCookieName()))
	}
	session := types.NewUserSession(user, stateCookie.Value)
	session.IPAddress = c.Request().RemoteAddr
	_ = ac.userRepository.AddSession(session)

	if oldUser != nil {
		_, _ = ac.auditRecordRepository.Save(types.NewAuditRecordFromUser(oldUser, types.UserLogin,
			common.NewQueryContext(oldUser, c.Request().RemoteAddr)))
		if redirectCookie, err := c.Cookie(common.RedirectCookieName); err == nil && redirectCookie.Value != "" &&
			!strings.HasPrefix(redirectCookie.Value, "/dashboard/users/new") {
			c.SetCookie(ac.commonCfg.Auth.ClearRedirectCookie())
			http.Redirect(c.Response(), c.Request(), redirectCookie.Value, http.StatusTemporaryRedirect)
		} else {
			http.Redirect(c.Response(), c.Request(), "/dashboard", http.StatusTemporaryRedirect)
		}
	}

	if redirectCookie, err := c.Cookie(common.RedirectCookieName); err == nil &&
		strings.HasPrefix(redirectCookie.Value, "/dashboard/users/new") {
		c.SetCookie(ac.commonCfg.Auth.ClearRedirectCookie())
		http.Redirect(c.Response(), c.Request(), redirectCookie.Value, http.StatusTemporaryRedirect)
	} else {
		http.Redirect(c.Response(), c.Request(), "/dashboard/users/new", http.StatusTemporaryRedirect)
	}

	return nil
}

// providerAuth - authentication
func (ac *AuthController) providerAuth(c web.WebContext) error {
	stateCookie, err := c.Cookie(ac.commonCfg.Auth.LoginStateCookieName())
	if err != nil || stateCookie.Value == "" {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Error":     err,
		}).Warnf("no state cookie configured")
		http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
		return nil
	}

	authProvider := ac.authProviders[c.Path()]
	if authProvider == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Path":      c.Path(),
			"Providers": ac.authProviders,
		}).Error("unsupported login provider")
		http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
		return nil
	}

	url := authProvider.AuthHandler(stateCookie.Value)
	http.Redirect(c.Response(), c.Request(), url, http.StatusTemporaryRedirect)
	return nil
}

// login - authentication
func (ac *AuthController) login(c web.WebContext) error {
	c.SetCookie(ac.commonCfg.Auth.LoginStateCookie())
	res := map[string]interface{}{
		"HasGoogleOAuth": ac.commonCfg.Auth.HasGoogleOAuth(),
		"HasGithubOAuth": ac.commonCfg.Auth.HasGithubOAuth(),
	}
	return c.Render(http.StatusOK, "users/login", res)
}

// logout - logout
func (ac *AuthController) logout(c web.WebContext) error {
	qc := web.BuildQueryContext(c)
	user := web.GetDBLoggedUserFromSession(c)
	c.SetCookie(ac.commonCfg.Auth.ExpiredCookie(ac.commonCfg.Auth.CookieName))
	c.SetCookie(ac.commonCfg.Auth.ExpiredCookie(ac.commonCfg.Auth.LoginStateCookieName()))
	c.SetCookie(ac.commonCfg.Auth.ExpiredCookie(common.RedirectCookieName))
	c.SetCookie(ac.commonCfg.Auth.ExpiredCookie("JSESSIONID"))
	if user != nil {
		_, _ = ac.auditRecordRepository.Save(types.NewAuditRecordFromUser(user, types.UserLogout, qc))
	}
	http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
	return nil
}

func (ac *AuthController) postLogin(user *common.User) (cookie *http.Cookie, oldUser *common.User, err error) {
	// not using query-context here because we just need to find user
	oldUser, _ = ac.userRepository.GetByUsername(
		common.NewQueryContext(nil, ""),
		user.Username)
	if oldUser != nil {
		user.CopyRolesPermissions(oldUser)
	}
	token, expiration, err := security.BuildToken(
		user,
		ac.commonCfg.Auth.JWTSecret,
		ac.commonCfg.Auth.MaxAge)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "Authentication",
			"ID":        ac.commonCfg.ID,
			"Error":     err,
			"User":      user,
		}).Error("failed to create token")
		return nil, nil, err
	}
	cookie = ac.commonCfg.Auth.SessionCookie(token, expiration)
	return cookie, oldUser, nil
}

func (ac *AuthController) nonSessionMiddleware(webServer web.Server) {
	webServer.AddMiddleware(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(web.AuthDisabled, true)
			c.Set(web.AppVersion, ac.commonCfg.Version.String())
			return next(c)
		}
	})
}

func (ac *AuthController) addSessionMiddleware(webServer web.Server) {
	webServer.AddMiddleware(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(web.AppVersion, ac.commonCfg.Version.String())
			user, dbUser, _, err := ac.addSessionUser(c)
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "AuthController",
					"Error":     err,
					"DBUser":    dbUser,
					"Path":      c.Path(),
				}).Debugf("session middleware")
			}

			if dbUser == nil {
				if web.IsWhiteListURL(c.Path(), c.Request().Method) ||
					web.IsWhiteListURL(c.Request().RequestURI, c.Request().Method) {
					// ok
				} else if strings.HasPrefix(c.Path(), "/dashboard") {
					http.Redirect(c.Response(), c.Request(), "/dashboard/users/new", http.StatusTemporaryRedirect)
					return nil
				} else {
					return &echo.HTTPError{
						Code: http.StatusUnauthorized,
						Message: fmt.Sprintf("JWT token is required for api access %s:%s (001)",
							c.Request().Method, c.Request().RequestURI),
					}
				}
			} else if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "Authentication",
					"ID":        ac.commonCfg.ID,
					"Error":     err,
					"User":      user,
				}).Error("failed to add session")
				if strings.HasPrefix(c.Path(), "/dashboard") {
					http.Redirect(c.Response(), c.Request(), "/login", http.StatusTemporaryRedirect)
					return nil
				}
				return &echo.HTTPError{
					Code:     http.StatusUnauthorized,
					Message:  "JWT token is required for api access (002)",
					Internal: err,
				}
			} else if dbUser.Locked {
				logrus.WithFields(logrus.Fields{
					"Component": "AuthController",
					"Error":     err,
					"DBUser":    dbUser,
					"Locked":    dbUser.Locked,
					"Path":      c.Path(),
				}).Warnf("user is locked")
				return &echo.HTTPError{
					Code:    http.StatusUnauthorized,
					Message: "user is locked (005)",
				}
			}
			return next(c)
		}
	})
}

func (ac *AuthController) addSessionUser(c web.WebContext) (
	user *common.User,
	dbUser *common.User,
	claims *web.JwtClaims, err error) {
	if ac.commonCfg.BlockUserAgent(c.Request().UserAgent()) {
		return nil, nil, nil, &echo.HTTPError{
			Code:    http.StatusUnauthorized,
			Message: fmt.Sprintf("unauthorized agent session for: %s %s", c.Request().Method, c.Path()),
		}
	}

	if !ac.commonCfg.Auth.Enabled {
		c.Set(web.AuthDisabled, true)
		return nil, nil, nil, nil
	}
	authCookie, err := c.Cookie(ac.commonCfg.Auth.CookieName)
	var token string
	if err != nil {
		tokenString := c.Request().Header.Get("Authorization")
		if tokenString == "" {
			tokenString = c.QueryParam("authorization")
		}
		if tokenString == "" {
			logrus.WithFields(logrus.Fields{
				"Component": "AuthController",
				"URL":       c.Request().URL,
				"Error":     err,
				"Headers":   c.Request().Header,
			}).Warnf("failed to find authorization in header")
			return nil, nil, nil, fmt.Errorf("could not find token")
		}
		token, err = stripTokenPrefix(tokenString)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "AuthController",
				"Error":     err,
			}).Warnf("addSessionUser failed to strip token")
			return nil, nil, nil, err
		}
	} else {
		token = authCookie.Value
	}

	claims, err = security.ParseToken(token, ac.commonCfg.Auth.JWTSecret)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Token":     token,
			"Error":     err,
		}).Warnf("addSessionUser failed to parse token")
		return nil, nil, nil, fmt.Errorf("failed to find session claims %v", err)
	}

	user = common.NewUser(
		claims.OrgID,
		claims.UserName,
		"",
		"",
		acl.NewRoles(""),
	)

	user.Name = claims.Name
	user.BundleID = claims.BundleID
	user.PictureURL = claims.PictureURL
	user.AuthProvider = claims.AuthProvider
	// not using query-context here because we just need to find user
	dbUser, err = ac.userRepository.GetByUsername(common.NewQueryContext(nil, ""), user.Username)
	if err != nil {
		// can't delete cookie
		//c.SetCookie(ac.commonCfg.Auth.ExpiredCookie(ac.commonCfg.Auth.CookieName))
	} else {
		if dbUser.OrganizationID != "" && dbUser.Organization == nil {
			dbUser.Organization, _ = ac.orgRepository.Get(
				common.NewQueryContext(nil, ""),
				dbUser.OrganizationID)
		}

		if dbUser.Organization != nil {
			dbUser.OrgUnit = dbUser.Organization.OrgUnit
			dbUser.BundleID = dbUser.Organization.BundleID
			dbUser.Subscription = dbUser.Organization.Subscription
			user.OrgUnit = dbUser.Organization.OrgUnit
			user.BundleID = dbUser.Organization.BundleID
			user.Salt = dbUser.Organization.Salt
			user.OrganizationID = dbUser.Organization.ID
			user.PictureURL = dbUser.PictureURL
			user.AuthProvider = dbUser.AuthProvider
			user.Subscription = dbUser.Organization.Subscription
		}

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":    "AuthController",
				"User":         dbUser,
				"Org":          dbUser.OrganizationID,
				"Subscription": dbUser.Subscription,
			}).Debugf("loaded db user for session")
		}
		c.Set(web.DBUser, dbUser)
	}
	c.Set(web.LoggedInUser, user)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "AuthController",
			"Headers":   c.Request().Header,
			"User":      dbUser,
		}).Debugf("added user to session")
	}
	return
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
