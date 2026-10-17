package kubernetes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"plexobject.com/formicary/internal/ant_config"
	common "plexobject.com/formicary/internal/types"
	qtypes "plexobject.com/formicary/queen/types"
)

// aiSkillVars are the job variables an ai-skill request resolves (org configs + job_variables defaults).
func aiSkillVars(rawArgs string) map[string]common.VariableValue {
	v := func(s string) common.VariableValue { return common.NewVariableValue(s, false) }
	return map[string]common.VariableValue{
		"JobRetry": v("0"), "JobID": v("job-1"), "RawArgs": v(rawArgs), "DefaultTracker": v("jira"),
		"SlackChannel": v("C1"), "SlackToken": v("x"), "SlackThreadTs": v("1.2"), "AiModel": v(""),
		"GitHubOrg": v("acme"), "GitHubRepo": v("widgets"), "GitHubRepoBranch": v("main"),
		"JiraUrl": v("https://jira.example.com"), "JiraProject": v("PROJ"),
		"BitbucketWorkspace": v("ws"), "BitbucketRepo": v("repo"), "BitbucketRepoBranch": v("dev"),
		"StandupTeamMembers": v(""), "StandupLookbackHours": v("26"), "ClaudeUseBedrock": v("1"),
		"AnthropicBedrockBaseUrl": v("http://ai/bedrock"), "ClaudeSkipBedrockAuth": v("1"), "CodebaseDir": v(""),
		"CodebaseRepoUrl": v(""), "GitBranch": v(""), "MaxTurnsAdhoc": v("100"), "MaxClaudeProcessTimeout": v("13500"),
		"ExtraSkillsRepos": v(""), "AiDevToolsDebug": v(""), "ServiceImage": v(""), "ServiceName": v("skill-service"),
		"ServicePort": v("9000"), "ServiceCommand": v(""), "ServiceArgs": v(""), "ServiceEntrypoint": v(""),
		"ServiceMemoryLimit": v("2G"), "ServiceCpuRequest": v("250m"), "DockerEnabled": v(""), "DindMemoryLimit": v("8G"),
		"FormicaryPublicURL": v("https://formicary.example.com"), "RunImage": v(""),
	}
}

// buildWorkflowPod renders a task of a real workflow definition under docs/examples and builds the pod the
// Kubernetes executor would create for it (same code path as production: job definition -> executor options -> BuildPod).
func buildWorkflowPod(t *testing.T, yamlFile, taskType string, vars map[string]common.VariableValue) *api.Pod {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "examples", yamlFile))
	require.NoError(t, err)
	job, err := qtypes.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	_, opts, err := job.GetDynamicTask(taskType, vars)
	require.NoError(t, err)
	opts.Name = "frm-test-" + taskType
	// BuildPod submits the pod through the Kubernetes API; a real clientset against a recording API server captures
	// exactly what the executor would create.
	var created api.Pod
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &created)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	cli, err := k8s.NewForConfig(&rest.Config{Host: srv.URL, ContentConfig: rest.ContentConfig{ContentType: "application/json"}})
	require.NoError(t, err)
	cfg := &ant_config.AntConfig{}
	cfg.Kubernetes.AllowPrivilegeEscalation = true // as in config/formicary-k8s.yaml
	u := &Utils{config: cfg, cli: cli}
	_, _, _, _, err = u.BuildPod(context.Background(), opts, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, created.Spec.Containers, "no pod was submitted")
	return &created
}

func container(t *testing.T, pod *api.Pod, name string) api.Container {
	t.Helper()
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "container not found", "%s in %v", name, pod.Spec.Containers)
	return api.Container{}
}

func mountOf(c api.Container, path string) (api.VolumeMount, bool) {
	for _, m := range c.VolumeMounts {
		if m.MountPath == path {
			return m, true
		}
	}
	return api.VolumeMount{}, false
}

// Test_AiSkillDindPodSharesTheWorkspaceWithTheDaemon: the Docker daemon runs in the dind container, so bind mounts of
// /workspace paths only work when that container mounts the same volume as the skill container.
func Test_AiSkillDindPodSharesTheWorkspaceWithTheDaemon(t *testing.T) {
	pod := buildWorkflowPod(t, "ai-skill.yaml", "run", aiSkillVars("qa-regression https://bitbucket.org/ws/repo/pull-requests/7 --dind"))
	require.True(t, pod.Spec.HostNetwork)
	dind := container(t, pod, "docker-dind")
	main := container(t, pod, "frm-test-run")
	dindMount, ok := mountOf(dind, "/workspace")
	require.True(t, ok, "docker-dind must mount /workspace; mounts=%v", dind.VolumeMounts)
	mainMount, ok := mountOf(main, "/workspace")
	require.True(t, ok)
	require.Equal(t, mainMount.Name, dindMount.Name, "both containers must mount the same volume")
	require.NotNil(t, dind.SecurityContext)
	require.True(t, *dind.SecurityContext.Privileged)
	envOf := func(c api.Container, name string) string {
		for _, e := range c.Env {
			if e.Name == name {
				return e.Value
			}
		}
		return "<unset>"
	}
	require.Equal(t, "tcp://localhost:2375", envOf(main, "DOCKER_HOST"))
	// every container gets DOCKER_HOST; the daemon must start without it or it would be pointed at itself
	require.Equal(t, []string{"env", "-u", "DOCKER_HOST"}, dind.Command)
	require.Equal(t, []string{"dockerd-entrypoint.sh"}, dind.Args)
}

func Test_AiSkillWithoutDindHasNoDaemon(t *testing.T) {
	pod := buildWorkflowPod(t, "ai-skill.yaml", "run", aiSkillVars("ygs-ask what is new"))
	for _, c := range pod.Spec.Containers {
		require.NotEqual(t, "docker-dind", c.Name)
	}
}

func Test_AiSkillPodGetsTheCredentialSecret(t *testing.T) {
	pod := buildWorkflowPod(t, "ai-skill.yaml", "run", aiSkillVars("ygs-ask hi"))
	main := container(t, pod, "frm-test-run")
	require.NotEmpty(t, main.EnvFrom)
	require.Equal(t, "ai-dev-credentials", main.EnvFrom[0].SecretRef.Name)
}

// Test_ExportWorkflowPods writes the pods above as JSON for the Python pod tests, so they run the exact pod the
// workflow produces instead of a hand-written look-alike. Set PODSPEC_OUT_DIR to enable.
func Test_ExportWorkflowPods(t *testing.T) {
	dir := os.Getenv("PODSPEC_OUT_DIR")
	if dir == "" {
		t.Skip("PODSPEC_OUT_DIR not set")
	}
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, rawArgs := range map[string]string{
		"ai-skill-dind": "qa-regression https://bitbucket.org/ws/repo/pull-requests/7 --dind",
		"ai-skill":      "ygs-ask hi",
	} {
		pod := buildWorkflowPod(t, "ai-skill.yaml", "run", aiSkillVars(rawArgs))
		data, err := json.MarshalIndent(pod, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644))
	}
}
