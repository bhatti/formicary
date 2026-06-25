package types

import (
	"fmt"
	"strings"

	logrus "github.com/sirupsen/logrus"
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
	LocalMode    bool   `yaml:"local_mode" mapstructure:"local_mode"`
	LocalDataDir string `yaml:"local_data_dir" mapstructure:"local_data_dir"`
	LocalWeedBin string `yaml:"local_weed_bin" mapstructure:"local_weed_bin"`
	// LocalS3Port pins the embedded SeaweedFS S3 port to a fixed value (0 = dynamic).
	// REQUIRED when Kubernetes helper containers must upload/download artifacts:
	// the port must be known at config time so BuildContainerEndpoint can construct
	// a stable URL before weed starts. Use a fixed value (e.g. 19000).
	// Dynamic (0) only works when all tasks run on Docker or Shell executors.
	LocalS3Port int `yaml:"local_s3_port" mapstructure:"local_s3_port"`
	// LocalContainerHost is the hostname/IP Kubernetes helper containers use to reach
	// the embedded SeaweedFS running on the host machine. Defaults to host.docker.internal
	// (works on Docker Desktop for Mac/Windows). On a Linux K8s cluster set this to the
	// host node's IP visible from pods (e.g. the node's eth0 address or a NodePort service).
	LocalContainerHost string `yaml:"local_container_host" mapstructure:"local_container_host"`
	// PublicEndpoint overrides the host:port used in presigned artifact download URLs shown
	// in the UI and returned by the API. Useful when the S3/SeaweedFS port is only reachable
	// inside the container (e.g. 127.0.0.1:19000) but the browser needs an externally-reachable
	// address (e.g. localhost:19000 when the port is published via -p 19000:19000).
	// If empty, the internal Endpoint is used as-is.
	PublicEndpoint string `yaml:"public_endpoint" mapstructure:"public_endpoint"`
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
		if c.LocalS3Port == 0 {
			logrus.Warn("s3.local_s3_port is not set; Kubernetes helper containers may fail to reach " +
				"embedded SeaweedFS — set a fixed port (e.g. local_s3_port: 19000) and expose it " +
				"from the host via local_container_host")
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

// BuildContainerEndpoint returns the S3 URL reachable from inside Docker/K8s
// helper containers. In LocalMode the embedded SeaweedFS runs on the host so
// containers must use LocalContainerEndpoint (e.g. host.docker.internal:8333).
// For a regular external S3 endpoint BuildEndpoint is used as-is.
func (c *S3Config) BuildContainerEndpoint() string {
	if c.LocalMode {
		host := c.LocalContainerHost
		if host == "" {
			host = "host.docker.internal"
		}
		port := "8333"
		if idx := strings.LastIndex(c.Endpoint, ":"); idx >= 0 {
			port = c.Endpoint[idx+1:]
		}
		return fmt.Sprintf("%s://%s:%s", httpPrefix, host, port)
	}
	return c.BuildEndpoint()
}
