package types

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"
)

// KafkaConfig kakfa config
type KafkaConfig struct {
	Brokers       []string      `yaml:"brokers"`
	Username      string        `yaml:"username"`
	Password      string        `yaml:"password"`
	CertFile      string        `yaml:"certificate"`
	KeyFile       string        `yaml:"key"`
	CAFile        string        `yaml:"ca"`
	VerifySSL     bool          `yaml:"verify"`
	UseTLS        bool          `yaml:"tls"`
	UseSasl       bool          `yaml:"sasl"`
	RetryMax      int           `yaml:"retry_max"`
	Group         string        `yaml:"group"`
	ChannelBuffer int           `yaml:"channel_buffer" mapstructure:"channel_buffer"`
	CommitTimeout time.Duration `yaml:"commit_timeout" mapstructure:"commit_timeout"`
	debug         bool
}

// BuildTLSConfiguration builds TLS config
func (c *KafkaConfig) BuildTLSConfiguration() (t *tls.Config, err error) {
	t = &tls.Config{
		InsecureSkipVerify: c.VerifySSL,
	}
	if c.CertFile != "" && c.KeyFile != "" && c.CAFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}

		caCert, err := ioutil.ReadFile(c.CAFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t.Certificates = []tls.Certificate{cert}
		t.RootCAs = caCertPool
	}
	return t, nil
}

// Validate kafka config
func (c *KafkaConfig) Validate() error {
	if c.Brokers == nil || len(c.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are not defined")
	}
	//if c.Username == "" {
	//	return fmt.Errorf("kafka username is not defined")
	//}
	//if c.Password == "" {
	//	return fmt.Errorf("kafka password is not defined")
	//}
	if c.RetryMax == 0 {
		c.RetryMax = 5
	}
	if c.ChannelBuffer == 0 {
		c.ChannelBuffer = 1
	}
	if c.CommitTimeout == 0 {
		c.CommitTimeout = time.Hour // auto-commit after 1 hour
	}
	return nil
}
