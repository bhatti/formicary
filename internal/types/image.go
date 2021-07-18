package types

import (
	api "k8s.io/api/core/v1"
)

// Image image definition
type Image struct {
	Name       string   `yaml:"name,omitempty" json:"name,omitempty"`
	Alias      string   `yaml:"alias,omitempty" json:"alias,omitempty"`
	Command    []string `yaml:"command,omitempty" json:"command,omitempty"`
	Entrypoint []string `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Ports      []Port   `yaml:"ports,omitempty" json:"ports,omitempty"`
}

// GetPorts ports
func (img *Image) GetPorts() ([]api.ContainerPort, []Port) {
	containerPorts := make([]api.ContainerPort, len(img.Ports))
	proxyPorts := make([]Port, len(img.Ports))

	for i, port := range img.Ports {
		if port.Protocol == "" {
			port.Protocol = string(api.ProtocolTCP)
		}
		proxyPorts[i] = Port{
			Name:     port.Name,
			Number:   port.Number,
			Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{
			Name:          port.Name,
			ContainerPort: int32(port.Number),
			HostPort:      int32(port.Number),
			Protocol:      api.Protocol(port.Protocol)}
	}
	return containerPorts, proxyPorts
}

// Port port definition
type Port struct {
	Number   int    `yaml:"number,omitempty" json:"number"`
	Protocol string `yaml:"protocol,omitempty" json:"protocol"`
	Name     string `yaml:"name,omitempty" json:"name"`
}
