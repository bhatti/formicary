package manager

import (
	"fmt"
	"plexobject.com/formicary/queen/config"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldAddStickyMessage(t *testing.T) {
	serverCfg := config.TestServerConfig()
	userMgr, err := TestUserManager(serverCfg)
	require.NoError(t, err)
	user := common.NewUser("", "username", "name", "email@formicary.io", false)
	qc := common.NewQueryContext("", "", "").WithAdmin()
	user, err = userMgr.CreateUser(qc, user)
	require.NoError(t, err)
	err = userMgr.AddStickyMessageForSlack(qc, user, nil, fmt.Errorf("failed"))
	require.NoError(t, err)
	err = userMgr.ClearStickyMessageForSlack(qc, user, nil)
	require.NoError(t, err)
}
