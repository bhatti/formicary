package types

import (
	"context"
	common "plexobject.com/formicary/internal/types"
)

// Sender defines operations to send message
type Sender interface {
	SendMessage(
		ctx context.Context,
		user *common.User,
		org *common.Organization,
		to []string,
		subject string,
		body string) error
	JobNotifyTemplateFile() string
}