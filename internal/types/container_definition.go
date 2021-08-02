package types

import (
	"fmt"
	"github.com/docker/docker/api/types/mount"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"reflect"
	"strings"
)

// ContainerDefinition defines container config
type ContainerDefinition struct {
	Image                   string      `json:"image" yaml:"image"`
	ImageDefinition         Image       `json:"imageDefinition,omitempty" yaml:"imageDefinition,omitempty"`
	Volumes                 interface{} `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	VolumeDriver            string      `json:"volume_driver,omitempty" yaml:"volume_driver,omitempty"`
	VolumesFrom             []string    `json:"volumes_from,omitempty" yaml:"volumes_from,omitempty"`
	Devices                 []string    `json:"devices,omitempty" yaml:"devices,omitempty"`
	BindDirectory           string      `json:"bind_dir,omitempty" yaml:"bind_dir,omitempty"`
	CPULimit                string      `json:"cpu_limit,omitempty" yaml:"cpu_limit,omitempty"`
	CPURequest              string      `json:"cpu_request,omitempty" yaml:"cpu_request,omitempty"`
	MemoryLimit             string      `json:"memory_limit,omitempty" yaml:"memory_limit,omitempty"`
	MemoryRequest           string      `json:"memory_request,omitempty" yaml:"memory_request,omitempty"`
	EphemeralStorageLimit   string      `json:"ephemeral_storage_limit,omitempty" yaml:"ephemeral_storage_limit,omitempty"`
	EphemeralStorageRequest string      `json:"ephemeral_storage_request,omitempty" yaml:"ephemeral_storage_request,omitempty"`
}

// NewContainerDefinition constructor
func NewContainerDefinition() *ContainerDefinition {
	return &ContainerDefinition{
		VolumesFrom: make([]string, 0),
		Devices:     make([]string, 0),
	}
}

func (cd *ContainerDefinition) String() string {
	return fmt.Sprintf(
		"Image=%s, CPU=%s/%s, Memory=%s/%s",
		cd.Image,
		cd.CPULimit,
		cd.CPURequest,
		cd.MemoryLimit,
		cd.MemoryRequest)
}

// HasDockerBindVolumes for docker volume
func (cd *ContainerDefinition) HasDockerBindVolumes() bool {
	return cd.Volumes != nil
}

// HasDockerFromVolumes for docker volume
func (cd *ContainerDefinition) HasDockerFromVolumes() bool {
	return cd.Volumes != nil &&
		cd.VolumesFrom != nil &&
		len(cd.VolumesFrom) > 0 &&
		len(cd.GetDockerVolumes()) == len(cd.VolumesFrom)
}

// HasKubernetesVolumes for kubernetes volume
func (cd *ContainerDefinition) HasKubernetesVolumes() bool {
	return cd.Volumes != nil &&
		cd.VolumesFrom != nil &&
		len(cd.VolumesFrom) > 0 &&
		len(cd.GetKubernetesVolumes().HostPaths) == len(cd.VolumesFrom)
}

// AddEmptyKubernetesVolume adds volume
func (cd *ContainerDefinition) AddEmptyKubernetesVolume(name string, mountPath string) {
	volumes := cd.GetKubernetesVolumes()
	volumes.AddEmptyVolume(name, mountPath)
}

// GetKubernetesVolumes volumes for kubernetes
func (cd *ContainerDefinition) GetKubernetesVolumes() *KubernetesVolumes {
	if cd.Volumes == nil {
		cd.Volumes = NewKubernetesVolumes()
	}
	cd.Volumes = getKubernetesVolumes(cd.Volumes)
	return cd.Volumes.(*KubernetesVolumes)
}

// GetDockerVolumeNames volumes for docker
func (cd *ContainerDefinition) GetDockerVolumeNames() map[string]string {
	if cd.Volumes == nil {
		cd.Volumes = make(map[string]string)
	}
	switch cd.Volumes.(type) {
	case map[string]string:
		return cd.Volumes.(map[string]string)
	case map[string]interface{}:
		m := make(map[string]string)
		for k, v := range cd.Volumes.(map[string]interface{}) {
			m[k] = fmt.Sprintf("%v", v)
		}
		cd.Volumes = m
		return m
	default:
		logrus.WithFields(
			logrus.Fields{
				"Component": "ContainerDefinition",
				"Type":      reflect.TypeOf(cd.Volumes),
				"Volumes":   cd.Volumes,
			}).Warn("unknown docker volumes type")
		return make(map[string]string)
	}
}

// GetDockerVolumes volumes for docker
func (cd *ContainerDefinition) GetDockerVolumes() map[string]struct{} {
	m := make(map[string]struct{})
	if cd.Volumes == nil {
		cd.Volumes = make(map[string]string)
	}
	switch cd.Volumes.(type) {
	case map[string]string:
		for k := range cd.Volumes.(map[string]string) {
			m[k] = struct{}{}
		}
	case map[string]interface{}:
		for k := range cd.Volumes.(map[string]interface{}) {
			m[k] = struct{}{}
		}
	default:
		logrus.WithFields(
			logrus.Fields{
				"Component": "ContainerDefinition",
				"Type":      reflect.TypeOf(cd.Volumes),
				"Volumes":   cd.Volumes,
			}).Warn("unknown docker volumes type")
	}
	return m
}

// GetDockerMounts mount volumes for docker
func (cd *ContainerDefinition) GetDockerMounts() []mount.Mount {
	vols := cd.GetDockerVolumeNames()
	mounts := make([]mount.Mount, 0)
	for k, v := range vols {
		var m mount.Mount
		if strings.Contains(k, "bind-mount") {
			m = mount.Mount{
				Type:   mount.TypeBind,
				Source: v,
				Target: k,
			}
		} else {
			m = mount.Mount{
				Type:   mount.TypeVolume,
				Source: k,
				Target: v,
			}
		}
		mounts = append(mounts, m)
	}
	return mounts
}

func getKubernetesVolumes(v interface{}) *KubernetesVolumes {
	switch v.(type) {
	case *KubernetesVolumes:
		return v.(*KubernetesVolumes)
	case map[string]interface{}:
		if b, err := yaml.Marshal(v); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "ContainerDefinition",
					"Error":     err,
					"Volumes":   v,
				}).Warn("failed to serialize kubernetes volumes")
		} else {
			vols := NewKubernetesVolumes()
			if err = yaml.Unmarshal(b, vols); err == nil {
				return vols
			}
			logrus.WithFields(
				logrus.Fields{
					"Component": "ContainerDefinition",
					"Error":     err,
					"Volumes":   v,
				}).Warn("failed to deserialize kubernetes volumes")
		}
	}
	return NewKubernetesVolumes()
}

