package ant_config

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldCreateValidKubernetesConfig(t *testing.T) {
	//GIVEN an ant config
	ant := newTestConfig()

	// WHEN a valid kubernetes config is initialized
	cfg := KubernetesConfig{}
	cfg.Registry.Server = "docker-registry-server"
	cfg.Registry.Username = "docker-registry-user"
	cfg.Registry.Password = "docker-registry-pass"
	cfg.Registry.PullPolicy = types.PullPolicyIfNotPresent
	cfg.Host = "kubernetes-host"
	cfg.BearerToken = "kubernetes-bearer"
	cfg.CertFile = "kubernetes-cert"
	cfg.KeyFile = "kubernetes-key"
	cfg.CAFile = "kubernetes-cafile"
	cfg.Namespace = "default"
	cfg.ServiceAccount = "my-svc-account"
	cfg.ImagePullSecrets = []string{"image-pull-secret"}
	cfg.AllowPrivilegeEscalation = false
	cfg.DNSPolicy = "none"
	user := int64(1000)
	group := int64(100)
	nonRoot := true
	cfg.PodSecurityContext = types.KubernetesPodSecurityContext{
		FSGroup:            &group,
		RunAsGroup:         &group,
		RunAsNonRoot:       &nonRoot,
		RunAsUser:          &user,
		SupplementalGroups: []int64{200, 300},
	}
	cfg.CapAdd = []string{"NET_RAW", "CAP1"}
	cfg.CapDrop = []string{"CAP2"}
	cfg.InitContainers = []types.Service{{Image: "redis:latest"}}
	ant.Kubernetes = cfg

	// THEN it should create valid containers
	require.Equal(t, 1, len(cfg.GetInitContainers()))

	// AND it should be marshalable
	_, err := yaml.Marshal(ant)
	require.NoError(t, err)
}

func Test_ShouldGetEmptyHostAliasesWhenCallingWithoutHosts(t *testing.T) {
	cfg := &KubernetesConfig{}
	require.Equal(t, 0, len(cfg.GetHostAliases()))
}

func Test_ShouldReturnNilWhenCallingGetPodSecurityContextWithoutSecurity(t *testing.T) {
	cfg := &KubernetesConfig{}
	require.Nil(t, cfg.GetPodSecurityContext())
}

func Test_ShouldReturnNilWhenCallingGetDNSConfigWithoutDNS(t *testing.T) {
	cfg := &KubernetesConfig{}
	require.Nil(t, cfg.GetDNSConfig())
}
