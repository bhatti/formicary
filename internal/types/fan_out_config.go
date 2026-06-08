// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "fmt"

// FanOutConfig configures dynamic fan-out for a single task definition.
//
// Two modes:
//   - Task fan-out (default): dispatches one TaskRequest per array item to ant workers
//     using the task's original execution method (SHELL, KUBERNETES, etc.).
//   - Job fan-out: when ForkJobType is set, spawns one child JobRequest per array item
//     using the FORK_JOB machinery, with full sub_workflow input/output mapping support.
//
// Results are aggregated back to the parent context under keys "{item_var}_{index}_{key}".
//
// Example YAML — task fan-out:
//
//	tasks:
//	  - task_type: deploy
//	    method: KUBERNETES
//	    fan_out:
//	      source: regions
//	      item_var: region
//	      max_parallel: 5
//	      fail_fast: false
//	    script:
//	      - deploy --region {{.region}}
//
// Example YAML — job fan-out (spawns child jobs):
//
//	tasks:
//	  - task_type: process
//	    method: FORK_JOB
//	    fan_out:
//	      source: datasets
//	      item_var: dataset
//	      fork_job_type: io.formicary.etl-child
//	      fork_job_version: "1.0"
//	      max_parallel: 3
//	      fail_fast: true
type FanOutConfig struct {
	// Source is the job execution context variable name that holds a JSON array.
	Source string `json:"source" yaml:"source"`
	// ItemVar is the variable name injected into each child task/job for its item value.
	ItemVar string `json:"item_var" yaml:"item_var"`
	// MaxParallel caps how many concurrent children run at a time. 0 means unlimited.
	MaxParallel int `json:"max_parallel,omitempty" yaml:"max_parallel,omitempty"`
	// FailFast cancels remaining in-flight children on the first failure.
	FailFast bool `json:"fail_fast,omitempty" yaml:"fail_fast,omitempty"`
	// ExecutionMethod is the TaskMethod used to dispatch child tasks in task fan-out mode.
	// Set automatically from the task definition's Method before overriding to FAN_OUT_JOB.
	ExecutionMethod TaskMethod `json:"execution_method,omitempty" yaml:"execution_method,omitempty"`
	// ForkJobType switches to job fan-out mode: spawns one child JobRequest per array item
	// of this registered job type. Supports sub_workflow input/output variable mapping.
	// Mutually exclusive with ExecutionMethod (task fan-out mode).
	ForkJobType string `json:"fork_job_type,omitempty" yaml:"fork_job_type,omitempty"`
	// ForkJobVersion is the optional semver of the ForkJobType to instantiate.
	ForkJobVersion string `json:"fork_job_version,omitempty" yaml:"fork_job_version,omitempty"`
	// RawScript holds the un-rendered script lines extracted from the task YAML before
	// queen-side template expansion runs. FanOutTasklet uses these as the template source
	// when rendering per-item (so {{.region}} resolves to the actual item, not "<no value>").
	RawScript []string `json:"raw_script,omitempty" yaml:"raw_script,omitempty"`
	// RawBeforeScript is the un-rendered before_script counterpart to RawScript.
	RawBeforeScript []string `json:"raw_before_script,omitempty" yaml:"raw_before_script,omitempty"`
	// RawAfterScript is the un-rendered after_script counterpart to RawScript.
	RawAfterScript []string `json:"raw_after_script,omitempty" yaml:"raw_after_script,omitempty"`
}

// IsJobFanOut reports whether this config uses job fan-out mode (ForkJobType is set).
func (f *FanOutConfig) IsJobFanOut() bool {
	return f != nil && f.ForkJobType != ""
}

// Validate checks FanOutConfig fields.
func (f *FanOutConfig) Validate() error {
	if f == nil {
		return nil
	}
	if f.Source == "" {
		return fmt.Errorf("fan_out.source is required")
	}
	if len(f.Source) > 200 {
		return fmt.Errorf("fan_out.source is too long (max 200 chars)")
	}
	if f.ItemVar == "" {
		return fmt.Errorf("fan_out.item_var is required")
	}
	if len(f.ItemVar) > 100 {
		return fmt.Errorf("fan_out.item_var is too long (max 100 chars)")
	}
	if f.MaxParallel < 0 {
		return fmt.Errorf("fan_out.max_parallel must be >= 0 (0 = unlimited)")
	}
	if len(f.ForkJobType) > 200 {
		return fmt.Errorf("fan_out.fork_job_type is too long (max 200 chars)")
	}
	return nil
}
