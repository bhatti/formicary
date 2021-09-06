package slack

import (
	"github.com/stretchr/testify/require"
	"os"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"testing"
)

func Test_ShouldSendSlackMessage(t *testing.T) {
	qc := common.NewQueryContext("", "", "").WithAdmin()
	serverCfg := config.TestServerConfig()
	userMgr, err := manager.TestUserManager(serverCfg)
	require.NoError(t, err)
	sender, err := New(serverCfg, userMgr)
	require.NoError(t, err)
	org := common.NewOrganization("owner", "unit", "bundle")
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		t.Logf("skip sending slack because token is not defined")
		return
	}
	_, _ = org.AddConfig(types.SlackToken, token, true)
	err = sender.SendMessage(
		qc,
		nil,
		org,
		[]string{"#formicary"},
		"Job FAILED",
		`
Message here
`,
		map[string]interface{}{types.Color: "#dc3545", types.Link: "https://formicary.io"})
	require.NoError(t, err)
}
