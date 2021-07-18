package types

import (
	"errors"
	"time"
)

// PulsarConfig pulsar config
type PulsarConfig struct {
	URL                  string            `yaml:"url" mapstructure:"url"`
	ConnectionTimeout    time.Duration     `yaml:"connection_timeout" mapstructure:"connection_timeout"`
	ChannelBuffer        int               `yaml:"channel_buffer" mapstructure:"channel_buffer"`
	OAuth2               map[string]string `yaml:"oauth" mapstructure:"oauth"`
	TopicSuffix          string            `yaml:"topic_suffix" mapstructure:"topic_suffix"`
	TopicTenant          string            `yaml:"topic_tenant" mapstructure:"topic_tenant"`
	TopicNamespace       string            `yaml:"topic_namespace" mapstructure:"topic_namespace"`
	MaxReconnectToBroker uint              `yaml:"max_reconnect_to_broker" mapstructure:"max_reconnect_to_broker"`
}

// Validate - validates
func (c *PulsarConfig) Validate() error {
	if c.URL == "" {
		return errors.New("pulsar URL is not set")
	}
	if c.ChannelBuffer <= 0 {
		c.ChannelBuffer = 100
	}
	if c.ConnectionTimeout <= 0 {
		c.ConnectionTimeout = 30
	}
	if c.MaxReconnectToBroker <= 0 {
		c.MaxReconnectToBroker = 100
	}
	if c.TopicTenant == "" {
		c.TopicTenant = "public"
	}
	if c.TopicNamespace == "" {
		c.TopicNamespace = "default"
	}
	return nil
}
