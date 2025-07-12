package types

import "fmt"

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
}

// Validate - validates
func (c *S3Config) Validate() error {
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
