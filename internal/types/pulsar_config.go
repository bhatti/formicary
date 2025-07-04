package types

import (
	"time"
)

// PulsarConfig pulsar config
type PulsarConfig struct {
	ConnectionTimeout    time.Duration     `yaml:"connection_timeout" mapstructure:"connection_timeout"`
	ChannelBuffer        int               `yaml:"channel_buffer" mapstructure:"channel_buffer"`
	OAuth2               map[string]string `yaml:"oauth" mapstructure:"oauth"`
	MaxReconnectToBroker uint              `yaml:"max_reconnect_to_broker" mapstructure:"max_reconnect_to_broker"`
}

// Validate - validates
func (c *PulsarConfig) Validate() error {
	//if c.URL == "" {
	//	return errors.New("pulsar URL is not set")
	//}
	if c.ConnectionTimeout.Seconds() == 0 {
		c.ConnectionTimeout = 30 * time.Second
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
	return nil
}
