package types

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// Webhook structure defines config options for callback webhook
type Webhook struct {
	URL     string            `yaml:"url" json:"url" gorm:"-"`
	Method  string            `yaml:"method" json:"method" gorm:"-"`
	Headers map[string]string `yaml:"headers" json:"headers" gorm:"-"`
	Query   map[string]string `yaml:"query" json:"query" gorm:"-"`
}

// NewWebhookFromString constructor
func NewWebhookFromString(val string) (*Webhook, error) {
	if val == "" {
		return nil, nil
	}
	var wh Webhook
	err := yaml.Unmarshal([]byte(val), &wh)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook from '%s' due to %s", val, err)
	}
	return &wh, nil
}

// NewWebhook constructor
func NewWebhook(val interface{}) (*Webhook, error) {
	if val == nil {
		return nil, nil
	}
	switch val.(type) {
	case string:
		return NewWebhookFromString(val.(string))
	default:
		b, err := yaml.Marshal(val)
		if err != nil {
			return nil, err
		}
		return NewWebhookFromString(string(b))
	}
}

// Serialize converts webhook into yaml
func (wh *Webhook) Serialize() (string, error) {
	b, err := yaml.Marshal(wh)
	if err != nil {
		return "", err
	}
	return string(b), err
}
