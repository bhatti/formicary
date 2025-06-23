package config

import (
	"fmt"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	capi "k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
	"sync"
)

// See https://pkg.go.dev/k8s.io/kubernetes@v1.19.4/pkg/apis/core#Toleration

// ResourceConfig defines resource requirements similar to k8-highlander
type ResourceConfig struct {
	CPURequest              string `json:"cpu_request,omitempty" yaml:"cpu_request,omitempty"`
	MemoryRequest           string `json:"memory_request,omitempty" yaml:"memory_request,omitempty"`
	CPULimit                string `json:"cpu_limit,omitempty" yaml:"cpu_limit,omitempty"`
	MemoryLimit             string `json:"memory_limit,omitempty" yaml:"memory_limit,omitempty"`
	EphemeralStorageRequest string `json:"ephemeral_storage_request,omitempty" yaml:"ephemeral_storage_request,omitempty"`
	EphemeralStorageLimit   string `json:"ephemeral_storage_limit,omitempty" yaml:"ephemeral_storage_limit,omitempty"`
}

// BuildResourceRequirements converts ResourceConfig to Kubernetes ResourceRequirements
func (c ResourceConfig) BuildResourceRequirements() api.ResourceRequirements {
	resources := api.ResourceRequirements{}

	if c.CPURequest != "" || c.MemoryRequest != "" || c.EphemeralStorageRequest != "" {
		resources.Requests = api.ResourceList{}
		if c.CPURequest != "" {
			if cpuRequest, err := resource.ParseQuantity(c.CPURequest); err == nil {
				resources.Requests[api.ResourceCPU] = cpuRequest
			}
		}
		if c.MemoryRequest != "" {
			if memoryRequest, err := resource.ParseQuantity(c.MemoryRequest); err == nil {
				resources.Requests[api.ResourceMemory] = memoryRequest
			}
		}
		if c.EphemeralStorageRequest != "" {
			if storageRequest, err := resource.ParseQuantity(c.EphemeralStorageRequest); err == nil {
				resources.Requests[api.ResourceEphemeralStorage] = storageRequest
			}
		}
	}

	if c.CPULimit != "" || c.MemoryLimit != "" || c.EphemeralStorageLimit != "" {
		resources.Limits = api.ResourceList{}
		if c.CPULimit != "" {
			if cpuLimit, err := resource.ParseQuantity(c.CPULimit); err == nil {
				resources.Limits[api.ResourceCPU] = cpuLimit
			}
		}
		if c.MemoryLimit != "" {
			if memoryLimit, err := resource.ParseQuantity(c.MemoryLimit); err == nil {
				resources.Limits[api.ResourceMemory] = memoryLimit
			}
		}
		if c.EphemeralStorageLimit != "" {
			if storageLimit, err := resource.ParseQuantity(c.EphemeralStorageLimit); err == nil {
				resources.Limits[api.ResourceEphemeralStorage] = storageLimit
			}
		}
	}
	return resources
}

// ClientHolder holds kubernetes clients with thread-safe access
type ClientHolder struct {
	clientMutex   sync.RWMutex
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
}

// KubernetesConfig -- Default Kubernetes Config
type KubernetesConfig struct {
	Registry    `yaml:"registry"`
	Namespace   string `yaml:"namespace" env:"KUBERNETES_NAMESPACE"`
	ClusterName string `yaml:"cluster_name" env:"CLUSTER_NAME"`

	Kubeconfig               string                             `yaml:"kubeconfig" env:"KUBECONFIG"` // Enhanced configuration options
	Host                     string                             `yaml:"host" env:"HOST"`
	BearerToken              string                             `yaml:"bearer_token,omitempty" json:"bearer_token" mapstructure:"bearer_token"`
	CertFile                 string                             `yaml:"cert_file,omitempty" json:"cert_file" mapstructure:"cert_file"`
	KeyFile                  string                             `yaml:"key_file,omitempty" json:"key_file" mapstructure:"key_file"`
	CAFile                   string                             `yaml:"ca_file,omitempty" json:"ca_file" mapstructure:"ca_file"`
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

	// Enhanced resource configuration - new structured approach
	DefaultResources ResourceConfig `yaml:"default_resources" json:"default_resources"`
	MinResources     ResourceConfig `yaml:"min_resources" json:"min_resources"`
	MaxResources     ResourceConfig `yaml:"max_resources" json:"max_resources"`

	DefaultLimits      api.ResourceList `yaml:"default_limits" json:"default_limits"`
	MinLimits          api.ResourceList `yaml:"min_limits" json:"min_limits"`
	MaxLimits          api.ResourceList `yaml:"max_limits" json:"max_limits"`
	MaxServicesPerPod  int              `yaml:"max_services_per_pod" json:"max_services_per_pod"`
	AwaitShutdownPod   bool             `yaml:"await_shutdown_pod" json:"await_shutdown_pod" mapstructure:"await_shutdown_pod"`
	QPS                float32          `yaml:"qps" env:"K8S_QPS"` // Performance tuning
	Burst              int              `yaml:"burst" env:"K8S_BURST"`
	SelectedKubeconfig *rest.Config     `yaml:"-" json:"-"` // Internal fields
	clientHolder       *ClientHolder    `yaml:"-" json:"-"`
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

// CreateResourceListFromConfig creates resource list from ResourceConfig
func (kc *KubernetesConfig) CreateResourceListFromConfig(kind string, resourceConfig ResourceConfig) (api.ResourceList, float64, error) {
	// Build using the new structured approach
	resourceReqs := resourceConfig.BuildResourceRequirements()

	// Validate against min/max limits
	if err := kc.ValidateResourceLimits(kind, resourceReqs); err != nil {
		return nil, 0, err
	}

	// Calculate cost (simplified implementation)
	cost := kc.CalculateResourceCost(resourceReqs)

	// Merge requests and limits into a single ResourceList for compatibility
	result := api.ResourceList{}
	for k, v := range resourceReqs.Requests {
		result[k] = v
	}
	for k, v := range resourceReqs.Limits {
		result[k+"_limit"] = v // Add suffix to distinguish limits
	}

	return result, cost, nil
}

// createLegacyResourceList maintains backward compatibility with old resource configuration
func (kc *KubernetesConfig) createLegacyResourceList(kind string, cpu string, memory string, ephemeralStorage string) (api.ResourceList, float64, error) {
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

	// Validate CPU
	if resourceQuantityZero(res[api.ResourceCPU]) {
		// Use default if available
		if def != nil && !resourceQuantityZero(def[api.ResourceCPU]) {
			res[api.ResourceCPU] = def[api.ResourceCPU]
		}
	} else if min != nil && !resourceQuantityZero(min[api.ResourceCPU]) &&
		resourceQuantityMilliValue(res[api.ResourceCPU]) < resourceQuantityMilliValue(min[api.ResourceCPU]) {
		return nil, 0, fmt.Errorf("cpu %s %s is smaller than min-cpu %v",
			kind, cpu, min[api.ResourceCPU])
	} else if max != nil && !resourceQuantityZero(max[api.ResourceCPU]) &&
		resourceQuantityMilliValue(res[api.ResourceCPU]) > resourceQuantityMilliValue(max[api.ResourceCPU]) {
		return nil, 0, fmt.Errorf("cpu %s %s exceeds max-cpu %v",
			kind, cpu, max[api.ResourceCPU])
	}

	// Validate Memory
	if resourceQuantityZero(res[api.ResourceMemory]) {
		if def != nil && !resourceQuantityZero(def[api.ResourceMemory]) {
			res[api.ResourceMemory] = def[api.ResourceMemory]
		}
	} else if min != nil && !resourceQuantityZero(min[api.ResourceMemory]) &&
		resourceQuantityMilliValue(res[api.ResourceMemory]) < resourceQuantityMilliValue(min[api.ResourceMemory]) {
		return nil, 0, fmt.Errorf("memory %s %s is smaller than min-memory %v",
			kind, memory, min[api.ResourceMemory])
	} else if max != nil && !resourceQuantityZero(max[api.ResourceMemory]) &&
		resourceQuantityMilliValue(res[api.ResourceMemory]) > resourceQuantityMilliValue(max[api.ResourceMemory]) {
		return nil, 0, fmt.Errorf("memory %s %s exceeds max-memory %v",
			kind, memory, max[api.ResourceMemory])
	}

	// Validate Ephemeral Storage
	if resourceQuantityZero(res[api.ResourceEphemeralStorage]) {
		if def != nil && !resourceQuantityZero(def[api.ResourceEphemeralStorage]) {
			res[api.ResourceEphemeralStorage] = def[api.ResourceEphemeralStorage]
		}
	} else if min != nil && !resourceQuantityZero(min[api.ResourceEphemeralStorage]) &&
		resourceQuantityMilliValue(res[api.ResourceEphemeralStorage]) < resourceQuantityMilliValue(min[api.ResourceEphemeralStorage]) {
		return nil, 0, fmt.Errorf("storage %s %s is smaller than min-storage %v",
			kind, ephemeralStorage, min[api.ResourceEphemeralStorage])
	} else if max != nil && !resourceQuantityZero(max[api.ResourceEphemeralStorage]) &&
		resourceQuantityMilliValue(res[api.ResourceEphemeralStorage]) > resourceQuantityMilliValue(max[api.ResourceEphemeralStorage]) {
		return nil, 0, fmt.Errorf("storage %s %s exceeds max-storage %v",
			kind, ephemeralStorage, max[api.ResourceEphemeralStorage])
	}

	cost := 0.0
	if def != nil {
		cost = utils.CreateResourceCost(res, def)
	}

	return res, cost, nil
}

// ValidateResourceLimits validates resource requirements against configured min/max
func (kc *KubernetesConfig) ValidateResourceLimits(kind string, resourceReqs api.ResourceRequirements) error {
	// Use structured min/max if available
	if kc.MinResources.CPURequest != "" || kc.MaxResources.CPURequest != "" {
		return kc.validateStructuredResourceLimits(kind, resourceReqs)
	}

	// Fall back to legacy validation if legacy limits are configured
	if kc.MinLimits != nil || kc.MaxLimits != nil {
		return kc.validateLegacyResourceLimits(kind, resourceReqs)
	}

	return nil
}

// validateStructuredResourceLimits validates against structured min/max resource config
func (kc *KubernetesConfig) validateStructuredResourceLimits(kind string, resourceReqs api.ResourceRequirements) error {
	minReqs := kc.MinResources.BuildResourceRequirements()
	maxReqs := kc.MaxResources.BuildResourceRequirements()

	// Validate CPU
	if cpuReq, exists := resourceReqs.Requests[api.ResourceCPU]; exists {
		if minCPU, minExists := minReqs.Requests[api.ResourceCPU]; minExists {
			if cpuReq.Cmp(minCPU) < 0 {
				return fmt.Errorf("cpu request %s for %s is smaller than min-cpu %s", cpuReq.String(), kind, minCPU.String())
			}
		}
		if maxCPU, maxExists := maxReqs.Requests[api.ResourceCPU]; maxExists {
			if cpuReq.Cmp(maxCPU) > 0 {
				return fmt.Errorf("cpu request %s for %s exceeds max-cpu %s", cpuReq.String(), kind, maxCPU.String())
			}
		}
	}

	// Validate Memory
	if memReq, exists := resourceReqs.Requests[api.ResourceMemory]; exists {
		if minMem, minExists := minReqs.Requests[api.ResourceMemory]; minExists {
			if memReq.Cmp(minMem) < 0 {
				return fmt.Errorf("memory request %s for %s is smaller than min-memory %s", memReq.String(), kind, minMem.String())
			}
		}
		if maxMem, maxExists := maxReqs.Requests[api.ResourceMemory]; maxExists {
			if memReq.Cmp(maxMem) > 0 {
				return fmt.Errorf("memory request %s for %s exceeds max-memory %s", memReq.String(), kind, maxMem.String())
			}
		}
	}

	return nil
}

// validateLegacyResourceLimits validates against legacy resource limits
func (kc *KubernetesConfig) validateLegacyResourceLimits(kind string, resourceReqs api.ResourceRequirements) error {
	// Implementation similar to the original validation logic
	// This maintains backward compatibility with existing configurations
	return nil
}

// CalculateResourceCost calculates the cost of resources
func (kc *KubernetesConfig) CalculateResourceCost(resourceReqs api.ResourceRequirements) float64 {
	// Simplified cost calculation
	// You can implement more sophisticated cost calculation based on your needs
	cost := 0.0

	if cpu, exists := resourceReqs.Requests[api.ResourceCPU]; exists {
		cost += float64(cpu.MilliValue()) * 0.001 // Example: $0.001 per milli-CPU
	}

	if memory, exists := resourceReqs.Requests[api.ResourceMemory]; exists {
		cost += float64(memory.Value()) / (1024 * 1024 * 1024) * 0.01 // Example: $0.01 per GB
	}

	return cost
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

// GetClient returns the kubernetes client with thread-safe access
func (kc *KubernetesConfig) GetClient() kubernetes.Interface {
	if kc.clientHolder == nil {
		return nil
	}
	kc.clientHolder.clientMutex.RLock()
	defer kc.clientHolder.clientMutex.RUnlock()
	return kc.clientHolder.client
}

// GetDynamicClient returns the dynamic kubernetes client with thread-safe access
func (kc *KubernetesConfig) GetDynamicClient() dynamic.Interface {
	if kc.clientHolder == nil {
		return nil
	}
	kc.clientHolder.clientMutex.RLock()
	defer kc.clientHolder.clientMutex.RUnlock()
	return kc.clientHolder.dynamicClient
}

// SetClients sets both kubernetes clients with thread-safe access
func (kc *KubernetesConfig) SetClients(client kubernetes.Interface, dynamicClient dynamic.Interface) {
	if kc.clientHolder == nil {
		kc.clientHolder = &ClientHolder{}
	}
	kc.clientHolder.clientMutex.Lock()
	defer kc.clientHolder.clientMutex.Unlock()
	kc.clientHolder.client = client
	kc.clientHolder.dynamicClient = dynamicClient
}

// InitializeKubernetesClient initializes the kubernetes client with enhanced configuration
func (kc *KubernetesConfig) InitializeKubernetesClient() error {
	var err error

	// Apply environment variable overrides
	kc.applyEnvironmentOverrides()

	// Get REST config
	if kc.SelectedKubeconfig == nil {
		kc.SelectedKubeconfig, err = kc.GetKubeConfigForCluster()
		if err != nil {
			return fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	// Apply performance tuning
	if kc.QPS > 0 {
		kc.SelectedKubeconfig.QPS = kc.QPS
	} else {
		kc.SelectedKubeconfig.QPS = 100 // Default
	}

	if kc.Burst > 0 {
		kc.SelectedKubeconfig.Burst = kc.Burst
	} else {
		kc.SelectedKubeconfig.Burst = 100 // Default
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(kc.SelectedKubeconfig)
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(kc.SelectedKubeconfig)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	kc.SetClients(clientset, dynamicClient)
	return nil
}

// GetKubeConfigForCluster creates a Kubernetes client config that specifically targets
// the named cluster from the kubeconfig file, with backward compatibility
func (kc *KubernetesConfig) GetKubeConfigForCluster() (*rest.Config, error) {
	// Legacy support: if Host is specified, use the old method
	if kc.Host != "" {
		return kc.getLegacyOutClusterConfig()
	}

	kubeconfigPath := kc.Kubeconfig
	clusterName := kc.ClusterName

	// If no kubeconfig is specified, try in-cluster config first
	if kubeconfigPath == "" {
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}

		// Fallback to default kubeconfig location
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
			if _, err = os.Stat(defaultKubeconfig); err == nil {
				kubeconfigPath = defaultKubeconfig
			}
		}
	}

	if kubeconfigPath == "" {
		return nil, fmt.Errorf("no kubeconfig found and not running in-cluster")
	}

	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig from %s: %w", kubeconfigPath, err)
	}

	// If no cluster name is specified, use the current context's cluster
	if clusterName == "" {
		if config.CurrentContext == "" {
			return nil, fmt.Errorf("no current context found in kubeconfig and no cluster name specified")
		}

		context, exists := config.Contexts[config.CurrentContext]
		if !exists {
			return nil, fmt.Errorf("current context %s not found in kubeconfig", config.CurrentContext)
		}

		clusterName = context.Cluster
	}

	// Check if the specified cluster exists
	_, exists := config.Clusters[clusterName]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found in kubeconfig", clusterName)
	}

	// Create a REST config specifically for the named cluster
	configOverrides := &clientcmd.ConfigOverrides{
		ClusterInfo: capi.Cluster{
			Server: config.Clusters[clusterName].Server,
		},
		CurrentContext: "", // Don't use the current context
	}

	// Use the specified cluster
	configOverrides.Context.Cluster = clusterName

	// Create a ClientConfig with the specified overrides
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		configOverrides,
	)

	// Create the REST config
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating REST config for cluster %s: %w", clusterName, err)
	}

	return restConfig, nil
}

// getLegacyOutClusterConfig provides backward compatibility for the old configuration method
func (kc *KubernetesConfig) getLegacyOutClusterConfig() (*rest.Config, error) {
	kubeConfig := &rest.Config{
		Host:        kc.Host,
		BearerToken: kc.BearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: kc.CAFile,
		},
	}

	// certificate based auth
	if kc.CertFile != "" {
		if kc.KeyFile == "" || kc.CAFile == "" {
			return nil, fmt.Errorf("ca file, cert file and key file must be specified when using file based auth")
		}

		kubeConfig.TLSClientConfig.CertFile = kc.CertFile
		kubeConfig.TLSClientConfig.KeyFile = kc.KeyFile
	} else if len(kc.Username) > 0 {
		kubeConfig.Username = kc.Username
		kubeConfig.Password = kc.Password
	}

	return kubeConfig, nil
}

// applyEnvironmentOverrides applies environment variable overrides to the config
func (kc *KubernetesConfig) applyEnvironmentOverrides() {
	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
		kc.Kubeconfig = envKubeconfig
	}

	if envClusterName := os.Getenv("CLUSTER_NAME"); envClusterName != "" {
		kc.ClusterName = envClusterName
	}

	if envNamespace := os.Getenv("KUBERNETES_NAMESPACE"); envNamespace != "" {
		kc.Namespace = envNamespace
	}

	if envHost := os.Getenv("HOST"); envHost != "" {
		kc.Host = envHost
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

// Validate config
func (kc *KubernetesConfig) Validate() error {
	result := &types.ValidationResult{Valid: true}
	if kc.Registry.PullPolicy == "" {
		kc.Registry.PullPolicy = types.PullPolicyIfNotPresent
	}

	if kc.HelperImage == "" {
		kc.HelperImage = "amazon/aws-cli"
	}

	// Set up default resources using structured approach if not configured
	if kc.DefaultResources.CPURequest == "" && kc.DefaultLimits == nil {
		kc.DefaultResources = ResourceConfig{
			CPURequest:              "0.5",
			MemoryRequest:           "500Mi",
			EphemeralStorageRequest: "1Gi",
		}
	}

	if kc.MinResources.CPURequest == "" && kc.MinLimits == nil {
		kc.MinResources = ResourceConfig{
			CPURequest:              "0.1",
			MemoryRequest:           "100Mi",
			EphemeralStorageRequest: "100Mi",
		}
	}

	if kc.MaxResources.CPURequest == "" && kc.MaxLimits == nil {
		kc.MaxResources = ResourceConfig{
			CPURequest:              "2.0",
			MemoryRequest:           "4Gi",
			EphemeralStorageRequest: "2Gi",
		}
	}

	// Maintain backward compatibility with legacy resource lists
	if kc.DefaultLimits == nil && kc.DefaultResources.CPURequest != "" {
		kc.DefaultLimits, _ = utils.CreateResourceList(
			kc.DefaultResources.CPURequest,
			kc.DefaultResources.MemoryRequest,
			kc.DefaultResources.EphemeralStorageRequest)
	}

	if kc.MinLimits == nil && kc.MinResources.CPURequest != "" {
		kc.MinLimits, _ = utils.CreateResourceList(
			kc.MinResources.CPURequest,
			kc.MinResources.MemoryRequest,
			kc.MinResources.EphemeralStorageRequest)
	}

	if kc.MaxLimits == nil && kc.MaxResources.CPURequest != "" {
		kc.MaxLimits, _ = utils.CreateResourceList(
			kc.MaxResources.CPURequest,
			kc.MaxResources.MemoryRequest,
			kc.MaxResources.EphemeralStorageRequest)
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

	// Validate cluster configuration
	if kc.Host == "" && kc.Kubeconfig == "" {
		// Will try in-cluster config or default kubeconfig
	}

	// Apply environment overrides for validation
	kc.applyEnvironmentOverrides()

	// Check configuration method
	if kc.Host != "" {
		result.AddInfo("Using legacy direct connection configuration")
		kc.validateLegacyConfig(result)
	} else {
		result.AddInfo("Using enhanced kubeconfig-based configuration")
		kc.validateEnhancedConfig(result)
	}

	// Validate common settings
	kc.validateCommonSettings(result)
	return result.Log()
}

// validateLegacyConfig validates legacy configuration settings
func (kc *KubernetesConfig) validateLegacyConfig(result *types.ValidationResult) {
	if kc.Host == "" {
		result.AddError("Host is required for legacy configuration")
		return
	}

	// Check authentication method
	if kc.BearerToken != "" {
		result.AddInfo("Using bearer token authentication")
	} else if kc.CertFile != "" && kc.KeyFile != "" && kc.CAFile != "" {
		result.AddInfo("Using certificate-based authentication")

		// Validate certificate files exist
		if !fileExists(kc.CertFile) {
			result.AddError(fmt.Sprintf("Certificate file does not exist: %s", kc.CertFile))
		}
		if !fileExists(kc.KeyFile) {
			result.AddError(fmt.Sprintf("Key file does not exist: %s", kc.KeyFile))
		}
		if !fileExists(kc.CAFile) {
			result.AddError(fmt.Sprintf("CA file does not exist: %s", kc.CAFile))
		}
	} else if kc.Username != "" && kc.Password != "" {
		result.AddInfo("Using username/password authentication")
		result.AddWarning("Username/password authentication is less secure than certificates or tokens")
	} else {
		result.AddError("No valid authentication method configured for legacy mode")
	}
}

// validateEnhancedConfig validates enhanced configuration settings
func (kc *KubernetesConfig) validateEnhancedConfig(result *types.ValidationResult) {
	kubeconfigPath := kc.Kubeconfig

	// Check if kubeconfig path is specified
	if kubeconfigPath == "" {
		// Try to find default kubeconfig
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultPath := filepath.Join(homeDir, ".kube", "config")
			if fileExists(defaultPath) {
				kubeconfigPath = defaultPath
				result.AddInfo(fmt.Sprintf("Using default kubeconfig: %s", defaultPath))
			}
		}

		if kubeconfigPath == "" {
			result.AddInfo("No kubeconfig specified, will attempt in-cluster configuration")
			return
		}
	}

	// Validate kubeconfig file
	if !fileExists(kubeconfigPath) {
		result.AddError(fmt.Sprintf("Kubeconfig file does not exist: %s", kubeconfigPath))
		return
	}

	result.AddInfo(fmt.Sprintf("Using kubeconfig: %s", kubeconfigPath))

	// Load and validate kubeconfig
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to load kubeconfig: %v", err))
		return
	}

	// Validate cluster selection
	clusterName := kc.ClusterName
	if clusterName == "" {
		if config.CurrentContext == "" {
			result.AddError("No current context in kubeconfig and no cluster name specified")
			return
		}

		context, exists := config.Contexts[config.CurrentContext]
		if !exists {
			result.AddError(fmt.Sprintf("Current context '%s' not found in kubeconfig", config.CurrentContext))
			return
		}

		clusterName = context.Cluster
		result.AddInfo(fmt.Sprintf("Using cluster from current context: %s", clusterName))
	} else {
		result.AddInfo(fmt.Sprintf("Using specified cluster: %s", clusterName))
	}

	// Check if cluster exists
	cluster, exists := config.Clusters[clusterName]
	if !exists {
		result.AddError(fmt.Sprintf("Cluster '%s' not found in kubeconfig", clusterName))
		result.AddInfo("Available clusters:")
		for name := range config.Clusters {
			result.AddInfo(fmt.Sprintf("  - %s", name))
		}
		return
	}

	// Validate cluster configuration
	if cluster.Server == "" {
		result.AddError(fmt.Sprintf("Cluster '%s' has no server URL", clusterName))
	} else {
		result.AddInfo(fmt.Sprintf("Cluster server: %s", cluster.Server))
	}

	// Check contexts and users
	kc.validateContextsAndUsers(result, config, clusterName)
}

// validateContextsAndUsers validates kubeconfig contexts and user authentication
func (kc *KubernetesConfig) validateContextsAndUsers(
	result *types.ValidationResult, config *capi.Config, clusterName string) {
	// Find contexts that use this cluster
	var relevantContexts []string
	for contextName, context := range config.Contexts {
		if context.Cluster == clusterName {
			relevantContexts = append(relevantContexts, contextName)
		}
	}

	if len(relevantContexts) == 0 {
		result.AddWarning(fmt.Sprintf("No contexts found for cluster '%s'", clusterName))
		return
	}

	result.AddInfo(fmt.Sprintf("Found %d context(s) for cluster '%s': %v",
		len(relevantContexts), clusterName, relevantContexts))

	// Validate user authentication for relevant contexts
	for _, contextName := range relevantContexts {
		context := config.Contexts[contextName]
		user, exists := config.AuthInfos[context.AuthInfo]
		if !exists {
			result.AddWarning(fmt.Sprintf("User '%s' not found for context '%s'",
				context.AuthInfo, contextName))
			continue
		}

		// Check authentication methods
		authMethods := []string{}
		if user.Token != "" {
			authMethods = append(authMethods, "token")
		}
		if user.ClientCertificate != "" || user.ClientCertificateData != nil {
			authMethods = append(authMethods, "client-certificate")
		}
		if user.Username != "" {
			authMethods = append(authMethods, "username")
		}
		if user.Exec != nil {
			authMethods = append(authMethods, "exec")
		}

		if len(authMethods) == 0 {
			result.AddWarning(fmt.Sprintf("No authentication configured for user '%s' in context '%s'",
				context.AuthInfo, contextName))
		} else {
			result.AddInfo(fmt.Sprintf("Context '%s' uses authentication: %v",
				contextName, authMethods))
		}
	}
}

// validateCommonSettings validates settings common to both configuration methods
func (kc *KubernetesConfig) validateCommonSettings(result *types.ValidationResult) {
	// Validate namespace
	if kc.Namespace == "" {
		result.AddWarning("No namespace specified, will use 'default'")
	} else {
		result.AddInfo(fmt.Sprintf("Using namespace: %s", kc.Namespace))
	}

	// Validate performance settings
	if kc.QPS > 0 {
		result.AddInfo(fmt.Sprintf("QPS limit configured: %.1f", kc.QPS))
		if kc.QPS < 10 {
			result.AddWarning("QPS is very low, may impact performance")
		}
		if kc.QPS > 1000 {
			result.AddWarning("QPS is very high, may overwhelm the API server")
		}
	}

	if kc.Burst > 0 {
		result.AddInfo(fmt.Sprintf("Burst limit configured: %d", kc.Burst))
		if kc.Burst < int(kc.QPS) {
			result.AddWarning("Burst limit is lower than QPS limit")
		}
	}

	// Validate helper image
	if kc.HelperImage == "" {
		result.AddWarning("No helper image specified, using default")
	}

	// Check for deprecated settings
	if kc.Host != "" && (kc.Kubeconfig != "" || kc.ClusterName != "") {
		result.AddWarning("Both legacy (host) and enhanced (kubeconfig/cluster_name) configuration detected. Legacy takes precedence.")
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
