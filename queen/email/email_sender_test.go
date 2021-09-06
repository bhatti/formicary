package email

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"testing"
)

func Test_ShouldSendEmail(t *testing.T) {
	qc := common.NewQueryContext("", "", "").WithAdmin()
	serverCfg := config.TestServerConfig()
	userMgr := manager.AssertTestUserManager(serverCfg, t)
	if err := serverCfg.Email.Validate(); err != nil {
		t.Logf("skip sending email because smtp is not setup - %s", err)
		return
	}
	sender, err := New(serverCfg, userMgr)
	require.NoError(t, err)
	err = sender.SendMessage(
		qc,
		nil,
		nil,
		[]string{"support@formicary.io"},
		"my email",
		"Test email",
		make(map[string]interface{}))
	require.NoError(t, err)
}
