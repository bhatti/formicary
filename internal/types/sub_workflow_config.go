// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "fmt"

// SubWorkflowVariable is a name/value pair used in sub_workflow param and output mappings.
// For input_params: Name is the child job param name, Value is a Go template expression
// resolved against the parent's non-secret execution-context variables.
// For output_variables: Name is the child execution-context variable name,
// Value is the parent execution-context variable name to promote it under.
type SubWorkflowVariable struct {
	// Name is the variable name (child param name for input; child context key for output).
	Name string `json:"name" yaml:"name"`
	// Value is the template expression (input_params) or parent rename target (output_variables).
	Value string `json:"value" yaml:"value"`
}

// SubWorkflowConfig configures child workflow composition for FORK_JOB tasks.
// All forked children are always cascade-cancelled when their parent is cancelled.
type SubWorkflowConfig struct {
	// InputParams defines which params to pass to the child job request.
	// Each entry: Name = child job param name, Value = Go template expression resolved
	// against the parent's non-secret execution-context variables.
	// When empty no parent variables are forwarded.
	InputParams []SubWorkflowVariable `json:"input_params,omitempty" yaml:"input_params,omitempty"`
	// OutputVariables defines which child execution-context variables to promote
	// to the parent context. Each entry: Name = child context key,
	// Value = parent context key to publish under.
	// When empty all child context variables are copied verbatim.
	OutputVariables []SubWorkflowVariable `json:"output_variables,omitempty" yaml:"output_variables,omitempty"`
	// WaitForCompletion makes the FORK_JOB task block until the child completes,
	// combining the fork and await steps into a single task.
	WaitForCompletion bool `json:"wait_for_completion,omitempty" yaml:"wait_for_completion,omitempty"`
}

// InputMap returns InputParams as a map[childParam]templateExpr for fast lookup.
// Duplicate names are rejected: returns an error if any Name appears more than once.
func (c *SubWorkflowConfig) InputMap() (map[string]string, error) {
	if c == nil || len(c.InputParams) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(c.InputParams))
	for _, v := range c.InputParams {
		if _, exists := m[v.Name]; exists {
			return nil, fmt.Errorf("sub_workflow.input_params: duplicate name %q", v.Name)
		}
		m[v.Name] = v.Value
	}
	return m, nil
}

// OutputMap returns OutputVariables as a map[childKey]parentKey for fast lookup.
// Duplicate names are rejected: returns an error if any Name appears more than once.
func (c *SubWorkflowConfig) OutputMap() (map[string]string, error) {
	if c == nil || len(c.OutputVariables) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(c.OutputVariables))
	for _, v := range c.OutputVariables {
		if _, exists := m[v.Name]; exists {
			return nil, fmt.Errorf("sub_workflow.output_variables: duplicate name %q", v.Name)
		}
		m[v.Name] = v.Value
	}
	return m, nil
}
