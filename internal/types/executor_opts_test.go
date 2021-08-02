package types

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"testing"
)

func Test_ShouldValidateExecutorOpts(t *testing.T) {
	// GIVEN
	// WHEN a valid executor-options is created
	opts := newTestExecutorOptions()
	err := opts.Validate()
	// THEN validation should not fail
	require.NoError(t, err)
	_, err = yaml.Marshal(opts)
	// AND marshaling should not fail
	require.NoError(t, err)
	require.NotEqual(t, "", opts.String())
}

func Test_ShouldGetAffinityForExecutorOpts(t *testing.T) {
	// GIVEN
	// WHEN a valid executor-options is created
	opts := newTestExecutorOptions()
	err := opts.Validate()
	// THEN validation should not fail
	require.NoError(t, err)
	_, err = yaml.Marshal(opts)
	// AND marshaling should not fail
	require.NoError(t, err)
	require.NotEqual(t, "", opts.String())
}

func newTestExecutorOptions() *ExecutorOptions {
	opts := NewExecutorOptions("task1", Kubernetes)
	opts.Environment["ENV1"] = "VAL1"
	opts.MainContainer.Image = "image1"
	opts.MainContainer.Volumes = KubernetesVolumes{
		HostPaths: []KubernetesHostPath{{
			Name:      "m1",
			MountPath: "/shared",
			HostPath:  "/host/shared",
		}},
		PVCs: []KubernetesPVC{{
			Name:      "m2",
			MountPath: "/mnt/sh1",
		}},
		ConfigMaps: []KubernetesConfigMap{{
			Name:      "m3",
			MountPath: "/nt/sh2",
			Items:     map[string]string{"item1": "val1"},
		}},
		Secrets: []KubernetesSecret{{
			Name:      "m4",
			MountPath: "/mypath",
			ReadOnly:  true,
			Items:     map[string]string{"mysecret": "/mnt/path/mysecret"},
			SubPath:   "mypath",
		}},
		EmptyDirs: []KubernetesEmptyDir{{
			Name:      "m4",
			MountPath: "/nt/sh3",
		}},
	}
	opts.MainContainer.VolumeDriver = "voldriver"
	opts.MainContainer.VolumesFrom = []string{"from"}
	opts.MainContainer.Devices = []string{"devices"}
	opts.MainContainer.BindDirectory = "/shared"
	opts.MainContainer.CPULimit = "1"
	opts.MainContainer.CPURequest = "500m"
	opts.MainContainer.EphemeralStorageRequest = "1Gi"
	opts.MainContainer.EphemeralStorageLimit = "2Gi"
	opts.MainContainer.MemoryLimit = "1Gi"
	opts.MainContainer.MemoryRequest = "1Gi"
	opts.HelperContainer.Image = "image2"
	opts.HelperContainer.Volumes = KubernetesVolumes{}
	opts.HelperContainer.VolumeDriver = "voldriver"
	opts.HelperContainer.VolumesFrom = []string{"from"}
	opts.HelperContainer.Devices = []string{"devices"}
	opts.HelperContainer.BindDirectory = "/helepr"
	opts.HelperContainer.CPULimit = "1"
	opts.HelperContainer.CPURequest = "500m"
	opts.HelperContainer.MemoryLimit = "1Gi"
	opts.HelperContainer.MemoryRequest = "1Gi"
	opts.Services = []Service{{
		Name:       "svc",
		Image:      "image3",
		Command:    []string{"cmd1"},
		Entrypoint: []string{"bash"},
		Ports:      []Port{{Name: "name", Number: 123}},
		Volumes: &KubernetesVolumes{
			HostPaths: []KubernetesHostPath{{
				Name:      "s1",
				MountPath: "/shared",
				HostPath:  "/host/shared",
			}},
			PVCs: []KubernetesPVC{{
				Name:      "s2",
				MountPath: "/mnt/sh1",
			}},
			ConfigMaps: []KubernetesConfigMap{{
				Name:      "s3",
				MountPath: "/nt/sh2",
				Items:     map[string]string{"item1": "val1"},
			}},
			Secrets: []KubernetesSecret{{
				Name:      "s4",
				MountPath: "/nt/sh3",
				Items:     map[string]string{"item1": "val1"},
			}},
			EmptyDirs: []KubernetesEmptyDir{{
				Name:      "s4",
				MountPath: "/nt/sh3",
			}},
		},
		CPULimit:                "1",
		CPURequest:              "500m",
		MemoryLimit:             "1Gi",
		MemoryRequest:           "1Gi",
		EphemeralStorageRequest: "1Gi",
		EphemeralStorageLimit:   "2Gi",
	}}
	opts.Privileged = true
	opts.NodeTolerations = map[string]string{"myrole": "NoSchedule", "empty": "PreferNoSchedule"}
	opts.NodeSelector["formicary"] = "true"
	opts.PodLabels["lab1"] = "true"
	opts.PodAnnotations["ann"] = "true"
	opts.NetworkMode = "mod1"
	opts.HostNetwork = true
	opts.Affinity = &KubernetesNodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &NodeSelector{
			NodeSelectorTerms: []NodeSelectorTerm{{
				MatchExpressions: []NodeSelectorRequirement{{
					Key:      "key1",
					Operator: "IN",
					Values:   []string{"val1"},
				}},
				MatchFields: []NodeSelectorRequirement{{
					Key:      "key2",
					Operator: "IN",
					Values:   []string{"val2"},
				}},
			}},
		},
		PreferredDuringSchedulingIgnoredDuringExecution: []PreferredSchedulingTerm{{
			Weight: 1,
			Preference: NodeSelectorTerm{
				MatchExpressions: []NodeSelectorRequirement{{
					Key:      "key3",
					Operator: "IN",
					Values:   []string{"val3"},
				}},
				MatchFields: []NodeSelectorRequirement{{
					Key:      "key4",
					Operator: "IN",
					Values:   []string{"val4"},
				}},
			}},
		},
	}
	return opts
}
