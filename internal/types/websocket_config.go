package types

import "time"

// WebSocketConfig holds configuration for the WebSocket messaging provider
type WebSocketConfig struct {
	// ServerEndpoint is the queen's WebSocket URL for ants to connect to.
	// If empty, this node acts as the server (queen mode).
	// Example: "ws://queen.example.com:7770/ws/queue"
	ServerEndpoint string `json:"server_endpoint,omitempty" yaml:"server_endpoint" mapstructure:"server_endpoint"`

	// Path is the HTTP path for the WebSocket endpoint on the queen.
	// Default: "/ws/queue"
	Path string `json:"path,omitempty" yaml:"path" mapstructure:"path"`

	// PingInterval is how often to send WebSocket ping frames to keep the connection alive.
	// Default: 10s
	PingInterval time.Duration `json:"ping_interval,omitempty" yaml:"ping_interval" mapstructure:"ping_interval"`

	// ReconnectMinDelay is the initial reconnect delay for ants after disconnection.
	// Default: 1s
	ReconnectMinDelay time.Duration `json:"reconnect_min_delay,omitempty" yaml:"reconnect_min_delay" mapstructure:"reconnect_min_delay"`

	// ReconnectMaxDelay is the maximum reconnect delay (exponential backoff cap).
	// Default: 30s
	ReconnectMaxDelay time.Duration `json:"reconnect_max_delay,omitempty" yaml:"reconnect_max_delay" mapstructure:"reconnect_max_delay"`

	// BufferDBPath is the path for the ant's SQLite offline buffer database.
	// Default: "./formicary-buffer.db"
	BufferDBPath string `json:"buffer_db_path,omitempty" yaml:"buffer_db_path" mapstructure:"buffer_db_path"`

	// WriteTimeout is the timeout for writing a message to the WebSocket connection.
	// Default: 10s
	WriteTimeout time.Duration `json:"write_timeout,omitempty" yaml:"write_timeout" mapstructure:"write_timeout"`

	// ReadBufferSize is the read buffer size for the WebSocket upgrader/dialer.
	// Default: 4096
	ReadBufferSize int `json:"read_buffer_size,omitempty" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`

	// WriteBufferSize is the write buffer size for the WebSocket upgrader/dialer.
	// Default: 4096
	WriteBufferSize int `json:"write_buffer_size,omitempty" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`

	// MaxMessageSize is the maximum allowed WebSocket message size in bytes.
	// Default: 1MB
	MaxMessageSize int64 `json:"max_message_size,omitempty" yaml:"max_message_size" mapstructure:"max_message_size"`

	// MaxBufferSize is the maximum number of messages to hold in the offline SQLite buffer.
	// Default: 10000
	MaxBufferSize int64 `json:"max_buffer_size,omitempty" yaml:"max_buffer_size" mapstructure:"max_buffer_size"`

	// BufferTTL is how long buffered (offline) messages are retained.
	// Default: 24h
	BufferTTL time.Duration `json:"buffer_ttl,omitempty" yaml:"buffer_ttl" mapstructure:"buffer_ttl"`
}

// Validate sets defaults on WebSocketConfig
func (c *WebSocketConfig) Validate() {
	if c.Path == "" {
		c.Path = "/ws/queue"
	}
	if c.PingInterval == 0 {
		c.PingInterval = 10 * time.Second
	}
	if c.ReconnectMinDelay == 0 {
		c.ReconnectMinDelay = 1 * time.Second
	}
	if c.ReconnectMaxDelay == 0 {
		c.ReconnectMaxDelay = 30 * time.Second
	}
	if c.BufferDBPath == "" {
		c.BufferDBPath = "./formicary-buffer.db"
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = 4096
	}
	if c.WriteBufferSize == 0 {
		c.WriteBufferSize = 4096
	}
	if c.MaxMessageSize == 0 {
		c.MaxMessageSize = 1024 * 1024 // 1MB
	}
	if c.MaxBufferSize == 0 {
		c.MaxBufferSize = 10000
	}
	if c.BufferTTL == 0 {
		c.BufferTTL = 24 * time.Hour
	}
}
