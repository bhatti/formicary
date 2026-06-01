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

// mockJobCountQuerier is a simple in-process querier used in template tests.
// It implements the full JobTemplateHelper interface.
type mockJobCountQuerier struct {
	counts      map[string]map[string]int64
	errorMsg    string // if non-empty, CountByJobTypeAndStateStrings returns this error
	submitError string // if non-empty, SubmitJob returns this error
	nextID      uint64 // auto-incremented ID returned by SubmitJob
	submitted   []submittedJob
}

type submittedJob struct {
	jobType     string
	description string
	params      map[string]string
}

func (m *mockJobCountQuerier) CountByJobTypeAndStateStrings(jobType string, states ...string) (int64, error) {
	if m.errorMsg != "" {
		return 0, fmt.Errorf("%s", m.errorMsg)
	}
	byState, ok := m.counts[jobType]
	if !ok {
		return 0, nil
	}
	if len(states) == 0 {
		var total int64
		for _, v := range byState {
			total += v
		}
		return total, nil
	}
	var total int64
	for _, s := range states {
		total += byState[s]
	}
	return total, nil
}

func (m *mockJobCountQuerier) SubmitJob(jobType string, description string, params map[string]string) (string, error) {
	if m.submitError != "" {
		return "", fmt.Errorf("%s", m.submitError)
	}
	m.nextID++
	id := fmt.Sprintf("01MOCK%012d", m.nextID)
	m.submitted = append(m.submitted, submittedJob{jobType: jobType, description: description, params: params})
	return id, nil
}

func Test_ShouldCountByJobTypeAndStateInTemplate(t *testing.T) {
	querier := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {
				"PENDING":   3,
				"EXECUTING": 2,
			},
		},
	}

	// WHEN template counts PENDING + EXECUTING for ai-gh-implement
	result, err := ParseTemplateWithQuerier(
		`{{CountByJobTypeAndState "ai-gh-implement" "PENDING" "EXECUTING"}}`,
		map[string]interface{}{}, querier)
	require.NoError(t, err)
	require.Equal(t, "5", result)

	// WHEN template uses ge to check capacity (5 < 10 → false)
	result, err = ParseTemplateWithQuerier(
		`{{if ge (CountByJobTypeAndState "ai-gh-implement" "PENDING,EXECUTING") 10}}true{{else}}false{{end}}`,
		map[string]interface{}{}, querier)
	require.NoError(t, err)
	require.Equal(t, "false", result)

	// WHEN no querier is provided, CountByJobTypeAndState returns 0 safely
	result, err = ParseTemplateWithQuerier(
		`{{CountByJobTypeAndState "ai-gh-implement" "PENDING"}}`,
		map[string]interface{}{}, nil)
	require.NoError(t, err)
	require.Equal(t, "0", result)

	// WHEN the querier returns an error (e.g. DB failure), the template still evaluates
	// (returning 0), but does not panic or propagate the error to the template engine.
	errQuerier := &mockJobCountQuerier{errorMsg: "db connection lost"}
	result, err = ParseTemplateWithQuerier(
		`{{if ge (CountByJobTypeAndState "ai-gh-implement" "PENDING") 10}}true{{else}}false{{end}}`,
		map[string]interface{}{}, errQuerier)
	require.NoError(t, err)
	// With 0 returned on error, the capacity check is NOT triggered (fail-open).
	// This is intentional: a DB blip should not block new job submissions.
	require.Equal(t, "false", result)
}

func Test_ShouldSubmitJobViaTemplate(t *testing.T) {
	// GIVEN a helper with no pre-existing jobs
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{},
		nextID: 0,
	}

	// WHEN the template calls SubmitJob
	result, err := ParseTemplateWithQuerier(
		`{{SubmitJob "my-job" "desc" "Key=val"}}`,
		map[string]interface{}{}, helper)

	// THEN it should return a non-empty job ID string
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// AND the job was submitted with the correct params
	require.Len(t, helper.submitted, 1)
	require.Equal(t, "my-job", helper.submitted[0].jobType)
	require.Equal(t, "desc", helper.submitted[0].description)
	require.Equal(t, "val", helper.submitted[0].params["Key"])
}

func Test_ShouldSubmitJobIfCapacityAvailable(t *testing.T) {
	// Use the {{if lt (CountByJobTypeAndState ...)}}{{SubmitJob ...}}{{end}} pattern
	// rather than the removed SubmitJobIfCapacity function.
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {
				"PENDING":   2,
				"EXECUTING": 1,
			},
		},
		nextID: 10,
	}

	// WHEN 3 in-flight jobs < max 5, SubmitJob should fire
	result, err := ParseTemplateWithQuerier(
		`{{if lt (CountByJobTypeAndState "ai-gh-implement" "PENDING" "EXECUTING") 5}}{{SubmitJob "ai-gh-implement" "issue #42" "IssueNumber=42"}}{{end}}`,
		map[string]interface{}{}, helper)

	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.Len(t, helper.submitted, 1)
	require.Equal(t, "ai-gh-implement", helper.submitted[0].jobType)
	require.Equal(t, "42", helper.submitted[0].params["IssueNumber"])
}

func Test_ShouldNotSubmitJobWhenAtCapacity(t *testing.T) {
	// Use the {{if lt ...}}{{SubmitJob ...}}{{end}} pattern.
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {
				"PENDING":   3,
				"EXECUTING": 2,
			},
		},
		nextID: 20,
	}

	// WHEN 5 in-flight jobs == max 5, SubmitJob should NOT fire
	result, err := ParseTemplateWithQuerier(
		`{{if lt (CountByJobTypeAndState "ai-gh-implement" "PENDING" "EXECUTING") 5}}{{SubmitJob "ai-gh-implement" "issue #99" "IssueNumber=99"}}{{end}}`,
		map[string]interface{}{}, helper)

	require.NoError(t, err)
	require.Equal(t, "", result)
	require.Len(t, helper.submitted, 0)
}

func Test_ShouldSubmitJobsFromJSON(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{},
		nextID: 100,
	}

	// Generic items — fields become params directly; _description sets job description.
	itemsJSON := `[{"OrderID":"1","Product":"Widget","_description":"Order 1: Widget"},` +
		`{"OrderID":"2","Product":"Gadget","_description":"Order 2: Gadget"}]`
	data := map[string]interface{}{
		"ItemsJSON": itemsJSON,
	}

	result, err := ParseTemplateWithQuerier(
		`{{SubmitJobsFromJSON "process-order" .ItemsJSON}}`,
		data, helper)

	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.Len(t, helper.submitted, 2)
	require.Equal(t, "process-order", helper.submitted[0].jobType)
	require.Equal(t, "1", helper.submitted[0].params["OrderID"])
	require.Equal(t, "Widget", helper.submitted[0].params["Product"])
	require.Equal(t, "Order 1: Widget", helper.submitted[0].description)
	require.Equal(t, "2", helper.submitted[1].params["OrderID"])
}

// pickerSubmitJobsTemplate matches the submit-jobs task environment in ai-gh-issue-picker.yaml.
// ItemsJSON contains pre-built params (including _description and _user_key) from gather-issues.
const pickerSubmitJobsTemplate = `{{if .ItemsJSON}}{{SubmitJobsFromJSON "ai-gh-implement" .ItemsJSON}}{{end}}`

// pickerSkipIfTemplate matches the skip_if field of ai-gh-issue-picker.yaml.
const pickerSkipIfTemplate = `{{if ge (CountByJobTypeAndState "ai-gh-implement" "PENDING") 10}} true {{end}}`

// Test_ShouldPickerSubmitJobsWhenItemsJSONPresent simulates what happens when gather-issues
// succeeds and exports ItemsJSON. The submit-jobs task template submits one job per item.
func Test_ShouldPickerSubmitJobsWhenIssuesJSONPresent(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {"PENDING": 1},
		},
		nextID: 200,
	}

	// Items already have all params built by the gather-issues script.
	itemsJSON := `[` +
		`{"IssueNumber":"10","IssueTitle":"Implement feature X","GitHubOrg":"myorg","GitHubRepo":"myrepo","Nonce":"abc1","_description":"#10: Implement feature X","_user_key":"ai-gh-implement-myorg-myrepo-10"},` +
		`{"IssueNumber":"11","IssueTitle":"Fix bug Y","GitHubOrg":"myorg","GitHubRepo":"myrepo","Nonce":"abc2","_description":"#11: Fix bug Y","_user_key":"ai-gh-implement-myorg-myrepo-11"}` +
		`]`
	ctx := map[string]interface{}{
		"ItemsJSON": itemsJSON,
	}

	result, err := ParseTemplateWithQuerier(pickerSubmitJobsTemplate, ctx, helper)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	require.Len(t, helper.submitted, 2)
	require.Equal(t, "ai-gh-implement", helper.submitted[0].jobType)
	require.Equal(t, "10", helper.submitted[0].params["IssueNumber"])
	require.Equal(t, "Implement feature X", helper.submitted[0].params["IssueTitle"])
	require.Equal(t, "myorg", helper.submitted[0].params["GitHubOrg"])
	require.Equal(t, "myrepo", helper.submitted[0].params["GitHubRepo"])
	require.Equal(t, "#10: Implement feature X", helper.submitted[0].description)
	require.Equal(t, "11", helper.submitted[1].params["IssueNumber"])
}

// Test_ShouldPickerSkipSubmitWhenItemsJSONMissing verifies the nil guard: when ItemsJSON is
// absent (gather-issues failed), the template produces empty output without error.
func Test_ShouldPickerSkipSubmitWhenIssuesJSONMissing(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{},
		nextID: 300,
	}

	ctx := map[string]interface{}{} // no ItemsJSON

	result, err := ParseTemplateWithQuerier(pickerSubmitJobsTemplate, ctx, helper)
	require.NoError(t, err)
	require.Empty(t, result)
	require.Len(t, helper.submitted, 0)
}

// Test_ShouldPickerSkipIfAtCapacity verifies the skip_if template: when PENDING >= 10,
// skip_if renders "true" causing the queen to skip the entire picker run.
func Test_ShouldPickerSkipIfAtCapacity(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {"PENDING": 10},
		},
	}

	result, err := ParseTemplateWithQuerier(pickerSkipIfTemplate, map[string]interface{}{}, helper)
	require.NoError(t, err)
	require.Contains(t, result, "true")
}

// Test_ShouldPickerNotSkipWhenBelowCapacity verifies the skip_if template: when PENDING < 10,
// skip_if renders empty and the picker runs normally.
func Test_ShouldPickerNotSkipWhenBelowCapacity(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{
			"ai-gh-implement": {"PENDING": 9},
		},
	}

	result, err := ParseTemplateWithQuerier(pickerSkipIfTemplate, map[string]interface{}{}, helper)
	require.NoError(t, err)
	require.NotContains(t, result, "true")
}

// Test_ShouldPickerJobDescriptionContainIssueInfo verifies that _description from the item
// is used as the job request description in the Formicary dashboard.
func Test_ShouldPickerJobDescriptionContainIssueInfo(t *testing.T) {
	helper := &mockJobCountQuerier{
		counts: map[string]map[string]int64{},
		nextID: 400,
	}

	itemsJSON := `[{"IssueNumber":"42","IssueTitle":"Add dark mode","_description":"#42: Add dark mode","_user_key":"ai-gh-implement-corp-app-42"}]`
	ctx := map[string]interface{}{
		"ItemsJSON": itemsJSON,
	}

	_, err := ParseTemplateWithQuerier(pickerSubmitJobsTemplate, ctx, helper)
	require.NoError(t, err)
	require.Len(t, helper.submitted, 1)

	require.Contains(t, helper.submitted[0].description, "42")
	require.Contains(t, helper.submitted[0].description, "Add dark mode")
	require.Equal(t, "42", helper.submitted[0].params["IssueNumber"])
}
