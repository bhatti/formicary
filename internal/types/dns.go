package types

import (
	"fmt"
	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
)

const (
	// DNSPolicyNone none
	DNSPolicyNone                    KubernetesDNSPolicy = "none"
	// DNSPolicyDefault default
	DNSPolicyDefault                 KubernetesDNSPolicy = "default"
	// DNSPolicyClusterFirst cluster first
	DNSPolicyClusterFirst            KubernetesDNSPolicy = "cluster-first"
	// DNSPolicyClusterFirstWithHostNet cluster first with host net
	DNSPolicyClusterFirstWithHostNet KubernetesDNSPolicy = "cluster-first-with-host-net"
)

// KubernetesDNSPolicy dns policy
type KubernetesDNSPolicy string

// Get returns one of the predefined values in kubernetes notation or an error if the value is not matched.
// If the DNSPolicy is a blank string, returns the k8s default ("ClusterFirst")
func (p KubernetesDNSPolicy) Get() (api.DNSPolicy, error) {
	const defaultPolicy = api.DNSClusterFirst

	switch p {
	case "":
		logrus.Debugf("DNSPolicy string is blank, using %q as default", defaultPolicy)
		return defaultPolicy, nil
	case DNSPolicyNone:
		return api.DNSNone, nil
	case DNSPolicyDefault:
		return api.DNSDefault, nil
	case DNSPolicyClusterFirst:
		return api.DNSClusterFirst, nil
	case DNSPolicyClusterFirstWithHostNet:
		return api.DNSClusterFirstWithHostNet, nil
	}

	return "", fmt.Errorf("unsupported kubernetes-dns-policy: %q", p)
}

// KubernetesDNSConfig dns config
type KubernetesDNSConfig struct {
	Nameservers []string                    `yaml:"nameservers" json:"nameservers"`
	Options     []KubernetesDNSConfigOption `yaml:"options" json:"options"`
	Searches    []string                    `yaml:"searches" json:"searches"`
}

// KubernetesDNSConfigOption dns config option
type KubernetesDNSConfigOption struct {
	Name  string  `yaml:"name" json:"name"`
	Value *string `yaml:"value,omitempty" json:"value,omitempty"`
}
