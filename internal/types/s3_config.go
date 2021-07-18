package types

import "fmt"

// S3Config S3 config
type S3Config struct {
	Endpoint           string `yaml:"endpoint" mapstructure:"endpoint"`
	AccessKeyID        string `yaml:"accessKeyID" mapstructure:"accessKeyID"`
	SecretAccessKey    string `yaml:"secretAccessKey" mapstructure:"secretAccessKey"`
	Token              string `yaml:"token" mapstructure:"token"`
	Region             string `yaml:"region" mapstructure:"region"`
	Prefix             string `yaml:"prefix" mapstructure:"prefix"`
	Bucket             string `yaml:"bucket" mapstructure:"bucket"`
	EncryptionPassword string `yaml:"encryptionPassword" mapstructure:"encryptionPassword"`
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
