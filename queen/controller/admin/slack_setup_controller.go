// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

// slackSetupController serves /dashboard/slack/setup.
// Generates single-use, time-limited registration codes so users can register
// with the Slack bot without ever DMing a raw API token.
type slackSetupController struct {
	codeRepo  repository.SlackRegCodeRepository
	webserver web.Server
}

// NewSlackSetupController registers dashboard routes for Slack setup.
func NewSlackSetupController(
	codeRepo repository.SlackRegCodeRepository,
	webserver web.Server,
) *slackSetupController {
	ctr := &slackSetupController{codeRepo: codeRepo, webserver: webserver}
	webserver.GET("/dashboard/slack/setup", ctr.showSetup, acl.NewPermission(acl.Dashboard, acl.View))
	webserver.POST("/dashboard/slack/setup/generate", ctr.generateCode, acl.NewPermission(acl.Dashboard, acl.View))
	return ctr
}

// showSetup renders the setup page, generating a fresh code each time.
func (ctr *slackSetupController) showSetup(c web.APIContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return &echo.HTTPError{Code: http.StatusUnauthorized, Message: "not logged in"}
	}
	code, err := ctr.newCode(user)
	if err != nil {
		return err
	}
	res := map[string]interface{}{
		"Code":    code.Code,
		"Expires": code.ExpiresAt.Format("15:04:05 MST"),
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "slack/setup", res)
}

// generateCode creates a new code and redirects to the setup page (PRG pattern).
func (ctr *slackSetupController) generateCode(c web.APIContext) error {
	user := web.GetDBLoggedUserFromSession(c)
	if user == nil {
		return &echo.HTTPError{Code: http.StatusUnauthorized, Message: "not logged in"}
	}
	if _, err := ctr.newCode(user); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/dashboard/slack/setup")
}

// newCode generates a cryptographically random 32-byte (64 hex char) code,
// stores it in the DB with a 15-minute TTL, and returns it.
func (ctr *slackSetupController) newCode(user *common.User) (*common.SlackRegCode, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	qc := common.NewQueryContextFromIDs(user.ID, user.OrganizationID)
	code := &common.SlackRegCode{
		Code:      hex.EncodeToString(raw),
		UserID:    user.ID,
		OrgID:     user.OrganizationID,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := ctr.codeRepo.Create(qc, code); err != nil {
		return nil, err
	}
	return code, nil
}
