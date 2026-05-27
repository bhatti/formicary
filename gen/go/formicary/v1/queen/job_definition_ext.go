// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated JobDefinition, TaskDefinition,
// JobDefinitionConfig, JobDefinitionVariable, TaskDefinitionVariable.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorhill/cronexpr"
	yaml "gopkg.in/yaml.v3"

	common "plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"
	qutils "plexobject.com/formicary/queen/utils"
)

const maxConfigValueLength = 1000
const maxTasksPerJob = 100
const keyRequiredParams = "required_params"
const keyResources = "resources"
const keyHeaders = "headers"
const keyScript = "script"
const keyBeforeScript = "before_script"
const keyAfterScript = "after_script"
const keyTags = "tags"
const keyExcept = "except"
const keyJobVersion = "job_version"
const keyDeps = "dependencies"
const keyArtifacts = "artifact_ids"

var rangeRegex, _ = regexp.Compile("{{[-\\s]*range")

// Package-level lookup caches for JobDefinition tasks.
// Keyed by JobDefinition.Id → *cutils.SafeMap (taskType → *TaskDefinition).
var jdTaskCache sync.Map

func jdTasks(id string) *cutils.SafeMap {
	if v, ok := jdTaskCache.Load(id); ok {
		return v.(*cutils.SafeMap)
	}
	m := cutils.NewSafeMap()
	jdTaskCache.Store(id, m)
	return m
}

// Package-level lookup caches for TaskDefinition variables.
// Keyed by TaskDefinition.Id → *cutils.SafeMap (name → *TaskDefinitionVariable).
var tdVarCache sync.Map

func tdVars(id string) *cutils.SafeMap {
	if v, ok := tdVarCache.Load(id); ok {
		return v.(*cutils.SafeMap)
	}
	m := cutils.NewSafeMap()
	tdVarCache.Store(id, m)
	return m
}

// tdTransient holds transient fields that proto structs cannot store.
// Keyed by TaskDefinition.Id.
type tdTransientFields struct {
	Dependencies []string
	ArtifactIDs  []string
	JobVersion   string
}

var tdTransientCache sync.Map

func tdTransient(id string) *tdTransientFields {
	if v, ok := tdTransientCache.Load(id); ok {
		return v.(*tdTransientFields)
	}
	f := &tdTransientFields{}
	tdTransientCache.Store(id, f)
	return f
}

// ──────────────────────────────────────────────────────────────────────────────
// GORM TableName
// ──────────────────────────────────────────────────────────────────────────────

func (*JobDefinition) TableName() string        { return "formicary_job_definitions" }
func (*JobDefinitionConfig) TableName() string  { return "formicary_job_definition_configs" }
func (*JobDefinitionVariable) TableName() string { return "formicary_job_definition_variables" }
func (*TaskDefinition) TableName() string       { return "formicary_task_definitions" }
func (*TaskDefinitionVariable) TableName() string { return "formicary_task_definition_variables" }

// ──────────────────────────────────────────────────────────────────────────────
// JobDefinition helpers
// ──────────────────────────────────────────────────────────────────────────────

// Editable returns true if the given user/org may modify this definition.
func (jd *JobDefinition) Editable(userID string, organizationID string) bool {
	if jd.OrganizationId != "" || organizationID != "" {
		return jd.OrganizationId == organizationID
	}
	return jd.UserId == userID
}

// Enabled returns true when the definition is not disabled.
func (jd *JobDefinition) Enabled() bool { return !jd.Disabled }

// JobTypeAndVersion returns "type:version" or just "type".
func (jd *JobDefinition) JobTypeAndVersion() string {
	if jd.SemVersion == "" {
		return jd.JobType
	}
	return jd.JobType + ":" + jd.SemVersion
}

// ShortUserID returns first 8 chars of UserId.
func (jd *JobDefinition) ShortUserID() string {
	if len(jd.UserId) > 8 {
		return jd.UserId[0:8] + "..."
	}
	return jd.UserId
}

// ShortJobType returns first 12 chars of JobType.
func (jd *JobDefinition) ShortJobType() string {
	if len(jd.JobType) > 12 {
		return jd.JobType[0:12] + "..."
	}
	return jd.JobType
}

// GetUserJobTypeKey returns a canonical key combining org/user and job type.
func (jd *JobDefinition) GetUserJobTypeKey() string {
	return getUserJobTypeKey(jd.OrganizationId, jd.UserId, jd.JobType, jd.SemVersion)
}

// Summary returns a short human-readable description.
func (jd *JobDefinition) Summary() string {
	return fmt.Sprintf("JobDefinition[type=%s ver=%s tasks=%d]",
		jd.JobType, jd.SemVersion, len(jd.Tasks))
}

// Yaml returns the raw YAML, marshaling if not cached.
func (jd *JobDefinition) Yaml() string {
	if jd.RawYaml != "" {
		return jd.RawYaml
	}
	b, _ := yaml.Marshal(jd)
	return string(b)
}

// UpdateRawYaml serializes the definition to its RawYaml field.
func (jd *JobDefinition) UpdateRawYaml() {
	b, _ := yaml.Marshal(jd)
	jd.RawYaml = string(b)
}

// TaskNames returns space-separated task type names.
func (jd *JobDefinition) TaskNames() string {
	var b strings.Builder
	for _, t := range jd.Tasks {
		b.WriteString(t.TaskType)
		b.WriteString(" ")
	}
	return b.String()
}

// VariablesString returns a compact text representation of variables.
func (jd *JobDefinition) VariablesString() string {
	var b strings.Builder
	sort.Slice(jd.Variables, func(i, j int) bool { return jd.Variables[i].Name < jd.Variables[j].Name })
	for _, c := range jd.Variables {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	return b.String()
}

// GetDelayBetweenRetries returns a non-zero delay with jitter.
func (jd *JobDefinition) GetDelayBetweenRetries() time.Duration {
	if jd.DelayBetweenRetriesNs <= 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(10)); err == nil {
			return time.Second * time.Duration(n.Int64()+5)
		}
		return time.Second * 10
	}
	return time.Duration(jd.DelayBetweenRetriesNs)
}

// GetPauseTime returns pause time with jitter when not set.
func (jd *JobDefinition) GetPauseTime() time.Duration {
	if jd.PauseTimeNs <= 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(10)); err == nil {
			return time.Second * time.Duration(n.Int64()+30)
		}
		return time.Second * 30
	}
	return time.Duration(jd.PauseTimeNs)
}

// ──────────────────────────────────────────────────────────────────────────────
// Task management
// ──────────────────────────────────────────────────────────────────────────────

// AddTasks adds multiple tasks, returning self for chaining.
func (jd *JobDefinition) AddTasks(tasks ...*TaskDefinition) *JobDefinition {
	for _, t := range tasks {
		jd.AddTask(t)
	}
	return jd
}

// AddTask adds or replaces a task in the definition.
func (jd *JobDefinition) AddTask(task *TaskDefinition) *TaskDefinition {
	lk := jdTasks(jd.Id)
	old := lk.GetObject(task.TaskType)
	if old == nil {
		jd.Tasks = append(jd.Tasks, task)
		maxOrder := int32(0)
		for _, t := range jd.Tasks {
			if t.TaskOrder > maxOrder {
				maxOrder = t.TaskOrder
			}
		}
		task.TaskOrder = maxOrder + 1
	} else {
		task.TaskOrder = old.(*TaskDefinition).TaskOrder
	}
	lk.SetObject(task.TaskType, task)
	return task
}

// GetTask finds a task by type name.
func (jd *JobDefinition) GetTask(taskType string) *TaskDefinition {
	obj := jdTasks(jd.Id).GetObject(taskType)
	if obj == nil {
		return nil
	}
	return obj.(*TaskDefinition)
}

// GetFirstTask returns the entry-point task, validating DAG reachability.
func (jd *JobDefinition) GetFirstTask() (*TaskDefinition, error) {
	onExitTypes, err := jd.validateReachableTasks()
	if err != nil {
		return nil, err
	}
	return jd.validateFirstTask(onExitTypes)
}

// GetLastTask returns the last task (the one with no exit codes).
func (jd *JobDefinition) GetLastTask() (last *TaskDefinition) {
	for _, t := range jd.Tasks {
		if len(t.OnExitCodeJson) == 0 {
			last = t
		}
	}
	return
}

// GetLastAlwaysRunTasks returns tasks marked AlwaysRun.
func (jd *JobDefinition) GetLastAlwaysRunTasks() []*TaskDefinition {
	out := make([]*TaskDefinition, 0)
	for _, t := range jd.Tasks {
		if t.AlwaysRun {
			out = append(out, t)
		}
	}
	return out
}

// GetNextTask returns the next task to run based on exit code routing.
func (jd *JobDefinition) GetNextTask(
	task *TaskDefinition,
	taskStatus common.RequestState,
	exitCode string,
) (nextTaskDef *TaskDefinition, parent bool, err error) {
	onExit := task.loadOnExitCodeMap()
	if len(onExit) == 0 {
		return nil, false, nil
	}

	nextTaskName := onExit[common.NewRequestState(exitCode)]

	switch common.NewRequestState(nextTaskName) {
	case common.EXECUTING:
		return task, false, nil
	case common.PAUSE_JOB, common.PAUSED:
		nextTaskName = onExit[common.PAUSE_JOB]
	case common.COMPLETED:
		nextTaskName = onExit[common.COMPLETED]
	case common.FAILED, common.FATAL:
		nextTaskName = onExit[common.FAILED]
	}

	if nextTaskDef = jd.GetTask(nextTaskName); nextTaskDef != nil {
		return nextTaskDef, true, nil
	}

	nextTaskDef = jd.GetTask(onExit[common.NewRequestState(string(taskStatus))])
	if nextTaskDef != nil {
		return nextTaskDef, false, nil
	}

	if task.AllowFailure {
		return jd.GetTask(onExit[common.COMPLETED]), false, nil
	}
	return nil, false, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Config management
// ──────────────────────────────────────────────────────────────────────────────

// AddConfig adds or updates a named config on the definition.
func (jd *JobDefinition) AddConfig(name string, value interface{}, secret bool) (*JobDefinitionConfig, error) {
	cfg, err := newJobDefinitionConfig(name, value, secret)
	if err != nil {
		return nil, err
	}
	for _, next := range jd.Configs {
		if next.Name == name {
			next.Value = cfg.Value
			next.Kind = cfg.Kind
			next.Secret = cfg.Secret
			return next, nil
		}
	}
	cfg.JobDefinitionId = jd.Id
	jd.Configs = append(jd.Configs, cfg)
	return cfg, nil
}

// RemoveConfig removes a named config, returning true if found.
func (jd *JobDefinition) RemoveConfig(name string) bool {
	for i, c := range jd.Configs {
		if c.Name == name {
			jd.Configs = append(jd.Configs[:i], jd.Configs[i+1:]...)
			return true
		}
	}
	return false
}

// GetConfig returns a named config, or nil.
func (jd *JobDefinition) GetConfig(name string) *JobDefinitionConfig {
	for _, c := range jd.Configs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// GetConfigByID returns a config by its ID, or nil.
func (jd *JobDefinition) GetConfigByID(configID string) *JobDefinitionConfig {
	for _, c := range jd.Configs {
		if c.Id == configID {
			return c
		}
	}
	return nil
}

// GetConfigString returns a config value as string.
func (jd *JobDefinition) GetConfigString(name string) string {
	for _, c := range jd.Configs {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// ──────────────────────────────────────────────────────────────────────────────
// Variable management
// ──────────────────────────────────────────────────────────────────────────────

// AddVariable adds or updates a named variable.
func (jd *JobDefinition) AddVariable(name string, value interface{}) (*JobDefinitionVariable, error) {
	variable, err := newJobDefinitionVariable(name, value)
	if err != nil {
		return nil, err
	}
	variable.JobDefinitionId = jd.Id

	for _, next := range jd.Variables {
		if next.Name == name {
			next.Value = variable.Value
			next.Kind = variable.Kind
			return next, nil
		}
	}
	jd.Variables = append(jd.Variables, variable)
	return variable, nil
}

// RemoveVariable removes a named variable, returning true if found.
func (jd *JobDefinition) RemoveVariable(name string) bool {
	for i, v := range jd.Variables {
		if v.Name == name {
			jd.Variables = append(jd.Variables[:i], jd.Variables[i+1:]...)
			return true
		}
	}
	return false
}

// GetDynamicConfigAndVariables returns merged config and variable values.
func (jd *JobDefinition) GetDynamicConfigAndVariables(data interface{}) map[string]common.VariableValue {
	res := make(map[string]common.VariableValue)
	res["JobID"] = common.NewVariableValue("0", false)
	res["JobType"] = common.NewVariableValue(jd.JobType, false)
	res["JobRetry"] = common.NewVariableValue(0, false)
	res["JobElapsedSecs"] = common.NewVariableValue(0, false)
	for _, v := range jd.Variables {
		if vv, err := v.GetVariableValue(); err == nil {
			res[v.Name] = vv
		}
	}
	if nvVars, err := jd.parseNameValueVariables(data); err == nil {
		for k, v := range nvVars {
			res[k] = common.NewVariableValue(v, false)
		}
	}
	for _, c := range jd.Configs {
		if vv, err := c.GetVariableValue(); err == nil {
			res[c.Name] = vv
		}
	}
	return res
}

func (jd *JobDefinition) parseNameValueVariables(data interface{}) (map[string]interface{}, error) {
	if !jd.UsesTemplate || jd.NameValueVariablesJson == "" {
		var m map[string]interface{}
		if jd.NameValueVariablesJson != "" {
			if err := json.Unmarshal([]byte(jd.NameValueVariablesJson), &m); err != nil {
				return nil, err
			}
		}
		return m, nil
	}
	serVars := qutils.ParseYamlTag(jd.RawYaml, "job_variables:")
	if serVars == "" {
		return nil, nil
	}
	parsed, err := qutils.ParseTemplate(serVars, data)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := yaml.Unmarshal([]byte(parsed), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Versioning
// ──────────────────────────────────────────────────────────────────────────────

// SemanticVersionType classifies a semantic version string.
type SemanticVersionType int

const (
	InvalidSemanticVersion    SemanticVersionType = 0
	ValidSemanticVersion      SemanticVersionType = 1
	ValidSemanticDevRcVersion SemanticVersionType = 2
)

// NormalizedSemVersion pads each numeric component to 9 digits for sort order.
func (jd *JobDefinition) NormalizedSemVersion() string {
	if jd.SemVersion == "" {
		return ""
	}
	semHyphenated := strings.Split(jd.SemVersion, "-")
	if len(semHyphenated) == 1 {
		semHyphenated = strings.Split(jd.SemVersion, "rc")
	}
	if len(semHyphenated) == 1 {
		semHyphenated = strings.Split(jd.SemVersion, "dev")
	}
	var sb strings.Builder
	parts := strings.Split(semHyphenated[0], ".")
	for i, p := range parts {
		if n, err := strconv.Atoi(p); err == nil && n >= 0 && i < 3 {
			if i > 0 {
				sb.WriteString(".")
			}
			sb.WriteString(fmt.Sprintf("%09d", n))
		}
	}
	return sb.String()
}

// CheckSemVersion validates the semantic version format.
func (jd *JobDefinition) CheckSemVersion() (SemanticVersionType, error) {
	ver := strings.Split(jd.SemVersion, ".")
	if len(ver) < 2 {
		return InvalidSemanticVersion, fmt.Errorf(
			"no major/minor plugin version, version '%s' must use semantic versioning like 1.2 or 1.0.1", jd.SemVersion)
	}
	for i := 0; i < len(ver)-1; i++ {
		if digit, err := strconv.Atoi(ver[i]); err != nil || digit < 0 {
			return InvalidSemanticVersion, fmt.Errorf(
				"non-numeric major/minor version (%s) in '%s'", ver[i], jd.SemVersion)
		}
	}
	last := ver[len(ver)-1]
	if digit, err := strconv.Atoi(last); err == nil && digit >= 0 {
		return ValidSemanticVersion, nil
	}
	numericDevRCPattern := regexp.MustCompile(`^\d*-?(dev|rc)-?\d*$`)
	if !numericDevRCPattern.MatchString(last) {
		return InvalidSemanticVersion, fmt.Errorf(
			"bad last digit (%s) in version '%s', must be like 1.2, 1.0.1, or 1.0.1-dev", last, jd.SemVersion)
	}
	return ValidSemanticDevRcVersion, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Cron / scheduling
// ──────────────────────────────────────────────────────────────────────────────

// CronAndScheduleTime returns a human-readable cron description with next run.
func (jd *JobDefinition) CronAndScheduleTime() string {
	if jd.CronTrigger == "" {
		return ""
	}
	nextTime, _ := jd.GetCronScheduleTimeAndUserKey()
	if nextTime == nil {
		return ""
	}
	return fmt.Sprintf("%s (Next: %s)", jd.CronTrigger, nextTime.Format(time.RFC3339))
}

// GetCronScheduleTimeAndUserKey returns the next scheduled time and a unique cron key.
func (jd *JobDefinition) GetCronScheduleTimeAndUserKey() (*time.Time, string) {
	if jd.Disabled {
		return nil, ""
	}
	orgIDOrUser := jd.OrganizationId
	if orgIDOrUser == "" {
		orgIDOrUser = jd.UserId
	}
	return GetCronScheduleTimeAndUserKey(orgIDOrUser, jd.JobType, jd.CronTrigger)
}

// GetCronScheduleTimeAndUserKey is a package-level helper for cron scheduling.
func GetCronScheduleTimeAndUserKey(orgIDOrUserID, jobType, cronTrigger string) (*time.Time, string) {
	if cronTrigger == "" {
		return nil, ""
	}
	nextTime := cronexpr.MustParse(cronTrigger).Next(time.Now())
	if nextTime.IsZero() {
		return nil, ""
	}
	return &nextTime, fmt.Sprintf("%s-%s-%s", orgIDOrUserID, jobType, nextTime.Format(time.RFC3339))
}

// DeleteFilteredCronJobs returns true when a cron job should delete filtered results.
func (jd *JobDefinition) DeleteFilteredCronJobs() bool {
	if jd.CronTrigger == "" {
		return false
	}
	var nv map[string]interface{}
	if jd.NameValueVariablesJson != "" {
		_ = json.Unmarshal([]byte(jd.NameValueVariablesJson), &nv)
	}
	if nv != nil {
		if v, ok := nv["DeleteFilteredCronJobs"]; ok {
			return v == true || v == "true"
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// AfterLoad
// ──────────────────────────────────────────────────────────────────────────────

// AfterLoad initializes transient state after loading from DB.
func (jd *JobDefinition) AfterLoad(key []byte) error {
	lk := jdTasks(jd.Id)
	// rebuild task lookup
	for _, t := range jd.Tasks {
		if err := t.AfterLoad(); err != nil {
			return err
		}
		lk.SetObject(t.TaskType, t)
	}

	// decrypt configs
	for _, cfg := range jd.Configs {
		if err := cfg.Decrypt(key); err != nil {
			return err
		}
	}

	// sort tasks by order
	sort.Slice(jd.Tasks, func(i, j int) bool { return jd.Tasks[i].TaskOrder < jd.Tasks[j].TaskOrder })
	return jd.Validate()
}

// ──────────────────────────────────────────────────────────────────────────────
// Validate / ValidateBeforeSave
// ──────────────────────────────────────────────────────────────────────────────

// Validate validates the job definition.
func (jd *JobDefinition) Validate() (err error) {
	jd.Errors = make(map[string]string)
	if jd.JobType == "" {
		err = errors.New("jobType is not specified")
		jd.Errors["JobType"] = err.Error()
		return err
	}
	if len(jd.JobType) > 100 {
		err = errors.New("jobType is too big")
		jd.Errors["JobType"] = err.Error()
		return err
	}
	if len(jd.Url) > 200 {
		err = errors.New("URL is too big")
		jd.Errors["URL"] = err.Error()
		return err
	}
	if len(jd.Description) > 500 {
		err = errors.New("description is too big")
		jd.Errors["Description"] = err.Error()
		return err
	}
	if len(jd.Platform) > 100 {
		err = errors.New("platform is too big")
		jd.Errors["Platform"] = err.Error()
		return err
	}
	if len(jd.Tags) > 1000 {
		err = errors.New("tags size is too big")
		jd.Errors["Tags"] = err.Error()
		return err
	}
	if jd.PublicPlugin && len(strings.Split(jd.JobType, ".")) < 3 {
		err = errors.New("the plugin jobType must start with an organization bundle id such as io.formicary.test-job")
		jd.Errors["PublicPlugin"] = err.Error()
		return err
	}
	if jd.SemVersion != "" || jd.PublicPlugin {
		if _, err = jd.CheckSemVersion(); err != nil {
			jd.Errors["SemVersion"] = err.Error()
			return err
		}
	}
	if jd.CronTrigger != "" && cronexpr.MustParse(jd.CronTrigger).Next(time.Now()).IsZero() {
		err = fmt.Errorf("cron expression %s is invalid", jd.CronTrigger)
		jd.Errors["CronTrigger"] = err.Error()
		return err
	}
	if len(jd.Tasks) == 0 {
		err = fmt.Errorf("tasks are not specified for %v", jd.JobType)
		jd.Errors["Tasks"] = err.Error()
		return err
	}
	if len(jd.Tasks) > maxTasksPerJob {
		err = fmt.Errorf("number of tasks cannot exceed %d for %v", maxTasksPerJob, jd.JobType)
		jd.Errors["Tasks"] = err.Error()
		return err
	}
	for _, t := range jd.Tasks {
		if err = t.Validate(); err != nil {
			jd.Errors["Tasks"] = err.Error()
			return err
		}
	}
	jd.Tags = jd.buildTags()
	jd.Methods = jd.buildMethods()
	if jd.Methods == "" {
		err = errors.New("methods not specified for job-definition")
		jd.Errors["Methods"] = err.Error()
		return err
	}
	if jd.RawYaml == "" {
		err = errors.New("raw-yaml not specified")
		jd.Errors["RawYaml"] = err.Error()
		return err
	}
	jd.UsesTemplate = strings.Contains(jd.RawYaml, "{{") && strings.Contains(jd.RawYaml, "}}")
	if jd.MaxConcurrency <= 1 {
		jd.MaxConcurrency = 3
	}
	if err = jd.validateTaskExitCodes(); err != nil {
		jd.Errors["Tasks"] = err.Error()
		return err
	}
	// validate notify email recipients
	if jd.NotifyJson != "" {
		var notify map[common.NotifyChannel]common.JobNotifyConfig
		if jsonErr := json.Unmarshal([]byte(jd.NotifyJson), &notify); jsonErr == nil {
			for source, cfg := range notify {
				if source == common.EmailChannel {
					if emailErr := cfg.ValidateEmail(); emailErr != nil {
						jd.Errors["EmailChannel"] = emailErr.Error()
						return emailErr
					}
				}
			}
		}
	}
	if _, err = jd.GetFirstTask(); err != nil {
		jd.Errors["Tasks"] = err.Error()
		return err
	}
	return nil
}

// ValidateBeforeSave validates and serializes the job definition before persistence.
func (jd *JobDefinition) ValidateBeforeSave(key []byte) error {
	if err := jd.Validate(); err != nil {
		return err
	}
	if err := jd.addVariablesFromNameValueVariables(); err != nil {
		return err
	}
	for _, cfg := range jd.Configs {
		if err := cfg.ValidateBeforeSave(key); err != nil {
			return err
		}
	}
	for _, t := range jd.Tasks {
		if err := t.ValidateBeforeSave(); err != nil {
			return err
		}
	}
	// serialize Notify map → NotifySerialized for DB
	if jd.NotifyJson != "" {
		jd.NotifySerialized = jd.NotifyJson
	}
	return nil
}

// Equals compares another job definition for logical equality.
func (jd *JobDefinition) Equals(other *JobDefinition) error {
	if other == nil {
		return errors.New("other job is nil")
	}
	if err := jd.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}
	if jd.JobType != other.JobType {
		return fmt.Errorf("expected jobType %v but was %v", jd.JobType, other.JobType)
	}
	if len(jd.Variables) != len(other.Variables) {
		return fmt.Errorf("expected %d job variables but was %d", len(jd.Variables), len(other.Variables))
	}
	if jd.VariablesString() != other.VariablesString() {
		return fmt.Errorf("expected job variables %s but was %s", jd.VariablesString(), other.VariablesString())
	}
	if len(jd.Tasks) != len(other.Tasks) {
		return fmt.Errorf("expected %d tasks but was %d", len(jd.Tasks), len(other.Tasks))
	}
	otherLk := jdTasks(other.Id)
	for _, t := range other.Tasks {
		local := otherLk.GetObject(t.TaskType)
		if local == nil {
			local = jd.GetTask(t.TaskType)
		}
		if local == nil {
			return fmt.Errorf("failed to find task for %s", t.TaskType)
		}
		if err := t.Equals(local.(*TaskDefinition)); err != nil {
			return err
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Private helpers
// ──────────────────────────────────────────────────────────────────────────────

func (jd *JobDefinition) addVariablesFromNameValueVariables() error {
	if jd.NameValueVariablesJson == "" {
		return nil
	}
	var nameValueVars map[string]interface{}
	if err := json.Unmarshal([]byte(jd.NameValueVariablesJson), &nameValueVars); err != nil {
		return err
	}
	for n, v := range nameValueVars {
		if _, err := jd.AddVariable(n, v); err != nil {
			return err
		}
	}
	return nil
}

func (jd *JobDefinition) validateFirstTask(onExitTypes map[string]bool) (*TaskDefinition, error) {
	var firstTask *TaskDefinition
	for _, t := range jd.Tasks {
		if !onExitTypes[t.TaskType] && firstTask == nil &&
			(len(jd.Tasks) == 1 || t.HasNext()) {
			firstTask = t
		} else if !onExitTypes[t.TaskType] && firstTask != nil {
			return nil, fmt.Errorf("task %v is not reachable, first task %v", t.TaskType, firstTask.TaskType)
		}
	}
	if firstTask == nil {
		return nil, fmt.Errorf("no first task found with onExitTypes %v", onExitTypes)
	}
	return firstTask, nil
}

func (jd *JobDefinition) validateReachableTasks() (map[string]bool, error) {
	onExitTypes := make(map[string]bool)
	reservedExitCodes := map[string]bool{
		string(common.FATAL):        true,
		string(common.RESTART_JOB):  true,
		string(common.PAUSE_JOB):    true,
		string(common.RESTART_TASK): true,
		string(common.EXECUTING):    true,
		string(common.FAILED):       true,
		string(common.COMPLETED):    true,
	}
	lk := jdTasks(jd.Id)
	for _, t := range jd.Tasks {
		onExit := t.loadOnExitCodeMap()
		for _, next := range onExit {
			if next == "" {
				return nil, fmt.Errorf("empty task target for %v", t.TaskType)
			}
			if strings.HasPrefix(next, "ERR_") || reservedExitCodes[next] {
				continue
			}
			if lk.GetObject(next) == nil {
				return nil, fmt.Errorf("task '%s' refers to '%s' on-exit but it's not defined", t.TaskType, next)
			}
			onExitTypes[next] = true
		}
	}
	return onExitTypes, nil
}

func (jd *JobDefinition) validateTaskExitCodes() error {
	lk := jdTasks(jd.Id)
	for _, t := range jd.Tasks {
		if err := t.Validate(); err != nil {
			return err
		}
		if lk.GetObject(t.TaskType) != nil {
			return fmt.Errorf("duplicate tasks of type %v", t.TaskType)
		}
		onExit := t.loadOnExitCodeMap()
		if t.OnCompleted != "" {
			onExit[common.COMPLETED] = t.OnCompleted
		}
		if t.OnFailed != "" {
			onExit[common.FAILED] = t.OnFailed
		}
		if t.AllowFailure && onExit[common.FAILED] == "" && onExit[common.COMPLETED] != "" {
			onExit[common.FAILED] = onExit[common.COMPLETED]
		}
		if b, err := json.Marshal(stringMap(onExit)); err == nil {
			t.OnExitCodeJson = string(b)
		}
		lk.SetObject(t.TaskType, t)
	}
	tasksWithoutExitCodes := 0
	for _, t := range jd.Tasks {
		if !t.HasNext() {
			tasksWithoutExitCodes++
		}
	}
	if tasksWithoutExitCodes == 0 {
		return errors.New("tasks are not valid and could not find starting task")
	}
	return nil
}

func (jd *JobDefinition) buildMethods() string {
	methods := make(map[string]bool)
	for _, t := range jd.Tasks {
		if t.Method != "" {
			methods[t.Method] = true
		}
	}
	var buf strings.Builder
	for k := range methods {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(k)
	}
	return buf.String()
}

func (jd *JobDefinition) buildTags() string {
	taskTags := make(map[string]bool)
	for _, t := range jd.Tasks {
		for _, tag := range t.Tags {
			taskTags[tag] = true
		}
	}
	var buf strings.Builder
	for k := range taskTags {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k)
	}
	return buf.String()
}

// stringMap converts map[RequestState]string to map[string]string for JSON.
func stringMap(m map[common.RequestState]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// JobDefinitionConfig
// ──────────────────────────────────────────────────────────────────────────────

// Validate checks required fields on the job definition config.
func (c *JobDefinitionConfig) Validate() error {
	c.Errors = make(map[string]string)
	var err error
	if c.Id != "" && c.JobDefinitionId == "" {
		err = errors.New("job-definition-id is not specified")
		c.Errors["JobDefinitionId"] = err.Error()
	}
	if c.Name == "" {
		err = errors.New("name is not specified")
		c.Errors["Name"] = err.Error()
	}
	if c.Kind == "" {
		err = errors.New("type is not specified")
		c.Errors["Kind"] = err.Error()
	}
	if c.Value == "" {
		err = errors.New("value is not specified")
		c.Errors["Value"] = err.Error()
	}
	if len(c.Value) > maxConfigValueLength {
		err = errors.New("value is too big")
		c.Errors["Value"] = err.Error()
	}
	return err
}

// ValidateBeforeSave validates and encrypts config before persistence.
func (c *JobDefinitionConfig) ValidateBeforeSave(key []byte) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Encrypt(key)
}

// GetVariableValue returns the config value as a VariableValue.
func (c *JobDefinitionConfig) GetVariableValue() (common.VariableValue, error) {
	return common.NewVariableValue(c.Value, c.Secret), nil
}

// Encrypt encrypts the config value using the provided key.
func (c *JobDefinitionConfig) Encrypt(key []byte) error {
	nv := common.NameTypeValue{Name: c.Name, Kind: c.Kind, Value: c.Value, Secret: c.Secret}
	if err := nv.Encrypt(key); err != nil {
		return err
	}
	c.Value = nv.Value
	return nil
}

// Decrypt decrypts the config value using the provided key.
func (c *JobDefinitionConfig) Decrypt(key []byte) error {
	nv := common.NameTypeValue{Name: c.Name, Kind: c.Kind, Value: c.Value, Secret: c.Secret}
	if err := nv.Decrypt(key); err != nil {
		return err
	}
	c.Value = nv.Value
	return nil
}

// Summary returns a short human-readable description of the config.
func (c *JobDefinitionConfig) Summary() string {
	return fmt.Sprintf("%s=%s", c.Name, c.Value)
}

// ──────────────────────────────────────────────────────────────────────────────
// JobDefinitionVariable
// ──────────────────────────────────────────────────────────────────────────────

// GetVariableValue returns the variable as a VariableValue.
func (v *JobDefinitionVariable) GetVariableValue() (common.VariableValue, error) {
	nv := common.NameTypeValue{Name: v.Name, Kind: v.Kind, Value: v.Value, Secret: v.Secret}
	return nv.GetVariableValue()
}

// GetParsedValue parses the variable value into its native Go type.
func (v *JobDefinitionVariable) GetParsedValue() (interface{}, error) {
	nv := common.NameTypeValue{Name: v.Name, Kind: v.Kind, Value: v.Value, Secret: v.Secret}
	return nv.GetParsedValue()
}

// ──────────────────────────────────────────────────────────────────────────────
// TaskDefinition
// ──────────────────────────────────────────────────────────────────────────────

// HasNext returns true if the task has any exit code routing.
func (td *TaskDefinition) HasNext() bool {
	return td.OnExitCodeJson != "" || td.OnCompleted != "" || td.OnFailed != ""
}

// ShortTaskType returns an abbreviated task type (alphanumeric, max 8 chars).
func (td *TaskDefinition) ShortTaskType() string {
	reg := regexp.MustCompile("[^a-zA-Z]+")
	if len(td.TaskType) <= 8 {
		return reg.ReplaceAllString(td.TaskType, "")
	}
	return reg.ReplaceAllString(td.TaskType[0:8], "")
}

// ScriptString returns a comma-joined script for display.
func (td *TaskDefinition) ScriptString() string {
	return strings.Join(td.Script, ",")
}

// VariablesString returns a compact text representation of variables.
func (td *TaskDefinition) VariablesString() string {
	var b strings.Builder
	for _, c := range td.Variables {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	return b.String()
}

// Summary returns a short human-readable description of the task.
func (td *TaskDefinition) Summary() string {
	return fmt.Sprintf("TaskType=%s Method=%s Script=%v OnCompleted=%s OnFailed=%s",
		td.TaskType, td.Method, td.ScriptString(), td.OnCompleted, td.OnFailed)
}

// GetDelayBetweenRetries returns the retry delay with jitter.
func (td *TaskDefinition) GetDelayBetweenRetries() time.Duration {
	if td.DelayBetweenRetriesNs <= 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(2)); err == nil {
			return time.Second * time.Duration(n.Int64()+1)
		}
		return time.Second * 2
	}
	return time.Duration(td.DelayBetweenRetriesNs)
}

// IsExcept evaluates the Except field for "true".
func (td *TaskDefinition) IsExcept() bool {
	return strings.Contains(td.Except, "true")
}

// SetAlwaysRun marks the task as always-run, returning self.
func (td *TaskDefinition) SetAlwaysRun() *TaskDefinition {
	td.AlwaysRun = true
	return td
}

// GetVariable returns a variable by name from the lookup cache.
func (td *TaskDefinition) GetVariable(name string) *TaskDefinitionVariable {
	lk := tdVars(td.Id)
	obj := lk.GetObject(name)
	if obj == nil {
		return nil
	}
	return obj.(*TaskDefinitionVariable)
}

// FilteredVariables returns non-reserved variables.
func (td *TaskDefinition) FilteredVariables() []*TaskDefinitionVariable {
	out := make([]*TaskDefinitionVariable, 0)
	for _, c := range td.Variables {
		if !isReservedConfigProperty(c.Name) {
			out = append(out, c)
		}
	}
	return out
}

// MaskTaskVariables returns non-secret variables.
func (td *TaskDefinition) MaskTaskVariables() []*TaskDefinitionVariable {
	out := make([]*TaskDefinitionVariable, 0)
	for _, v := range td.Variables {
		if !v.Secret {
			out = append(out, v)
		}
	}
	return out
}

// GetNameValueVariables returns all variables as a map of VariableValue.
func (td *TaskDefinition) GetNameValueVariables() map[string]common.VariableValue {
	res := make(map[string]common.VariableValue)
	for _, v := range td.Variables {
		if vv, err := v.GetVariableValue(); err == nil {
			res[v.Name] = vv
		}
	}
	return res
}

// AddExitCode registers an exit-code routing entry.
func (td *TaskDefinition) AddExitCode(status string, targetTask string) *TaskDefinition {
	onExit := td.loadOnExitCodeMap()
	onExit[common.NewRequestState(status)] = targetTask
	td.saveOnExitCodeMap(onExit)
	return td
}

// LoadOnExitCode deserializes OnExitCodeSerialized into a map and returns it.
func (td *TaskDefinition) LoadOnExitCode() (map[common.RequestState]string, error) {
	return td.loadOnExitCodeMap(), nil
}

// SaveOnExitCode serializes the on-exit-code map to OnExitCodeSerialized.
func (td *TaskDefinition) SaveOnExitCode() (string, error) {
	onExit := td.loadOnExitCodeMap()
	if len(onExit) > 0 {
		b, err := json.Marshal(stringMap(onExit))
		if err != nil {
			return "", err
		}
		td.OnExitCodeSerialized = string(b)
	} else {
		td.OnExitCodeSerialized = ""
	}
	return td.OnExitCodeSerialized, nil
}

// OverrideStatusAndErrorCode maps an exit code to a RequestState and error code.
func (td *TaskDefinition) OverrideStatusAndErrorCode(exitCode string) (status common.RequestState, errorCode string) {
	onExit := td.loadOnExitCodeMap()
	if len(onExit) == 0 {
		return
	}
	target := onExit[common.NewRequestState(exitCode)]
	targetState := common.NewRequestState(target)
	switch targetState {
	case common.FATAL:
		return common.FAILED, common.ErrorFatal
	case common.FAILED:
		return common.FAILED, ""
	case common.COMPLETED:
		return common.COMPLETED, ""
	case common.EXECUTING:
		return common.EXECUTING, ""
	case common.RESTART_JOB:
		return common.FAILED, common.ErrorRestartJob
	case common.PAUSE_JOB:
		return common.PAUSED, common.ErrorPauseJob
	case common.WAIT_FOR_APPROVAL:
		return common.MANUAL_APPROVAL_REQUIRED, common.ErrorManualApprovalRequired
	case common.RESTART_TASK:
		return common.FAILED, common.ErrorRestartTask
	default:
		if strings.HasPrefix(target, "ERR_") {
			return common.FAILED, target
		}
	}
	return
}

// AddVariable adds or updates a task variable, handling reserved keys.
func (td *TaskDefinition) AddVariable(name string, value interface{}) (*TaskDefinitionVariable, error) {
	variable, err := newTaskDefinitionVariable(name, value)
	if err != nil {
		return nil, err
	}
	lk := tdVars(td.Id)

	// Handle reserved keys by updating transient fields
	switch name {
	case keyHeaders:
		td.Headers = value.(map[string]string)
	case keyBeforeScript:
		td.BeforeScript = value.([]string)
	case keyAfterScript:
		td.AfterScript = value.([]string)
	case keyScript:
		td.Script = value.([]string)
	case keyTags:
		td.Tags = value.([]string)
	case keyExcept:
		td.Except = fmt.Sprintf("%s", value)
	case keyJobVersion:
		tdTransient(td.Id).JobVersion = fmt.Sprintf("%s", value)
	case keyDeps:
		tdTransient(td.Id).Dependencies = value.([]string)
	case keyArtifacts:
		switch v := value.(type) {
		case []string:
			tdTransient(td.Id).ArtifactIDs = v
		default:
			tdTransient(td.Id).ArtifactIDs = strings.Split(fmt.Sprintf("%v", value), ",")
		}
	}

	variable.TaskDefinitionId = td.Id
	if lk.GetObject(name) == nil {
		td.Variables = append(td.Variables, variable)
	} else {
		for _, next := range td.Variables {
			if next.Name == name {
				next.Value = variable.Value
			}
		}
	}
	lk.SetObject(name, variable)
	return variable, nil
}

// AfterLoad initializes transient fields after loading from DB.
func (td *TaskDefinition) AfterLoad() error {
	lk := tdVars(td.Id)
	nameValueVars := make(map[string]interface{})

	for _, c := range td.Variables {
		lk.SetObject(c.Name, c)
		nv := common.NameTypeValue{Name: c.Name, Kind: c.Kind, Value: c.Value, Secret: c.Secret}
		v, err := nv.GetParsedValue()
		if err != nil {
			return err
		}

		switch c.Name {
		case keyHeaders:
			td.Headers = make(map[string]string)
			if err = json.Unmarshal([]byte(c.Value), &td.Headers); err != nil {
				return err
			}
		case keyBeforeScript:
			td.BeforeScript = make([]string, 0)
			if err = json.Unmarshal([]byte(c.Value), &td.BeforeScript); err != nil {
				return err
			}
		case keyAfterScript:
			td.AfterScript = make([]string, 0)
			if err = json.Unmarshal([]byte(c.Value), &td.AfterScript); err != nil {
				return err
			}
		case keyScript:
			td.Script = make([]string, 0)
			if err = json.Unmarshal([]byte(c.Value), &td.Script); err != nil {
				return err
			}
		case keyTags:
			if err = json.Unmarshal([]byte(c.Value), &td.Tags); err != nil {
				return err
			}
		case keyExcept:
			if err = json.Unmarshal([]byte(c.Value), &td.Except); err != nil {
				return err
			}
		case keyJobVersion:
			var jv string
			if err = json.Unmarshal([]byte(c.Value), &jv); err != nil {
				return err
			}
			tdTransient(td.Id).JobVersion = jv
		case keyDeps:
			var deps []string
			if err = json.Unmarshal([]byte(c.Value), &deps); err != nil {
				return err
			}
			tdTransient(td.Id).Dependencies = deps
		case keyArtifacts:
			var arts []string
			if err = json.Unmarshal([]byte(c.Value), &arts); err != nil {
				return err
			}
			tdTransient(td.Id).ArtifactIDs = arts
		default:
			nameValueVars[c.Name] = v
		}
	}

	if td.NameValueVariablesJson == "" && len(nameValueVars) > 0 {
		if b, err := json.Marshal(nameValueVars); err == nil {
			td.NameValueVariablesJson = string(b)
		}
	}

	// deserialize on_exit_code
	if td.OnExitCodeSerialized != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(td.OnExitCodeSerialized), &raw); err != nil {
			return err
		}
		result := make(map[common.RequestState]string, len(raw))
		for k, v := range raw {
			result[common.NewRequestState(k)] = v
		}
		if b, err := json.Marshal(result); err == nil {
			td.OnExitCodeJson = string(b)
		}
	}

	if td.OnCompleted != "" {
		onExit := td.loadOnExitCodeMap()
		onExit[common.COMPLETED] = td.OnCompleted
		td.saveOnExitCodeMap(onExit)
	}
	if td.OnFailed != "" {
		onExit := td.loadOnExitCodeMap()
		onExit[common.FAILED] = td.OnFailed
		td.saveOnExitCodeMap(onExit)
	}

	// no Url field on TaskDefinition in proto; URL handling happens at executor level
	return nil
}

// Validate validates required fields on the task definition.
func (td *TaskDefinition) Validate() error {
	if td.TaskType == "" {
		return errors.New("taskType is not specified")
	}
	if td.Method == "" {
		td.Method = "KUBERNETES"
	}
	if len(td.TaskType) > 100 {
		return errors.New("taskType is too big")
	}
	if len(td.Description) > 500 {
		return errors.New("description is too big")
	}
	if len(td.HostNetwork) > 100 {
		return errors.New("host network is too big")
	}
	for i := range td.Tags {
		td.Tags[i] = strings.ToLower(td.Tags[i])
	}
	return nil
}

// ValidateBeforeSave validates and serializes the task definition before persistence.
func (td *TaskDefinition) ValidateBeforeSave() error {
	if err := td.Validate(); err != nil {
		return err
	}
	if len(td.Headers) > 0 {
		_, _ = td.AddVariable(keyHeaders, td.Headers)
	}
	if len(td.BeforeScript) > 0 {
		if _, err := td.AddVariable(keyBeforeScript, td.BeforeScript); err != nil {
			return err
		}
	}
	if len(td.AfterScript) > 0 {
		if _, err := td.AddVariable(keyAfterScript, td.AfterScript); err != nil {
			return err
		}
	}
	if len(td.Script) > 0 {
		if _, err := td.AddVariable(keyScript, td.Script); err != nil {
			return err
		}
	}
	if len(td.Tags) > 0 {
		if _, err := td.AddVariable(keyTags, td.Tags); err != nil {
			return err
		}
	}
	if td.Except != "" {
		if _, err := td.AddVariable(keyExcept, td.Except); err != nil {
			return err
		}
	}
	tr := tdTransient(td.Id)
	if tr.JobVersion != "" {
		if _, err := td.AddVariable(keyJobVersion, tr.JobVersion); err != nil {
			return err
		}
	}
	if len(tr.Dependencies) > 0 {
		if _, err := td.AddVariable(keyDeps, tr.Dependencies); err != nil {
			return err
		}
	}
	if len(tr.ArtifactIDs) > 0 {
		if _, err := td.AddVariable(keyArtifacts, tr.ArtifactIDs); err != nil {
			return err
		}
	}
	// serialize name-value variables
	if td.NameValueVariablesJson != "" {
		var nameValueVars map[string]interface{}
		if err := json.Unmarshal([]byte(td.NameValueVariablesJson), &nameValueVars); err == nil {
			for n, v := range nameValueVars {
				if _, err := td.AddVariable(n, v); err != nil {
					return err
				}
			}
		}
	}
	_, err := td.SaveOnExitCode()
	return err
}

// Equals compares another task definition for logical equality.
func (td *TaskDefinition) Equals(other *TaskDefinition) error {
	if other == nil {
		return errors.New("found nil other task")
	}
	if err := td.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}
	if td.TaskType != other.TaskType {
		return fmt.Errorf("expected taskType %v but was %v", td.TaskType, other.TaskType)
	}
	if len(td.Variables) != len(other.Variables) {
		return fmt.Errorf("expected %d task variables but was %d\nmine: %v\ntheirs: %v",
			len(td.Variables), len(other.Variables), td.VariablesString(), other.VariablesString())
	}
	otherLk := tdVars(other.Id)
	for _, c := range other.Variables {
		local := otherLk.GetObject(c.Name)
		if local == nil {
			local = tdVars(td.Id).GetObject(c.Name)
		}
		if local == nil {
			return fmt.Errorf("failed to find task variable for %s as %s", c.Name, c.Value)
		}
		if local.(*TaskDefinitionVariable).Value != c.Value {
			return fmt.Errorf("expected task variable %s as %s but was %s", c.Name, local.(*TaskDefinitionVariable).Value, c.Value)
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// loadOnExitCodeMap / saveOnExitCodeMap (internal)
// ──────────────────────────────────────────────────────────────────────────────

func (td *TaskDefinition) loadOnExitCodeMap() map[common.RequestState]string {
	onExit := make(map[common.RequestState]string)
	if td.OnExitCodeJson != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(td.OnExitCodeJson), &raw); err == nil {
			for k, v := range raw {
				onExit[common.NewRequestState(k)] = v
			}
		}
	} else if td.OnExitCodeSerialized != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(td.OnExitCodeSerialized), &raw); err == nil {
			for k, v := range raw {
				onExit[common.NewRequestState(k)] = v
			}
		}
	}
	if td.OnCompleted != "" {
		onExit[common.COMPLETED] = td.OnCompleted
	}
	if td.OnFailed != "" {
		onExit[common.FAILED] = td.OnFailed
	}
	return onExit
}

func (td *TaskDefinition) saveOnExitCodeMap(onExit map[common.RequestState]string) {
	if len(onExit) == 0 {
		td.OnExitCodeJson = ""
		return
	}
	if b, err := json.Marshal(stringMap(onExit)); err == nil {
		td.OnExitCodeJson = string(b)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// TaskDefinitionVariable
// ──────────────────────────────────────────────────────────────────────────────

// GetVariableValue returns the variable as a VariableValue.
func (v *TaskDefinitionVariable) GetVariableValue() (common.VariableValue, error) {
	nv := common.NameTypeValue{Name: v.Name, Kind: v.Kind, Value: v.Value, Secret: v.Secret}
	return nv.GetVariableValue()
}

// ──────────────────────────────────────────────────────────────────────────────
// Factory helpers (package-private)
// ──────────────────────────────────────────────────────────────────────────────

func newJobDefinitionConfig(name string, value interface{}, secret bool) (*JobDefinitionConfig, error) {
	nv, err := common.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionConfig{
		Id:        ulid(),
		Name:      nv.Name,
		Kind:      nv.Kind,
		Value:     nv.Value,
		Secret:    nv.Secret,
		CreatedAt: nowTimestamp(),
		UpdatedAt: nowTimestamp(),
	}, nil
}

func newJobDefinitionVariable(name string, value interface{}) (*JobDefinitionVariable, error) {
	nv, err := common.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionVariable{
		Id:        ulid(),
		Name:      nv.Name,
		Kind:      nv.Kind,
		Value:     nv.Value,
		Secret:    nv.Secret,
		CreatedAt: nowTimestamp(),
		UpdatedAt: nowTimestamp(),
	}, nil
}

func newTaskDefinitionVariable(name string, value interface{}) (*TaskDefinitionVariable, error) {
	nv, err := common.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &TaskDefinitionVariable{
		Id:        ulid(),
		Name:      nv.Name,
		Kind:      nv.Kind,
		Value:     nv.Value,
		Secret:    nv.Secret,
		CreatedAt: nowTimestamp(),
		UpdatedAt: nowTimestamp(),
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Reserved config property names
// ──────────────────────────────────────────────────────────────────────────────

func isReservedConfigProperty(name string) bool {
	switch name {
	case keyHeaders, keyAfterScript, keyBeforeScript, keyScript,
		keyResources, keyRequiredParams, keyTags, keyExcept,
		keyJobVersion, keyDeps, keyArtifacts:
		return true
	}
	return false
}
