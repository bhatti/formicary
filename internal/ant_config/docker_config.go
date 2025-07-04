package ant_config

import "plexobject.com/formicary/internal/types"

// DockerConfig -- Default Docker Config
type DockerConfig struct {
	Registry `yaml:"registry"`
	Host     string               `yaml:"host" json:"host" mapstructure:"host" env:"HOST"`
	Labels      map[string]string    `yaml:"labels" json:"labels" mapstructure:"labels"`
	Environment types.EnvironmentMap `yaml:"environment" json:"environment" mapstructure:"environment"`
	HelperImage string               `yaml:"helper_image" json:"helper_image" mapstructure:"helper_image"`
}

// Validate config
func (dc *DockerConfig) Validate() error {
	if dc.Registry.PullPolicy == "" {
		dc.Registry.PullPolicy = types.PullPolicyIfNotPresent
	}
	if dc.HelperImage == "" {
		dc.HelperImage = "amazon/aws-cli"
	}
	return nil
}
