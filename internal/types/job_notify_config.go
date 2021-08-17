package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
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

func (w NotifyWhen) Accept(state RequestState) bool {
	if w == "" {
		return true
	}
	return w == NotifyWhenAlways ||
		(state.Completed() && w == NotifyWhenOnSuccess) ||
		(state.Failed() && w == NotifyWhenOnFailure)
}

type JobNotifyConfig struct {
	Recipients []string   `yaml:"recipients" json:"recipients" gorm:"-"`
	When       NotifyWhen `json:"when,omitempty" yaml:"when,omitempty"`
}

func (c JobNotifyConfig) ValidateEmail() error {
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

func JobNotifyConfigWithEmail(email string, when NotifyWhen) (JobNotifyConfig, error) {
	notifyCfg := JobNotifyConfig{
		When: when,
	}
	return notifyCfg, notifyCfg.SetEmails(email)
}

func (c *JobNotifyConfig) SetEmails(rawEmail string) error {
	emails := strings.Split(rawEmail, ",")
	c.Recipients = make([]string, 0)
	for _, email := range emails {
		email = strings.TrimSpace(email)
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
