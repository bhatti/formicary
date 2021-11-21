package slack

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"jaytaylor.com/html2text"
	"os"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"testing"
)

func Test_ShouldFormatHTML(t *testing.T) {
	content, err := ioutil.ReadFile("../../sample-report.html")
	if err != nil {
		panic(err)
	}
	inputHTML := string(content)

	text, err := html2text.FromString(inputHTML, html2text.Options{PrettyTables: true})
	if err != nil {
		panic(err)
	}
	fmt.Println(text)
}

func Test_ShouldSendSlackMessage(t *testing.T) {
	qc, err := repository.NewTestQC()
	serverCfg := config.TestServerConfig()
	userMgr, err := manager.TestUserManager(serverCfg)
	require.NoError(t, err)
	sender, err := New(serverCfg, userMgr)
	require.NoError(t, err)
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		t.Logf("skip sending slack because token is not defined")
		return
	}
	_, _ = qc.User.Organization.AddConfig(types.SlackToken, token, true)
	err = sender.SendMessage(
		qc,
		qc.User,
		[]string{"#formicary"},
		"Job FAILED",
		`
Message here
`,
		map[string]interface{}{types.Color: "#dc3545", types.Link: "https://formicary.io"})
	require.NoError(t, err)
}
