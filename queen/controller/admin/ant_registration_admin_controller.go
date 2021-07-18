package admin

import (
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/queen/resource"

	"plexobject.com/formicary/internal/web"
)

// AntAdminController structure
type AntAdminController struct {
	resourceManager resource.Manager
	webserver       web.Server
}

// NewAntAdminController admin dashboard for managing system-ants
func NewAntAdminController(
	resourceManager resource.Manager,
	webserver web.Server) *AntAdminController {
	wac := &AntAdminController{
		resourceManager: resourceManager,
		webserver:       webserver,
	}
	webserver.GET("/dashboard/ants", wac.queryAnts, acl.New(acl.AntExecutor, acl.Query)).Name = "query_admin_ants"
	webserver.GET("/dashboard/ants/:id", wac.getAnt, acl.New(acl.AntExecutor, acl.View)).Name = "get_admin_ant"
	return wac
}

// ********************************* HTTP Handlers ***********************************
// queryAnts - queries ants registered with the server
func (wac *AntAdminController) queryAnts(c web.WebContext) error {
	ants := wac.resourceManager.Registrations()
	res := map[string]interface{}{"Ants": ants}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "ants/index", res)
}

// getAnt - finds ant by id
func (wac *AntAdminController) getAnt(c web.WebContext) error {
	id := c.Param("id")
	ant := wac.resourceManager.Registration(id)
	res := map[string]interface{}{}
	if ant != nil {
		res["Ant"] = ant
	} else {
		res["Error"] = "ant not found"
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "ants/view", res)
}
