package types

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/xdg-go/scram"
	"io/ioutil"
	"log"
	"os"
)

// KafkaConfig kakfa config
type KafkaConfig struct {
	Brokers   []string `yaml:"brokers"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Algorithm string   `yaml:"algorithm"` // sha256
	CertFile  string   `yaml:"certificate"`
	KeyFile   string   `yaml:"key"`
	CAFile    string   `yaml:"ca"`
	VerifySSL bool     `yaml:"verify"`
	UseTLS    bool     `yaml:"tls"`
	UseSasl   bool     `yaml:"sasl"`
	RetryMax  int      `yaml:"retry_max"`
	Assignor  string   `yaml:"assignor"`
	Group     string   `yaml:"group"`
	clientID  string
	debug     bool
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

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: c.VerifySSL,
		}
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
	if c.clientID == "" {
		return fmt.Errorf("kafka clientID is not defined")
	}
	if c.RetryMax == 0 {
		c.RetryMax = 5
	}
	if c.Assignor == "" {
		c.Assignor = "roundrobin"
	}
	return nil
}

// BuildSaramaConfig creates sarama config
func (c *KafkaConfig) BuildSaramaConfig(debug bool, oldest bool) (conf *sarama.Config, err error) {
	if debug {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}
	conf = sarama.NewConfig()
	conf.Producer.Retry.Max = c.RetryMax
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Producer.Compression = sarama.CompressionSnappy
	//conf.Producer.Flush.Frequency = 100 * time.Millisecond
	//conf.Producer.Idempotent = true
	//conf.Net.MaxOpenRequests = 1 // idempotent

	switch c.Assignor {
	case "sticky":
		conf.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	case "roundrobin":
		conf.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	case "range":
		conf.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	default:
		return nil, fmt.Errorf("unrecognized consumer group partition assignor: %s", c.Assignor)
	}
	if oldest {
		conf.Consumer.Offsets.Initial = sarama.OffsetOldest
	}
	conf.Producer.Return.Successes = true
	conf.Consumer.Return.Errors = true
	conf.Metadata.Full = true
	conf.Version = sarama.V2_0_0_0
	conf.ClientID = c.clientID
	conf.Metadata.Full = true
	if c.UseSasl {
		conf.Net.SASL.Enable = true
		conf.Net.SASL.User = c.Username
		conf.Net.SASL.Password = c.Password
		conf.Net.SASL.Handshake = true
		if c.Algorithm == "sha512" {
			conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
			conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		} else if c.Algorithm == "sha256" {
			conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
			conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256

		} else {
			return nil, fmt.Errorf("invalid SHA algorithm '%s': can be either 'sha256' or 'sha512'", c.Algorithm)
		}

		if c.UseTLS {
			conf.Net.TLS.Enable = true
			conf.Net.TLS.Config, err = c.BuildTLSConfiguration()
			if err != nil {
				return nil, err
			}
		}
	}
	return conf, nil
}

// XDGSCRAMClient definition
type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

// Begin XDGSCRAMClient implements SCRAMClient
func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

// Step XDGSCRAMClient implements SCRAMClient
func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

// Done XDGSCRAMClient implements SCRAMClient
func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
