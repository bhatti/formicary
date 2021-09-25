package manager

import (
	"fmt"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/queen/config"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldAddStickyMessage(t *testing.T) {
	serverCfg := config.TestServerConfig()
	userMgr, err := TestUserManager(serverCfg)
	require.NoError(t, err)
	user := common.NewUser("", "username", "name", "email@formicary.io", acl.NewRoles(""))
	qc := common.NewQueryContext(nil, "").WithAdmin()
	user, err = userMgr.CreateUser(qc, user)
	require.NoError(t, err)
	err = userMgr.AddStickyMessageForSlack(qc, user, fmt.Errorf("failed"))
	require.NoError(t, err)
	err = userMgr.ClearStickyMessageForSlack(qc, user)
	require.NoError(t, err)
}
