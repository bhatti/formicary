package utils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_ShouldNotHaveExtraSpace(t *testing.T) {
	// GIVEN a template string
	str := `
<script type="text/javascript">
    function load_{{.Digest}}() {
        document.getElementById("log_btn_{{.Digest}}").hidden = true;
        let xmlhttp = new XMLHttpRequest();
        xmlhttp.open("GET", "{{.DashboardRawURL}}", false);
        xmlhttp.send();
        document.getElementById("logs_{{.Digest}}").textContent = xmlhttp.responseText;
    }
</script>
`

	// WHEN parsing template
	_, err := ParseTemplate(str, map[string]interface{}{"Digest": "123"})
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldFailOnNoVariables(t *testing.T) {
	// GIVEN a template string
	str := `
{{with .Account -}}
Account: {{.}}
{{- end}}
Money: {{.Money}}
{{if .Note -}}
Note: {{.Note}}
{{- end}}
`

	// WHEN parsing template
	_, err := ParseTemplate(str, map[string]interface{}{})

	// THEN it should not fail without params
	require.NoError(t, err)

	// AND it should not fail with params
	_, err = ParseTemplate(str, map[string]interface{}{"Account": "x123", "Money": 12, "Note": "ty"})
	require.NoError(t, err)
}

func Test_ShouldParseExpression(t *testing.T) {
	// GIVEN a template string
	str := `{"device_id": {{.device_id}}, "description": "
  {{if lt .t_av 30.0}}
    Current temperature is {{.t_av}}, it's normal."
  {{else if ge .t_av 30.0}}
    Current temperature is {{.t_av}}, it's high."
  {{end}}
}
`

	// WHEN parsing template
	_, err := ParseTemplate(str, map[string]interface{}{"t_av": 10.0, "device_id": "ABC"})

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldParseTemplateExcept(t *testing.T) {
	// GIVEN a template string
	str := `
task_type: init
method: SHELL
except:{{if lt .JobID 5}} true {{else}} false {{end}}
script:
  - pwd
on_completed: build
`
	// WHEN parsing template
	serTaskAfterTemplate, err := ParseTemplate(str, map[string]interface{}{"JobID": 100})
	// THEN it should not fail
	require.NoError(t, err)
	require.Contains(t, serTaskAfterTemplate, "except: false")
}

func Test_ShouldParseTemplate(t *testing.T) {
	// GIVEN a job loaded from YAML
	b, err := ioutil.ReadFile("../../fixtures/encoding-job.yaml")
	require.NoError(t, err)

	// WHEN parsing YAML for validate tag
	serTask := ParseYamlTag(string(b), "validate")

	// THEN it should find the validate tag
	require.NotEqual(t, "", serTask)

	// WHEN parsing template
	serTaskAfterTemplate, err := ParseTemplate(serTask, map[string]interface{}{"Token": "tutti", "EncodingFormat": "mp4"})
	require.NoError(t, err)

	// THEN it should match expected task headers/variables
	var task map[string]interface{}
	err = yaml.Unmarshal([]byte(serTaskAfterTemplate), &task)
	require.NoError(t, err)
	require.Contains(t, fmt.Sprintf("%v", task["headers"]), "tutti")
	require.Contains(t, fmt.Sprintf("%v", task["variables"]), "mp4")
}
