package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// NotifyChannel for notification
type NotifyChannel string

const (
	// EmailChannel send via email
	EmailChannel NotifyChannel = "email"
	// SlackChannel send via slack
	SlackChannel NotifyChannel = "slack"
)

// NotifyWhen type alias for when notify should be used
type NotifyWhen string

const (
	// NotifyWhenOnSuccess on-success
	NotifyWhenOnSuccess NotifyWhen = "onSuccess"
	// NotifyWhenOnFailure on-failure
	NotifyWhenOnFailure NotifyWhen = "onFailure"
	// NotifyWhenAlways default
	NotifyWhenAlways NotifyWhen = "always"
	// NotifyWhenNever no notification
	NotifyWhenNever NotifyWhen = "never"
)

// Accept returns true if notification can be sent based on state
func (w NotifyWhen) Accept(state RequestState, lastState RequestState) bool {
	if w == "" || w == NotifyWhenNever {
		return true
	}
	return w == NotifyWhenAlways ||
		(state.Completed() && w == NotifyWhenOnSuccess) ||
		(state.Failed() && w == NotifyWhenOnFailure) ||
		(state.Completed() && w == NotifyWhenOnFailure && lastState.Failed()) // completed after failure
}

// JobNotifyConfig structure for notification config
type JobNotifyConfig struct {
	Recipients []string   `yaml:"recipients" json:"recipients" gorm:"-"`
	When       NotifyWhen `json:"when,omitempty" yaml:"when,omitempty"`
}

// ValidateEmail validates email of recipients
func (c *JobNotifyConfig) ValidateEmail() error {
	for _, email := range c.Recipients {
		if email == "" {
			return errors.New("email is not specified")
		}
		re := regexp.MustCompile(emailRegex)
		if !re.MatchString(email) {
			return errors.New("email is not valid")
		}
	}
	return nil
}

// JobNotifyConfigWithEmail factory method to create notification config for emails
func JobNotifyConfigWithEmail(email string, when NotifyWhen) (JobNotifyConfig, error) {
	notifyCfg := JobNotifyConfig{
		When: when,
	}
	return notifyCfg, notifyCfg.SetEmails(email)
}

// JobNotifyConfigWithChannel factory method to create notification config for channels
func JobNotifyConfigWithChannel(channel string, when NotifyWhen) (JobNotifyConfig, error) {
	notifyCfg := JobNotifyConfig{
		When: when,
	}
	return notifyCfg, notifyCfg.SetChannels(channel)
}

// SetEmails - set emails of recipients
func (c *JobNotifyConfig) SetEmails(rawEmail string) error {
	emails := strings.Split(rawEmail, ",")
	c.Recipients = make([]string, 0)
	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		re := regexp.MustCompile(emailRegex)
		if !re.MatchString(email) {
			return fmt.Errorf("email '%s' is not valid", email)
		}
		c.Recipients = append(c.Recipients, email)
	}
	if len(c.Recipients) == 0 {
		return fmt.Errorf("email is not specified")
	}
	return nil
}

// SetChannels - set channel for recipients
func (c *JobNotifyConfig) SetChannels(rawChannel string) error {
	channels := strings.Split(rawChannel, ",")
	c.Recipients = make([]string, 0)
	for _, channel := range channels {
		channel = strings.TrimSpace(strings.ToLower(channel))
		if channel == "" {
			continue
		}
		c.Recipients = append(c.Recipients, channel)
	}
	if len(c.Recipients) == 0 {
		return fmt.Errorf("channel is not specified")
	}
	return nil
}
