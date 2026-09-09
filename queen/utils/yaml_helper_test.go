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
