package utils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"

	"gopkg.in/yaml.v3"
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
