// SPDX-License-Identifier: AGPL-3.0-or-later
package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_ApprovalPolicy_UnmarshalYAML_DurationString(t *testing.T) {
	input := `
min_approvals: 1
sla_deadline: "4h"
timeout_action: ESCALATE
`
	var p ApprovalPolicy
	require.NoError(t, yaml.Unmarshal([]byte(input), &p))
	require.Equal(t, int64(4*time.Hour), p.SLADeadline)
	require.Equal(t, TimeoutActionEscalate, p.TimeoutAction)
}

func Test_ApprovalPolicy_UnmarshalYAML_DurationStringUnquoted(t *testing.T) {
	input := `
min_approvals: 2
sla_deadline: 2h30m
`
	var p ApprovalPolicy
	require.NoError(t, yaml.Unmarshal([]byte(input), &p))
	require.Equal(t, int64(2*time.Hour+30*time.Minute), p.SLADeadline)
}

func Test_ApprovalPolicy_UnmarshalYAML_IntegerNanoseconds(t *testing.T) {
	ns := int64(time.Hour)
	input := `min_approvals: 1
sla_deadline: 3600000000000
`
	var p ApprovalPolicy
	require.NoError(t, yaml.Unmarshal([]byte(input), &p))
	require.Equal(t, ns, p.SLADeadline)
}

func Test_ApprovalPolicy_UnmarshalYAML_ZeroNoSLA(t *testing.T) {
	input := `min_approvals: 1`
	var p ApprovalPolicy
	require.NoError(t, yaml.Unmarshal([]byte(input), &p))
	require.Equal(t, int64(0), p.SLADeadline)
}

func Test_ApprovalPolicy_UnmarshalYAML_InvalidDuration(t *testing.T) {
	input := `
min_approvals: 1
sla_deadline: "notaduration"
`
	var p ApprovalPolicy
	require.Error(t, yaml.Unmarshal([]byte(input), &p))
}
