package utils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"

	"gopkg.in/yaml.v3"
	domain "plexobject.com/formicary/internal/types"
)

// Test parse yaml
func Test_ShouldParseYaml(t *testing.T) {
	// GIVEN a yaml job config
	b, err := ioutil.ReadFile("../../fixtures/basic-job.yaml")
	require.NoError(t, err)
	m := make(map[string]interface{})

	// WHEN unmarshalling yaml into map
	err = yaml.Unmarshal(b, &m)
	// THEN it should not fail
	require.NoError(t, err)
}

// Test parse yaml tag for task
func Test_ShouldParseTaskYamlTag(t *testing.T) {
	// GIVEN a yaml job config
	b, err := ioutil.ReadFile("../../fixtures/basic-job.yaml")
	require.NoError(t, err)
	taskNames := []string{"task1", "task2", "task3"}
	for _, name := range taskNames {
		// WHEN parsing yaml for the task
		ser := ParseYamlTag(string(b), fmt.Sprintf("task_type: %s", name))
		task := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(ser), &task)

		// THEN it should not fail and match task
		require.NoError(t, err)
		require.Equal(t, name, task["task_type"])
	}
}

// Test parse yaml tag for config
func Test_ShouldParseConfigYamlTag(t *testing.T) {
	// GIVEN a yaml job configs
	files := []string{
		"../../fixtures/test_job.yaml",
		"../../fixtures/basic-job.yaml",
		"../../fixtures/encoding-job.yaml",
		"../../fixtures/kube-build.yaml",
	}
	numConfigs := []int{3, 3, 3, 3}
	for i, file := range files {
		b, err := ioutil.ReadFile(file)
		require.NoError(t, err)
		// WHEN parsing yaml for the job-variables
		ser := ParseYamlTag(string(b), "job_variables:")
		var cfg interface{}
		err = yaml.Unmarshal([]byte(ser), &cfg)

		// THEN it should not fail and contains expected variables
		require.NoError(t, err)
		m, err := ParseNameValueConfigs(cfg)
		require.NoError(t, err)
		require.Equal(t, len(m), numConfigs[i])
	}
}

func Test_ShouldParseImagePullPolicyFromTaskYaml(t *testing.T) {
	jobYaml := `
job_type: test-pull-policy
tasks:
- task_type: audit-prs
  method: KUBERNETES
  report_stdout: true
  host_network: true
  working_dir: /workspace
  container:
    image: plexobject/ai-dev-tools:latest
    image_pull_policy: Always
    memory_limit: 8G
  environment:
    FOO: bar
  on_completed: done
- task_type: done
  method: SHELL
  script:
    - echo done
`
	serData := ParseYamlTag(jobYaml, "task_type: audit-prs")
	require.NotEmpty(t, serData, "ParseYamlTag should return non-empty for audit-prs")
	t.Logf("Extracted YAML fragment:\n%s", serData)

	require.Contains(t, serData, "image_pull_policy")

	opts := domain.NewExecutorOptions("", "")
	err := yaml.Unmarshal([]byte(serData), opts)
	require.NoError(t, err)
	require.Equal(t, "plexobject/ai-dev-tools:latest", opts.MainContainer.Image)
	require.Equal(t, "Always", opts.MainContainer.ImagePullPolicy,
		"ImagePullPolicy should survive ParseYamlTag + Unmarshal round-trip")
}

// Test_ShouldParseTaskWithTemplateConditional verifies that a task containing a
// {{if}}...{{end}} block at column 0 (common for optional services blocks) is fully
// extracted by ParseYamlTag — specifically that fields AFTER the conditional block
// (container, script, environment) are included in the output.
func Test_ShouldParseTaskWithTemplateConditional(t *testing.T) {
	jobYaml := `job_type: ai-contract-test
tasks:
- task_type: record
  method: KUBERNETES
  timeout: 15m
  host_network: true
  working_dir: /workspace
{{if .ServiceImage}}
  services:
    - name: svc
      image: "{{.ServiceImage}}"
{{end}}
  container:
    image: plexobject/ai-dev-tools:latest
    image_pull_policy: Always
    memory_limit: 4G
  environment:
    PR_NUMBER: "{{.PRNumber}}"
  script:
    - python -m scripts.mq.clone_pr
    - python -m scripts.contract.record
  on_completed: fuzz
- task_type: done
  method: SHELL
  script:
    - echo done
`
	// ParseYamlTag must include the template conditional block AND all fields that
	// follow it (container, environment, script) — not terminate at {{if .ServiceImage}}.
	ser := ParseYamlTag(jobYaml, "task_type: record")
	require.NotEmpty(t, ser)
	require.Contains(t, ser, "script", "script must be extracted even when preceded by {{if}} block")
	require.Contains(t, ser, "container", "container must be extracted even when preceded by {{if}} block")
	require.Contains(t, ser, "{{if .ServiceImage}}", "template directive must be preserved verbatim")
	require.Contains(t, ser, "{{end}}", "{{end}} directive must be preserved verbatim")

	// The extracted fragment must not include the next task.
	require.NotContains(t, ser, "task_type: done")
}

// Test_ShouldNotTerminateTaskExtractionOnTemplateEnd verifies that a lone {{end}}
// at column 0 does not prematurely stop extraction.
func Test_ShouldNotTerminateTaskExtractionOnTemplateEnd(t *testing.T) {
	jobYaml := `job_type: test
tasks:
- task_type: worker
  method: KUBERNETES
  host_network: true
{{if .UseCache}}
  cache:
    key: deps
    paths: [vendor/]
{{end}}
  container:
    image: alpine:latest
  script:
    - echo hello
- task_type: done
  method: SHELL
  script:
    - echo done
`
	ser := ParseYamlTag(jobYaml, "task_type: worker")
	require.Contains(t, ser, "script")
	require.Contains(t, ser, "container")
	require.NotContains(t, ser, "task_type: done")
}
