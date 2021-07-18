package types

import (
	"fmt"
	"github.com/docker/distribution/reference"
	api "k8s.io/api/core/v1"
	"plexobject.com/formicary/internal/utils"
	"regexp"
	"strings"
)

// Service service definition
type Service struct {
	Name                    string             `yaml:"name" json:"name"`
	Alias                   string             `yaml:"alias,omitempty" json:"alias,omitempty"`
	WorkingDirectory        string             `yaml:"working_directory,omitempty" json:"working_directory,omitempty"`
	Image                   string             `yaml:"image,omitempty" json:"image,omitempty"`
	Command                 []string           `yaml:"command" json:"command"`
	Entrypoint              []string           `yaml:"entrypoint" json:"entrypoint"`
	Volumes                 *KubernetesVolumes `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Ports                   []Port             `yaml:"ports" json:"ports"`
	CPULimit                string             `yaml:"cpu_limit,omitempty" json:"cpu_limit,omitempty"`
	CPURequest              string             `yaml:"cpu_request,omitempty" json:"cpu_request,omitempty"`
	MemoryLimit             string             `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
	MemoryRequest           string             `yaml:"memory_request,omitempty" json:"memory_request,omitempty"`
	EphemeralStorageLimit   string             `json:"ephemeral_storage_limit,omitempty" yaml:"ephemeral_storage_limit,omitempty"`
	EphemeralStorageRequest string             `json:"ephemeral_storage_request,omitempty" yaml:"ephemeral_storage_request,omitempty"`
}

// ParsedService parsed service
type ParsedService struct {
	Service string
	Image   string
	Aliases []string
	Version string
}

// ToImageDefinition image
func (s *Service) ToImageDefinition() Image {
	if s.Ports == nil {
		s.Ports = []Port{}
	}
	return Image{
		Name:       s.Name,
		Alias:      s.Alias,
		Command:    s.Command,
		Entrypoint: s.Entrypoint,
		Ports:      s.Ports,
	}
}

// ToContainer container
func (s *Service) ToContainer(
	volumeMounts []api.VolumeMount,
	pullPolicy api.PullPolicy) api.Container {
	return api.Container{
		Name:            s.Name,
		Image:           s.Image,
		Command:         s.Command,
		VolumeMounts:    volumeMounts,
		ImagePullPolicy: pullPolicy,
	}
}

// AddEmptyKubernetesVolume add empty volume
func (s *Service) AddEmptyKubernetesVolume(name string, mountPath string) {
	volumes := s.GetKubernetesVolumes()
	volumes.AddEmptyVolume(name, mountPath)
}

// GetKubernetesVolumes volumes for kubernetes
func (s *Service) GetKubernetesVolumes() *KubernetesVolumes {
	if s.Volumes == nil {
		s.Volumes = NewKubernetesVolumes()
	}
	return s.Volumes
}

// CreateHostAliases create host aliases
func CreateHostAliases(services []Service, hostAliases []api.HostAlias) ([]api.HostAlias, error) {
	servicesHostAlias, err := createServicesHostAlias(services)
	if err != nil {
		return hostAliases, err
	}

	var allHostAliases []api.HostAlias
	if servicesHostAlias != nil {
		allHostAliases = append(allHostAliases, *servicesHostAlias)
	}
	allHostAliases = append(allHostAliases, hostAliases...)

	return allHostAliases, nil
}

func createServicesHostAlias(srvs []Service) (*api.HostAlias, error) {
	var hostnames []string

	for _, srv := range srvs {
		if len(srv.Ports) > 0 {
			continue
		}

		serviceMeta := SplitNameAndVersion(srv.Name)
		for _, alias := range serviceMeta.Aliases {
			// For backward compatibility reasons a non DNS1123 compliant alias might be generated,
			// this will be removed in https://gitlab.com/gitlab-org/gitlab-runner/issues/6100
			err := utils.ValidateDNS1123Subdomain(alias)
			if err == nil {
				hostnames = append(hostnames, alias)
			}
		}

		if srv.Alias == "" {
			continue
		}
		err := utils.ValidateDNS1123Subdomain(srv.Alias)
		if err != nil {
			return nil, &invalidHostAliasDNSError{service: srv, inner: err}
		}

		hostnames = append(hostnames, srv.Alias)
	}

	// no service hostnames to add to aliases
	if len(hostnames) == 0 {
		return nil, nil
	}

	return &api.HostAlias{IP: "127.0.0.1", Hostnames: hostnames}, nil
}

type invalidHostAliasDNSError struct {
	service Service
	inner   error
}

func (e *invalidHostAliasDNSError) Error() string {
	return fmt.Sprintf(
		"provided host alias %s for service %s is invalid DNS. %s",
		e.service.Alias,
		e.service.Name,
		e.inner,
	)
}

var referenceRegexpNoPort = regexp.MustCompile(`^(.*?)(|:[0-9]+)(|/.*)$`)

const imageVersionLatest = "latest"

// SplitNameAndVersion parses Docker registry image urls and constructs a struct with correct
// image url, name, version and aliases
func SplitNameAndVersion(serviceDescription string) ParsedService {
	// Try to find matches in e.g. subdomain.domain.tld:8080/namespace/service:version
	matches := reference.ReferenceRegexp.FindStringSubmatch(serviceDescription)
	if len(matches) == 0 {
		return ParsedService{
			Image:   serviceDescription,
			Version: imageVersionLatest,
		}
	}

	// -> subdomain.domain.tld:8080/namespace/service
	imageWithoutVersion := matches[1]
	// -> version
	imageVersion := matches[2]

	registryMatches := referenceRegexpNoPort.FindStringSubmatch(imageWithoutVersion)
	// -> subdomain.domain.tld
	registry := registryMatches[1]
	// -> /namespace/service
	imageName := registryMatches[3]

	service := ParsedService{}
	service.Service = registry + imageName

	if len(imageVersion) > 0 {
		service.Image = serviceDescription
		service.Version = imageVersion
	} else {
		service.Image = fmt.Sprintf("%s:%s", imageWithoutVersion, imageVersionLatest)
		service.Version = imageVersionLatest
	}

	alias := strings.ReplaceAll(service.Service, "/", "__")
	service.Aliases = append(service.Aliases, alias)

	// Create alternative link name according to RFC 1123
	// Where you can use only `a-zA-Z0-9-`
	alternativeName := strings.ReplaceAll(service.Service, "/", "-")
	if alias != alternativeName {
		service.Aliases = append(service.Aliases, alternativeName)
	}
	return service
}
