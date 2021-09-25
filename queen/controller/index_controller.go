package controller

import (
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
)

// IndexController structure
type IndexController struct {
	webserver web.Server
}

// NewIndexController controller
func NewIndexController(
	webserver web.Server) *IndexController {
	ctr := &IndexController{
		webserver: webserver,
	}
	webserver.GET("/terms_service", ctr.terms, acl.NewPermission(acl.TermsService, acl.View)).Name = "terms_service"
	webserver.GET("/privacy_policies", ctr.privacy, acl.NewPermission(acl.TermsService, acl.View)).Name = "privacy_policies"

	return ctr
}

// ********************************* HTTP Handlers ***********************************
func (ctr *IndexController) terms(c web.WebContext) error {
	res := make(map[string]interface{})
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "terms_service", res)
}

func (ctr *IndexController) privacy(c web.WebContext) error {
	res := make(map[string]interface{})
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "privacy_policies", res)
}
