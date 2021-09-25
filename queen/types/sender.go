package types

import (
	common "plexobject.com/formicary/internal/types"
)

// Sender defines operations to send message
type Sender interface {
	SendMessage(
		qc *common.QueryContext,
		user *common.User,
		to []string,
		subject string,
		body string,
		opts map[string]interface{}) error
	JobNotifyTemplateFile() string
}

// SlackToken token
const SlackToken = "SlackToken"

const (
	// Color option
	Color = "Color"
	// Link option
	Link = "Link"
	// Emoji option
	Emoji = "Emoji"
	// Thread option
	Thread = "Thread"
)
