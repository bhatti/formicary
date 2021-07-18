package types

import "fmt"

// PullPolicy alias for pull policy
type PullPolicy string

const (
	// PullPolicyAlways always pull
	PullPolicyAlways       PullPolicy = "always"
	// PullPolicyNever never pull
	PullPolicyNever        PullPolicy = "never"
	// PullPolicyIfNotPresent pull if not present
	PullPolicyIfNotPresent PullPolicy = "if-not-present"
)

// Always pull
func (p PullPolicy) Always() bool {
	v, _ := p.GetDockerPullPolicy()
	return v == PullPolicyAlways
}

// IfNotPresent pull
func (p PullPolicy) IfNotPresent() bool {
	v, _ := p.GetDockerPullPolicy()
	return v == PullPolicyIfNotPresent
}

// GetDockerPullPolicy pull policy
func (p PullPolicy) GetDockerPullPolicy() (PullPolicy, error) {
	if p == "" {
		return PullPolicyIfNotPresent, nil
	}

	// Verify pull policy
	if p != PullPolicyNever &&
		p != PullPolicyIfNotPresent &&
		p != PullPolicyAlways {
		return "", fmt.Errorf("unsupported docker-pull-policy: %v", p)
	}
	return p, nil
}

// GetKubernetesPullPolicy policy
func (p PullPolicy) GetKubernetesPullPolicy() PullPolicy {
	switch {
	case p == PullPolicyAlways:
		return "Always"
	case p == PullPolicyNever:
		return "Never"
	case p == PullPolicyIfNotPresent:
		return "IfNotPresent"
	}
	return "IfNotPresent"
}
