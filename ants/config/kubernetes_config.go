package config

import (
	"fmt"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
)

// See https://pkg.go.dev/k8s.io/kubernetes@v1.19.4/pkg/apis/core#Toleration

// KubernetesConfig -- Default Kubernetes Config
type KubernetesConfig struct {
	Registry                 `yaml:"registry"`
	Host                     string                             `yaml:"host"`
	BearerToken              string                             `yaml:"bearer_token,omitempty" json:"bearer_token" mapstructure:"bearer_token"`
	CertFile                 string                             `yaml:"cert_file,omitempty" json:"cert_file" mapstructure:"cert_file"`
	KeyFile                  string                             `yaml:"key_file,omitempty" json:"key_file" mapstructure:"key_file"`
	CAFile                   string                             `yaml:"ca_file,omitempty" json:"ca_file" mapstructure:"ca_file"`
	Namespace                string                             `yaml:"namespace" env:"KUBERNETES_NAMESPACE"`
	HelperImage              string                             `yaml:"helper_image" json:"helper_image" mapstructure:"helper_image"`
	ServiceAccount           string                             `yaml:"service_account" json:"service_account" mapstructure:"service_account"`
	ImagePullSecrets         []string                           `yaml:"image_pull_secrets" json:"image_pull_secrets" mapstructure:"image_pull_secrets"`
	InitContainers           []types.Service                    `yaml:"init_containers" json:"init_containers" mapstructure:"init_containers"`
	AllowPrivilegeEscalation bool                               `yaml:"allow_privilege_escalation" json:"allow_privilege_escalation" mapstructure:"allow_privilege_escalation"`
	DNSPolicy                types.KubernetesDNSPolicy          `yaml:"dns_policy" json:"dns_policy,omitempty" mapstructure:"dns_policy,omitempty"`
	DNSConfig                *types.KubernetesDNSConfig         `yaml:"dns_config" json:"dns_config,omitempty" mapstructure:"dns_config,omitempty"`
	Volumes                  types.KubernetesVolumes            `yaml:"volumes"`
	PodSecurityContext       types.KubernetesPodSecurityContext `yaml:"pod_security_context,omitempty" json:"pod_security_context,omitempty" mapstructure:"pod_security_context,omitempty"`
	HostAliases              []types.KubernetesHostAliases      `json:"host_aliases" yaml:"host_aliases,omitempty" mapstructure:"host_aliases,omitempty"`
	CapAdd                   []string                           `yaml:"cap_add" json:"cap_add" mapstructure:"cap_add"`
	CapDrop                  []string                           `yaml:"cap_drop" json:"cap_drop" mapstructure:"cap_drop"`
	Environment              types.EnvironmentMap               `yaml:"environment" json:"environment"`
	DefaultLimits            api.ResourceList                   `yaml:"default_limits" json:"default_limits"`
	MinLimits                api.ResourceList                   `yaml:"min_limits" json:"min_limits"`
	MaxLimits                api.ResourceList                   `yaml:"max_limits" json:"max_limits"`
	MaxServicesPerPod        int                                `yaml:"max_services_per_pod" json:"max_services_per_pod"`
	AwaitShutdownPod         bool                               `yaml:"await_shutdown_pod" json:"await_shutdown_pod" mapstructure:"await_shutdown_pod"`
}

// GetInitContainers initial containers
func (kc *KubernetesConfig) GetInitContainers() []api.Container {
	volumeMounts := kc.Volumes.GetVolumeMounts()
	containers := make([]api.Container, len(kc.InitContainers))
	for i, c := range kc.InitContainers {
		containers[i] = c.ToContainer(volumeMounts, api.PullPolicy(kc.PullPolicy))
	}
	return containers
}

// GetPodSecurityContext returns security context
func (kc *KubernetesConfig) GetPodSecurityContext() *api.PodSecurityContext {
	podSecurityContext := kc.PodSecurityContext

	if podSecurityContext.FSGroup == nil &&
		podSecurityContext.RunAsGroup == nil &&
		podSecurityContext.RunAsNonRoot == nil &&
		podSecurityContext.RunAsUser == nil &&
		len(podSecurityContext.SupplementalGroups) == 0 {
		return nil
	}

	return &api.PodSecurityContext{
		FSGroup:            podSecurityContext.FSGroup,
		RunAsGroup:         podSecurityContext.RunAsGroup,
		RunAsNonRoot:       podSecurityContext.RunAsNonRoot,
		RunAsUser:          podSecurityContext.RunAsUser,
		SupplementalGroups: podSecurityContext.SupplementalGroups,
	}
}

// GetDNSConfig returns dns config
func (kc *KubernetesConfig) GetDNSConfig() *api.PodDNSConfig {
	if kc == nil {
		return nil
	}
	if kc.DNSConfig == nil ||
		(len(kc.DNSConfig.Nameservers) == 0 &&
			len(kc.DNSConfig.Searches) == 0 &&
			len(kc.DNSConfig.Options) == 0) {
		return nil
	}

	var config api.PodDNSConfig

	config.Nameservers = kc.DNSConfig.Nameservers
	config.Searches = kc.DNSConfig.Searches

	for _, opt := range kc.DNSConfig.Options {
		config.Options = append(config.Options, api.PodDNSConfigOption{
			Name:  opt.Name,
			Value: opt.Value,
		})
	}

	return &config
}

// GetHostAliases returns host aliases
func (kc *KubernetesConfig) GetHostAliases() []api.HostAlias {
	var hostAliases []api.HostAlias

	for _, hostAlias := range kc.HostAliases {
		hostAliases = append(
			hostAliases,
			api.HostAlias{
				IP:        hostAlias.IP,
				Hostnames: hostAlias.Hostnames,
			},
		)
	}

	return hostAliases
}

// CreateResourceList creates resource list for cpu, memory, and storage
func (kc *KubernetesConfig) CreateResourceList(
	kind string,
	cpu string,
	memory string,
	ephemeralStorage string) (api.ResourceList, float64, error) {
	def := kc.DefaultLimits
	min := kc.MinLimits
	max := kc.MaxLimits
	res, err := utils.CreateResourceList(cpu, memory, ephemeralStorage)
	if err != nil {
		return nil, 0, err
	}
	resourceQuantityMilliValue := func(q resource.Quantity) int64 {
		return q.MilliValue()
	}

	resourceQuantityZero := func(q resource.Quantity) bool {
		return q.IsZero()
	}

	if resourceQuantityZero(res[api.ResourceCPU]) {
		//res[api.ResourceCPU] = def[api.ResourceCPU]
	} else if !resourceQuantityZero(min[api.ResourceCPU]) &&
		resourceQuantityMilliValue(res[api.ResourceCPU]) < resourceQuantityMilliValue(min[api.ResourceCPU]) {
		return nil, 0, fmt.Errorf("cpu %s %s is smaller than min-cpu %v",
			kind, cpu, min[api.ResourceCPU])
	} else if !resourceQuantityZero(max[api.ResourceCPU]) &&
		resourceQuantityMilliValue(res[api.ResourceCPU]) > resourceQuantityMilliValue(max[api.ResourceCPU]) {
		return nil, 0, fmt.Errorf("cpu %s %s exceeds max-cpu %v",
			kind, cpu, max[api.ResourceCPU])
	}

	if resourceQuantityZero(res[api.ResourceMemory]) {
		//res[api.ResourceMemory] = def[api.ResourceMemory]
	} else if !resourceQuantityZero(min[api.ResourceMemory]) &&
		resourceQuantityMilliValue(res[api.ResourceMemory]) < resourceQuantityMilliValue(min[api.ResourceMemory]) {
		return nil, 0, fmt.Errorf("memory %s %s is smaller than min-memory %v",
			kind, memory, min[api.ResourceMemory])
	} else if !resourceQuantityZero(max[api.ResourceMemory]) &&
		resourceQuantityMilliValue(res[api.ResourceMemory]) > resourceQuantityMilliValue(max[api.ResourceMemory]) {
		return nil, 0, fmt.Errorf("memory %s %s exceeds max-memory %v",
			kind, memory, max[api.ResourceMemory])
	}

	if resourceQuantityZero(res[api.ResourceEphemeralStorage]) {
		//res[api.ResourceEphemeralStorage] = def[api.ResourceEphemeralStorage]
	} else if !resourceQuantityZero(min[api.ResourceEphemeralStorage]) &&
		resourceQuantityMilliValue(res[api.ResourceEphemeralStorage]) < resourceQuantityMilliValue(min[api.ResourceEphemeralStorage]) {
		return nil, 0, fmt.Errorf("storage %s %s is smaller than min-storage %v",
			kind, ephemeralStorage, min[api.ResourceEphemeralStorage])
	} else if !resourceQuantityZero(max[api.ResourceEphemeralStorage]) &&
		resourceQuantityMilliValue(res[api.ResourceEphemeralStorage]) > resourceQuantityMilliValue(max[api.ResourceEphemeralStorage]) {
		return nil, 0, fmt.Errorf("storage %s %s exceeds max-storage %v",
			kind, ephemeralStorage, max[api.ResourceEphemeralStorage])
	}
	return res, utils.CreateResourceCost(res, def), nil
}

// Validate config
func (kc *KubernetesConfig) Validate() error {
	if kc.Registry.PullPolicy == "" {
		kc.Registry.PullPolicy = types.PullPolicyIfNotPresent
	}

	if kc.HelperImage == "" {
		kc.HelperImage = "amazon/aws-cli"
	}
	if kc.DefaultLimits == nil {
		kc.DefaultLimits, _ = utils.CreateResourceList("0.5", "500m", "1G")
	}
	if kc.MinLimits == nil {
		kc.MinLimits, _ = utils.CreateResourceList("0.1", "100m", "100m")
	}
	if kc.MaxLimits == nil {
		kc.MaxLimits, _ = utils.CreateResourceList("2.0", "4G", "2G")
	}
	if kc.MaxServicesPerPod <= 0 {
		kc.MaxServicesPerPod = 100
	}
	if len(kc.HostAliases) == 0 {
		kc.HostAliases = []types.KubernetesHostAliases{
			{Hostnames: []string{"dns1", "dns2"}, IP: "8.8.8.8"},
		}
	}
	return nil
}
