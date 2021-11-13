package types

import (
	"fmt"
	api "k8s.io/api/core/v1"
)

// See https://pkg.go.dev/k8s.io/kubernetes@v1.19.4/pkg/apis/core#Toleration

// KubernetesHostAliases host aliases
// swagger:ignore
type KubernetesHostAliases struct {
	IP        string   `yaml:"ip" json:"ip"`
	Hostnames []string `yaml:"hostnames" json:"hostnames"`
}

// KubernetesVolumes volumes
// swagger:ignore
type KubernetesVolumes struct {
	HostPaths  []KubernetesHostPath  `yaml:"host_path" json:"host_path" mapstructure:"host_path"`
	PVCs       []KubernetesPVC       `yaml:"pvc" json:"pvc" mapstructure:"pvc"`
	ConfigMaps []KubernetesConfigMap `yaml:"config_map" json:"config_map" mapstructure:"config_map"`
	Secrets    []KubernetesSecret    `yaml:"secret" json:"secret" mapstructure:"secret"`
	Projected  []KubernetesProjected `yaml:"projected" json:"projected" mapstructure:"projected"`
	EmptyDirs  []KubernetesEmptyDir  `yaml:"empty_dir" json:"empty_dir" mapstructure:"empty_dir"`
}

// NewKubernetesVolumes constructor for volumes
func NewKubernetesVolumes() *KubernetesVolumes {
	return &KubernetesVolumes{
		HostPaths:  make([]KubernetesHostPath, 0),
		PVCs:       make([]KubernetesPVC, 0),
		ConfigMaps: make([]KubernetesConfigMap, 0),
		Secrets:    make([]KubernetesSecret, 0),
		EmptyDirs:  make([]KubernetesEmptyDir, 0),
	}
}

// KubernetesConfigMap map
// swagger:ignore
type KubernetesConfigMap struct {
	Name      string            `yaml:"name" json:"name"`
	MountPath string            `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	SubPath   string            `yaml:"sub_path,omitempty" json:"sub_path,omitempty" mapstructure:"sub_path,omitempty"`
	ReadOnly  bool              `yaml:"read_only,omitempty" json:"read_only,omitempty" mapstructure:"read_only,omitempty"`
	Items     map[string]string `yaml:"items,omitempty" json:"items,omitempty"`
}

// KubernetesHostPath host-path
// swagger:ignore
type KubernetesHostPath struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	SubPath   string `yaml:"sub_path,omitempty" json:"sub_path,omitempty" mapstructure:"sub_path,omitempty"`
	ReadOnly  bool   `yaml:"read_only,omitempty" json:"read_only,omitempty" mapstructure:"read_only,omitempty"`
	HostPath  string `yaml:"host_path,omitempty" json:"host_path,omitempty" mapstructure:"host_path,omitempty"`
}

// KubernetesPVC pvc
// swagger:ignore
type KubernetesPVC struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	SubPath   string `yaml:"sub_path,omitempty" json:"sub_path,omitempty" mapstructure:"sub_path,omitempty"`
	ReadOnly  bool   `yaml:"read_only,omitempty" json:"read_only,omitempty" mapstructure:"read_only,omitempty"`
}

// KubernetesSecret secrets
// swagger:ignore
type KubernetesSecret struct {
	Name      string            `yaml:"name" json:"name"`
	MountPath string            `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	SubPath   string            `yaml:"sub_path,omitempty" json:"sub_path,omitempty" mapstructure:"sub_path,omitempty"`
	ReadOnly  bool              `yaml:"read_only,omitempty" json:"read_only,omitempty" mapstructure:"read_only,omitempty"`
	Items     map[string]string `yaml:"items,omitempty" json:"items,omitempty"`
}

// KubernetesProjected sources
// swagger:ignore
type KubernetesProjected struct {
	Name      string                       `yaml:"name" json:"name"`
	MountPath string                       `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	Sources   []KubernetesVolumeProjection `yaml:"sources" json:"sources" mapstructure:"sources"`
}

// KubernetesVolumeProjection sources
// swagger:ignore
type KubernetesVolumeProjection struct {
	Secret              *KubernetesSecretProjection              `yaml:"secret" json:"secret" mapstructure:"secret"`
	ConfigMap           *KubernetesConfigMapProjection           `yaml:"config_map" json:"config_map" mapstructure:"config_map"`
	ServiceAccountToken *KubernetesServiceAccountTokenProjection `yaml:"service_account_token" json:"service_account_token" mapstructure:"service_account_token"`
}

// KubernetesSecretProjection projection
type KubernetesSecretProjection struct {
	Items []api.KeyToPath `yaml:"items" json:"items" mapstructure:"items"`
}

// KubernetesConfigMapProjection projection
type KubernetesConfigMapProjection struct {
	Items []api.KeyToPath `yaml:"items" json:"items" mapstructure:"items"`
}

// KubernetesServiceAccountTokenProjection account
// swagger:ignore
type KubernetesServiceAccountTokenProjection struct {
	Audience          string `yaml:"audience" json:"audience" mapstructure:"audience"`
	ExpirationSeconds *int64 `yaml:"expiration_seconds" json:"expiration_seconds" mapstructure:"expiration_seconds"`
	Path              string `yaml:"path" json:"path" mapstructure:"path"`
}

// KubernetesEmptyDir empty-dir
// swagger:ignore
type KubernetesEmptyDir struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mount_path" json:"mount_path" mapstructure:"mount_path"`
	SubPath   string `yaml:"sub_path,omitempty" json:"sub_path,omitempty" mapstructure:"sub_path,omitempty"`
	Medium    string `yaml:"medium,omitempty" json:"medium,omitempty" mapstructure:"medium,omitempty"`
}

// KubernetesPodSecurityContext security
// swagger:ignore
type KubernetesPodSecurityContext struct {
	FSGroup            *int64  `yaml:"fs_group,omitempty" json:"fs_group" mapstructure:"fs_group"`
	RunAsGroup         *int64  `yaml:"run_as_group,omitempty" json:"run_as_group,omitempty" mapstructure:"run_as_group,omitempty"`
	RunAsNonRoot       *bool   `yaml:"run_as_non_root,omitempty" json:"run_as_non_root,omitempty" mapstructure:"run_as_non_root,omitempty"`
	RunAsUser          *int64  `yaml:"run_as_user,omitempty" json:"run_as_user,omitempty" mapstructure:"run_as_user,omitempty"`
	SupplementalGroups []int64 `yaml:"supplemental_groups,omitempty" json:"supplemental_groups,omitempty" mapstructure:"supplemental_groups,omitempty"`
}

// KubernetesAffinity affinity
//type KubernetesAffinity struct {
//	NodeAffinity *KubernetesNodeAffinity `yaml:"node_affinity" json:"node_affinity" mapstructure:"node_affinity"`
//}

// KubernetesNodeAffinity affinity
// swagger:ignore
type KubernetesNodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `yaml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution" mapstructure:"required_during_scheduling_ignored_during_execution"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `yaml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution" mapstructure:"preferred_during_scheduling_ignored_during_execution"`
}

// NodeSelector selector
// swagger:ignore
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `yaml:"node_selector_terms" json:"node_selector_terms" mapstructure:"node_selector_terms"`
}

// GetNodeSelector node selector
func (c *NodeSelector) GetNodeSelector() *api.NodeSelector {
	var nodeSelector api.NodeSelector
	for _, selector := range c.NodeSelectorTerms {
		nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, selector.GetNodeSelectorTerm())
	}
	return &nodeSelector
}

// PreferredSchedulingTerm term
// swagger:ignore
type PreferredSchedulingTerm struct {
	Weight     int32            `yaml:"weight" json:"weight"`
	Preference NodeSelectorTerm `yaml:"preference" json:"preference"`
}

// GetPreferredSchedulingTerm term
func (c *PreferredSchedulingTerm) GetPreferredSchedulingTerm() api.PreferredSchedulingTerm {
	return api.PreferredSchedulingTerm{
		Weight:     c.Weight,
		Preference: c.Preference.GetNodeSelectorTerm(),
	}
}

// NodeSelectorTerm term
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `yaml:"match_expressions,omitempty" json:"match_expressions" mapstructure:"match_expressions"`
	MatchFields      []NodeSelectorRequirement `yaml:"match_fields,omitempty" json:"match_fields" mapstructure:"match_fields"`
}

// GetNodeSelectorTerm term
func (c *NodeSelectorTerm) GetNodeSelectorTerm() api.NodeSelectorTerm {
	var nodeSelectorTerm = api.NodeSelectorTerm{}
	for _, expression := range c.MatchExpressions {
		nodeSelectorTerm.MatchExpressions = append(
			nodeSelectorTerm.MatchExpressions,
			expression.GetNodeSelectorRequirement())
	}
	for _, fields := range c.MatchFields {
		nodeSelectorTerm.MatchFields = append(
			nodeSelectorTerm.MatchFields,
			fields.GetNodeSelectorRequirement())
	}
	return nodeSelectorTerm
}

// NodeSelectorRequirement selector
// swagger:ignore
type NodeSelectorRequirement struct {
	Key      string   `yaml:"key,omitempty" json:"key"`
	Operator string   `yaml:"operator,omitempty" json:"operator"`
	Values   []string `yaml:"values,omitempty" json:"values"`
}

// GetNodeSelectorRequirement selector
func (c *NodeSelectorRequirement) GetNodeSelectorRequirement() api.NodeSelectorRequirement {
	return api.NodeSelectorRequirement{
		Key:      c.Key,
		Operator: api.NodeSelectorOperator(c.Operator),
		Values:   c.Values,
	}
}

func addVolumeMounts(mounts []api.VolumeMount, mount api.VolumeMount) ([]api.VolumeMount, error) {
	for _, m := range mounts {
		if m.Name == mount.Name {
			return mounts, fmt.Errorf("duplicate mount %s", mount.Name)
		}
	}
	return append(mounts, mount), nil
}

// GetVolumeMounts mount volumes
func (kv *KubernetesVolumes) GetVolumeMounts() []api.VolumeMount {
	var mounts []api.VolumeMount
	return kv.AddVolumeMounts(mounts)
}

// AddEmptyVolume adds empty volume
func (kv *KubernetesVolumes) AddEmptyVolume(name string, mountPath string) {
	found := false
	for _, vol := range kv.EmptyDirs {
		if vol.Name == name {
			found = true
			break
		}
	}
	if !found {
		kv.EmptyDirs = append(kv.EmptyDirs,
			KubernetesEmptyDir{
				Name:      name,
				MountPath: mountPath,
			},
		)
	}
}

// AddConfigVolume adds config volume
func (kv *KubernetesVolumes) AddConfigVolume(name string, mountPath string) {
	found := false
	for _, vol := range kv.ConfigMaps {
		if vol.Name == name {
			found = true
			break
		}
	}
	if !found {
		kv.ConfigMaps = append(kv.ConfigMaps,
			KubernetesConfigMap{
				Name:      name,
				MountPath: mountPath,
			},
		)
	}
}

// AddVolumeMounts adds mount volumes
func (kv *KubernetesVolumes) AddVolumeMounts(mounts []api.VolumeMount) []api.VolumeMount {
	for _, mount := range kv.HostPaths {
		mounts, _ = addVolumeMounts(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range kv.Secrets {
		mounts, _ = addVolumeMounts(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range kv.PVCs {
		mounts, _ = addVolumeMounts(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range kv.ConfigMaps {
		mounts, _ = addVolumeMounts(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}
	for _, mount := range kv.EmptyDirs {
		mounts, _ = addVolumeMounts(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
		})
	}

	return mounts
}

func addVolume(volumes []api.Volume, volume api.Volume) ([]api.Volume, error) {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return volumes, fmt.Errorf("duplicate mount %s", volume.Name)
		}
	}
	return append(volumes, volume), nil
}

func addVolumes(oldVolumes []api.Volume, newVolumes []api.Volume) []api.Volume {
	all := oldVolumes
	for _, v := range newVolumes {
		all, _ = addVolume(all, v)
	}
	return all
}

// GetVolumes volumes
func (kv *KubernetesVolumes) GetVolumes() []api.Volume {
	var volumes []api.Volume
	return kv.AddVolumes(volumes)
}

// AddVolumes adds volumes
func (kv *KubernetesVolumes) AddVolumes(volumes []api.Volume) []api.Volume {
	volumes = addVolumes(volumes, kv.getVolumesForHostPaths())
	volumes = addVolumes(volumes, kv.getVolumesForSecrets())
	volumes = addVolumes(volumes, kv.getVolumesForPVCs())
	volumes = addVolumes(volumes, kv.getVolumesForConfigMaps())
	volumes = addVolumes(volumes, kv.getVolumesForEmptyDirs())
	volumes = addVolumes(volumes, kv.getVolumesForProjected())

	return volumes
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (kv *KubernetesVolumes) getVolumesForHostPaths() []api.Volume {
	var volumes []api.Volume

	for _, volume := range kv.HostPaths {
		path := volume.HostPath
		if path == "" {
			path = volume.MountPath
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: path,
				},
			},
		})
	}

	return volumes
}

func (kv *KubernetesVolumes) getVolumesForProjected() []api.Volume {
	var volumes []api.Volume

	for _, volume := range kv.Projected {
		var sources []api.VolumeProjection
		for _, source := range volume.Sources {
			projection := api.VolumeProjection{}
			if source.Secret != nil {
				projection.Secret = &api.SecretProjection{
					Items: source.Secret.Items,
				}
			}
			if source.ServiceAccountToken != nil {
				projection.ServiceAccountToken = &api.ServiceAccountTokenProjection{
					Audience:          source.ServiceAccountToken.Audience,
					Path:              source.ServiceAccountToken.Path,
					ExpirationSeconds: source.ServiceAccountToken.ExpirationSeconds,
				}
			}
			if source.ConfigMap != nil {
				projection.ConfigMap = &api.ConfigMapProjection{
					Items: source.ConfigMap.Items,
				}
			}
			sources = append(sources, projection)
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Projected: &api.ProjectedVolumeSource{
					Sources: sources,
				},
			},
		})
	}

	return volumes
}

func (kv *KubernetesVolumes) getVolumesForSecrets() []api.Volume {
	var volumes []api.Volume

	for _, volume := range kv.Secrets {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Secret: &api.SecretVolumeSource{
					SecretName: volume.Name,
					Items:      items,
				},
			},
		})
	}

	return volumes
}

func (kv *KubernetesVolumes) getVolumesForPVCs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range kv.PVCs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
					ClaimName: volume.Name,
					ReadOnly:  volume.ReadOnly,
				},
			},
		})
	}

	return volumes
}

func (kv *KubernetesVolumes) getVolumesForConfigMaps() []api.Volume {
	var volumes []api.Volume

	mode := int32(0777)
	optional := false
	for _, volume := range kv.ConfigMaps {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				ConfigMap: &api.ConfigMapVolumeSource{
					LocalObjectReference: api.LocalObjectReference{
						Name: volume.Name,
					},
					DefaultMode: &mode,
					Optional:    &optional,
					Items:       items,
				},
			},
		})
	}

	return volumes
}

func (kv *KubernetesVolumes) getVolumesForEmptyDirs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range kv.EmptyDirs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{
					Medium: api.StorageMedium(volume.Medium),
				},
			},
		})
	}
	return volumes
}
