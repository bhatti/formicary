package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateDnsConfigEmpty(t *testing.T) {
	// Given dns config
	d := KubernetesDNSPolicy("")

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.NoError(t, err)
}

func Test_ShouldCreateDnsConfigUnknown(t *testing.T) {
	// Given dns config
	d := KubernetesDNSPolicy("Unknown")

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.Error(t, err)
}

func Test_ShouldCreateDnsConfigDefault(t *testing.T) {
	// Given dns config
	d := DNSPolicyDefault

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.NoError(t, err)
}

func Test_ShouldCreateDnsConfigNone(t *testing.T) {
	// Given dns config
	d := DNSPolicyNone

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.NoError(t, err)
}

func Test_ShouldCreateDnsConfigClusterFirst(t *testing.T) {
	// Given dns config
	d := DNSPolicyClusterFirst

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.NoError(t, err)
}

func Test_ShouldCreateDnsConfigClusterFirstHostNet(t *testing.T) {
	// Given dns config
	d := DNSPolicyClusterFirstWithHostNet

	// WHEN accessing policy
	// THEN it should return expected values
	_, err := d.Get()
	require.NoError(t, err)
}
