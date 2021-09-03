package slack

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
	"strings"
)

const slackToken = "SlackToken"

// DefaultSlackSender defines operations to send slack
type DefaultSlackSender struct {
	cfg *config.ServerConfig
}

// New constructor
func New(cfg *config.ServerConfig) (types.Sender, error) {
	return &DefaultSlackSender{cfg: cfg}, nil
}

// SendMessage sends slack to recipients
func (d *DefaultSlackSender) SendMessage(
	ctx context.Context,
	_ *common.User,
	org *common.Organization,
	to []string,
	subject string,
	body string) (err error) {
	var token string
	if org != nil {
		token = org.GetConfigString(slackToken)
	}
	if token == "" && ctx.Value(slackToken) != nil {
		token = ctx.Value(slackToken).(string)
	}
	if token == "" {
		return fmt.Errorf("SlackToken is not found in organization config")
	}

	api := slack.New(strings.TrimSpace(token))
	//attachment := slack.Attachment{
	//	Pretext: subject,
	//	Text:    body,
	//}
	for _, recipient := range to {
		_, _, err = api.PostMessage(
			recipient,
			slack.MsgOptionText(body, false),
			//slack.MsgOptionAttachments(attachment),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			return fmt.Errorf("failed to send message to slack: %s", err)
		}
	}
	logrus.WithFields(logrus.Fields{
		"Component":             "DefaultSlackSender",
		"Subject":               subject,
		"To":                    to,
		"JobNotifyTemplateFile": d.JobNotifyTemplateFile(),
		"Size":                  len(body),
	}).Infof("sending slack message")

	return nil
}

// JobNotifyTemplateFile template file
func (d *DefaultSlackSender) JobNotifyTemplateFile() string {
	return d.cfg.Notify.SlackJobsTemplateFile
}
