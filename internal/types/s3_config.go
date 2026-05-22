package types

import (
	"fmt"
	"strings"
)

// S3Config S3 config
type S3Config struct {
	Endpoint           string `yaml:"endpoint" mapstructure:"endpoint" env:"ENDPOINT"`
	AccessKeyID        string `yaml:"access_key_id" mapstructure:"access_key_id" env:"ACCESS_KEY_ID"`
	SecretAccessKey    string `yaml:"secret_access_key" mapstructure:"secret_access_key" env:"SECRET_ACCESS_KEY"`
	Token              string `yaml:"token" mapstructure:"token"`
	Region             string `yaml:"region" mapstructure:"region" env:"REGION"`
	Prefix             string `yaml:"prefix" mapstructure:"prefix" env:"PREFIX"`
	Bucket             string `yaml:"bucket" mapstructure:"bucket" env:"BUCKET"`
	EncryptionPassword string `yaml:"encryption_password" mapstructure:"encryption_password" env:"PASSWORD"`
	UseSSL             bool   `yaml:"useSSL" mapstructure:"useSSL"`
	// LocalMode starts an embedded SeaweedFS subprocess instead of using an external endpoint.
	LocalMode         bool   `yaml:"local_mode" mapstructure:"local_mode"`
	LocalDataDir      string `yaml:"local_data_dir" mapstructure:"local_data_dir"`
	LocalWeedBin      string `yaml:"local_weed_bin" mapstructure:"local_weed_bin"`
	// LocalContainerHost overrides the host used by Docker/K8s helper containers to reach the embedded store.
	LocalContainerHost string `yaml:"local_container_host" mapstructure:"local_container_host"`
}

// IsLocalMode returns true when an embedded SeaweedFS subprocess should be used.
func (c *S3Config) IsLocalMode() bool { return c.LocalMode }

// LocalContainerEndpoint returns the S3 endpoint reachable from inside Docker/K8s helper containers.
func (c *S3Config) LocalContainerEndpoint() string {
	host := c.LocalContainerHost
	if host == "" {
		host = "host.docker.internal"
	}
	port := "8333" // SeaweedFS default S3 port
	if idx := strings.LastIndex(c.Endpoint, ":"); idx >= 0 {
		port = c.Endpoint[idx+1:]
	}
	return host + ":" + port
}

// Validate - validates
func (c *S3Config) Validate() error {
	if c.LocalMode {
		// Defaults for embedded mode — credentials are only used locally
		if c.AccessKeyID == "" {
			c.AccessKeyID = "localkey"
		}
		if c.SecretAccessKey == "" {
			c.SecretAccessKey = "localsecret"
		}
		if c.Bucket == "" {
			c.Bucket = "formicary-artifacts"
		}
		if c.Region == "" {
			c.Region = "us-east-1"
		}
		if c.LocalDataDir == "" {
			c.LocalDataDir = "./data/seaweedfs"
		}
		if c.LocalWeedBin == "" {
			c.LocalWeedBin = "weed"
		}
		return nil
	}
	if c.AccessKeyID == "" {
		return fmt.Errorf("s3 access-key is not defined")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("s3 secret-access-key is not defined")
	}
	if c.Bucket == "" {
		return fmt.Errorf("s3 bucket is not defined")
	}
	if c.Endpoint == "" {
		c.Endpoint = "s3.amazonaws.com"
	}
	if c.Region == "" {
		c.Region = "US-WEST-2"
	}
	return nil
}

func (c *S3Config) BuildEndpoint() string {
	if c.UseSSL {
		return fmt.Sprintf("%s://%s", httpsPrefix, c.Endpoint)
	} else {
		return fmt.Sprintf("%s://%s", httpPrefix, c.Endpoint)
	}
}
