package types

import (
	"errors"
	"fmt"
	"strings"

	"plexobject.com/formicary/internal/utils"

	api "k8s.io/api/core/v1"
)

// AntID constant for label
const AntID = "AntID"

// UserID constant for label
const UserID = "UserID"

// OrgID constant for label
const OrgID = "OrganizationID"

// RequestID constant for label
const RequestID = "RequestID"

// NodeTolerations alias for tolerations
type NodeTolerations map[string]string

// Get converts tolerations into kubernetes type
func (nt NodeTolerations) Get() []api.Toleration {
	var tolerations []api.Toleration

	for toleration, effect := range nt {
		newToleration := api.Toleration{
			Effect: api.TaintEffect(effect),
		}

		if strings.Contains(toleration, "=") {
			parts := strings.Split(toleration, "=")
			newToleration.Key = parts[0]
			if len(parts) > 1 {
				newToleration.Value = parts[1]
			}
			newToleration.Operator = api.TolerationOpEqual
		} else {
			newToleration.Key = toleration
			newToleration.Operator = api.TolerationOpExists
		}

		tolerations = append(tolerations, newToleration)
	}

	return tolerations
}

// EnvironmentMap alias
type EnvironmentMap map[string]string

// NewEnvironmentMap constructor
func NewEnvironmentMap() EnvironmentMap {
	m := make(map[string]string)
	return m
}

// AsArray to string array
func (em EnvironmentMap) AsArray() []string {
	env := make([]string, 0)
	for k, v := range em {
		env = append(env, fmt.Sprintf("%v=%v", k, v))
	}
	return env
}

// AddFromEnvCommand updates env from command
func (em EnvironmentMap) AddFromEnvCommand(cmd string) bool {
	if strings.HasPrefix(cmd, "env ") {
		parts := strings.Split(cmd, "=")
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0][4:])
			value := strings.TrimSpace(parts[1])
			em[name] = value
			return true
		}
	}
	return false
}

// HelperContainerKey key for context
const HelperContainerKey = "HelperContainer"

// ExecutorOptions options for executor
type ExecutorOptions struct {
	Name                       string                  `json:"name" yaml:"name"`
	Method                     TaskMethod              `json:"method" yaml:"method"`
	Environment                EnvironmentMap          `json:"environment,omitempty" yaml:"environment,omitempty"`
	HelperEnvironment          EnvironmentMap          `json:"helper_environment,omitempty" yaml:"helper_environment,omitempty"`
	WorkingDirectory           string                  `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	ArtifactsDirectory         string                  `json:"artifacts_dir,omitempty" yaml:"artifacts_dir,omitempty"`
	Artifacts                  ArtifactsConfig         `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	CacheDirectory             string                  `json:"cache_dir,omitempty" yaml:"cache_dir,omitempty"`
	Cache                      CacheConfig             `json:"cache,omitempty" yaml:"cache,omitempty"`
	DependentArtifactIDs       []string                `json:"dependent_artifact_ids,omitempty" yaml:"dependent_artifact_ids,omitempty"`
	MainContainer              *ContainerDefinition    `json:"container,omitempty" yaml:"container,omitempty"`
	HelperContainer            *ContainerDefinition    `json:"helper,omitempty" yaml:"helper,omitempty"`
	Services                   []Service               `json:"services,omitempty" yaml:"services,omitempty"`
	Privileged                 bool                    `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Affinity                   *KubernetesNodeAffinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	NodeSelector               map[string]string       `json:"node_selector,omitempty" yaml:"node_selector,omitempty"`
	NodeTolerations            NodeTolerations         `json:"node_tolerations,omitempty" yaml:"node_tolerations,omitempty"`
	PodLabels                  map[string]string       `json:"pod_labels,omitempty" yaml:"pod_labels,omitempty"`
	PodAnnotations             map[string]string       `json:"pod_annotations,omitempty" yaml:"pod_annotations,omitempty"`
	NetworkMode                string                  `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	HostNetwork                bool                    `json:"host_network,omitempty" yaml:"host_network,omitempty"`
	Headers                    map[string]string       `yaml:"headers,omitempty" json:"headers"`
	QueryParams                map[string]string       `yaml:"query,omitempty" json:"query"`
	ReportStdout               bool                    `yaml:"report_stdout,omitempty" json:"report_stdout"`
	MessagingRequestQueue      string                  `json:"messaging_request_queue,omitempty" yaml:"messaging_request_queue,omitempty"`
	MessagingReplyQueue        string                  `json:"messaging_reply_queue,omitempty" yaml:"messaging_reply_queue,omitempty"`
	ForkJobType                string                  `json:"fork_job_type,omitempty" yaml:"fork_job_type,omitempty"`
	ForkJobVersion             string                  `json:"fork_job_version,omitempty" yaml:"fork_job_version,omitempty"`
	ArtifactKeyPrefix          string                  `json:"artifact_key_prefix,omitempty" yaml:"artifact_key_prefix,omitempty"`
	AwaitForkedTasks           []string                `json:"await_forked_tasks,omitempty" yaml:"await_forked_tasks,omitempty"`
	CostFactor                 float64                 `json:"cost_factor,omitempty" yaml:"cost_factor,omitempty"`
	ExecuteCommandWithoutShell bool                    `json:"execute_command_without_shell,omitempty" yaml:"execute_command_without_shell,omitempty"`
	Debug                      bool                    `json:"debug,omitempty" yaml:"debug,omitempty"`
}

// NewExecutorOptions constructor
func NewExecutorOptions(name string, method TaskMethod) *ExecutorOptions {
	return &ExecutorOptions{
		Name:                 utils.MakeDNS1123Compatible(name),
		Method:               method,
		Environment:          NewEnvironmentMap(),
		HelperEnvironment:    NewEnvironmentMap(),
		Artifacts:            NewArtifactsConfig(),
		Cache:                NewCacheConfig(),
		DependentArtifactIDs: make([]string, 0),
		NodeTolerations:      make(map[string]string),
		NodeSelector:         make(map[string]string),
		PodLabels:            make(map[string]string),
		PodAnnotations:       make(map[string]string),
		Services:             make([]Service, 0),
		MainContainer:        NewContainerDefinition(),
		HelperContainer:      NewContainerDefinition(),
	}
}

// Validate validates options for executor
func (opt *ExecutorOptions) Validate() error {
	if opt.Method == "" {
		return errors.New("method is not specified in executor-options")
	}
	if !opt.Method.IsValid() {
		return fmt.Errorf("method %s is not supported", opt.Method)
	}
	if opt.Method == ForkJob && opt.ForkJobType == "" {
		return fmt.Errorf("forkJobType is not specified")
	}
	if opt.Method == AwaitForkedJob && (opt.AwaitForkedTasks == nil || len(opt.AwaitForkedTasks) == 0) {
		return fmt.Errorf("waitJobTasks is not specified")
	} else if (opt.Method == Kubernetes || opt.Method == Docker) && opt.MainContainer.Image == "" {
		//return fmt.Errorf("image is not specified")
	}

	if opt.Artifacts.Paths == nil {
		opt.Artifacts.Paths = make([]string, 0)
	}
	if opt.Cache.Paths == nil {
		opt.Cache.Paths = make([]string, 0)
	}
	if opt.Cache.KeyPaths == nil {
		opt.Cache.KeyPaths = make([]string, 0)
	}
	if opt.DependentArtifactIDs == nil {
		opt.DependentArtifactIDs = make([]string, 0)
	}
	if opt.Environment == nil {
		opt.Environment = NewEnvironmentMap()
	}
	if opt.HelperEnvironment == nil {
		opt.HelperEnvironment = NewEnvironmentMap()
	}
	if opt.NodeSelector == nil {
		opt.NodeSelector = make(map[string]string)
	}
	if opt.NodeTolerations == nil {
		opt.NodeTolerations = make(map[string]string)
	}
	if opt.PodLabels == nil {
		opt.PodLabels = make(map[string]string)
	}
	if opt.PodAnnotations == nil {
		opt.PodAnnotations = make(map[string]string)
	}
	return nil
}

func (opt *ExecutorOptions) String() string {
	return fmt.Sprintf(
		"Name=%s Labels=%v DependentArtifacts=%d Main=%v",
		opt.Name,
		opt.PodLabels,
		len(opt.DependentArtifactIDs),
		opt.MainContainer,
	)
}

// GetAffinity affinity
func (opt *ExecutorOptions) GetAffinity() *api.Affinity {
	var affinity api.Affinity

	if opt.Affinity != nil {
		affinity.NodeAffinity = opt.GetNodeAffinity()
	} else {
		return nil
	}
	return &affinity
}

// GetNodeAffinity node affinity
func (opt *ExecutorOptions) GetNodeAffinity() *api.NodeAffinity {
	var nodeAffinity api.NodeAffinity

	if opt.Affinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution =
			opt.Affinity.RequiredDuringSchedulingIgnoredDuringExecution.GetNodeSelector()
	}

	for _, preferred := range opt.Affinity.PreferredDuringSchedulingIgnoredDuringExecution {
		nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution =
			append(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, preferred.GetPreferredSchedulingTerm())
	}
	return &nodeAffinity
}
