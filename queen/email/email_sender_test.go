package email

import (
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"testing"
)

func Test_ShouldSendEmail(t *testing.T) {
	qc := common.NewQueryContext(nil, "").WithAdmin()
	serverCfg := config.TestServerConfig()
	userMgr := manager.AssertTestUserManager(serverCfg, t)
	require.NoError(t, serverCfg.SMTP.Validate())
	sender, err := New(serverCfg, userMgr)
	require.NoError(t, err)
	err = sender.SendMessage(
		qc,
		nil,
		[]string{"support@formicary.io"},
		"my email",
		"Test email",
		make(map[string]interface{}))
	require.Error(t, err) // Should fail due to missing setup
}
