// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SubWorkflowConfig_InputMap_ReturnsNilWhenEmpty(t *testing.T) {
	sw := &SubWorkflowConfig{}
	m, err := sw.InputMap()
	require.NoError(t, err)
	require.Nil(t, m)
}

func Test_SubWorkflowConfig_InputMap_ReturnsNilOnNilReceiver(t *testing.T) {
	var sw *SubWorkflowConfig
	m, err := sw.InputMap()
	require.NoError(t, err)
	require.Nil(t, m)
}

func Test_SubWorkflowConfig_InputMap_BuildsMap(t *testing.T) {
	sw := &SubWorkflowConfig{
		InputParams: []SubWorkflowVariable{
			{Name: "param_a", Value: "{{ .val_a }}"},
			{Name: "param_b", Value: "literal"},
		},
	}
	m, err := sw.InputMap()
	require.NoError(t, err)
	require.Equal(t, "{{ .val_a }}", m["param_a"])
	require.Equal(t, "literal", m["param_b"])
}

func Test_SubWorkflowConfig_InputMap_RejectsDuplicateNames(t *testing.T) {
	sw := &SubWorkflowConfig{
		InputParams: []SubWorkflowVariable{
			{Name: "dup", Value: "first"},
			{Name: "dup", Value: "second"},
		},
	}
	_, err := sw.InputMap()
	require.Error(t, err)
	require.Contains(t, err.Error(), "dup")
}

func Test_SubWorkflowConfig_OutputMap_ReturnsNilWhenEmpty(t *testing.T) {
	sw := &SubWorkflowConfig{}
	m, err := sw.OutputMap()
	require.NoError(t, err)
	require.Nil(t, m)
}

func Test_SubWorkflowConfig_OutputMap_ReturnsNilOnNilReceiver(t *testing.T) {
	var sw *SubWorkflowConfig
	m, err := sw.OutputMap()
	require.NoError(t, err)
	require.Nil(t, m)
}

func Test_SubWorkflowConfig_OutputMap_BuildsMap(t *testing.T) {
	sw := &SubWorkflowConfig{
		OutputVariables: []SubWorkflowVariable{
			{Name: "row_count", Value: "etl_row_count"},
			{Name: "status", Value: "child_status"},
		},
	}
	m, err := sw.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", m["row_count"])
	require.Equal(t, "child_status", m["status"])
}

func Test_SubWorkflowConfig_OutputMap_RejectsDuplicateNames(t *testing.T) {
	sw := &SubWorkflowConfig{
		OutputVariables: []SubWorkflowVariable{
			{Name: "dup", Value: "target_a"},
			{Name: "dup", Value: "target_b"},
		},
	}
	_, err := sw.OutputMap()
	require.Error(t, err)
	require.Contains(t, err.Error(), "dup")
}
