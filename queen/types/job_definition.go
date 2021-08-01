package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v3"

	"github.com/gorhill/cronexpr"
	common "plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/queen/utils"
)

const maxConfigValueLength = 1000
const jobVariables = "job_variables:"
const maxTasksPerJob = 100
const keyRequiredParams = "required_params"

// JobTypeCronTrigger abstracts job-type and cron trigger
type JobTypeCronTrigger struct {
	// UserID defines user who updated the job
	UserID string
	// OrganizationID defines org who submitted the job
	OrganizationID string
	// JobType defines type of job
	JobType string
	// CronTrigger can be used to run the job periodically
	CronTrigger string
	// User key
	UserKey string
}

// NewJobTypeCronTrigger constructor
func NewJobTypeCronTrigger(job *JobDefinition) JobTypeCronTrigger {
	return JobTypeCronTrigger{
		UserID:         job.UserID,
		OrganizationID: job.OrganizationID,
		JobType:        job.JobType,
		CronTrigger:    job.CronTrigger,
		UserKey:        job.GetUserJobTypeKey(),
	}
}

func (jtc JobTypeCronTrigger) String() string {
	return jtc.UserID + jtc.OrganizationID + jtc.JobType
}

// JobDefinition defines a DAG (directed acyclic graph) of tasks, which are executed by the ant followers.
// The workflow of job uses task exit codes to define next task to execute. The task definition
// represents definition of a job and instance of the job is created using JobExecution when a new job request is
// submitted.
type JobDefinition struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobType defines a unique type of job
	JobType string `yaml:"job_type" json:"job_type"`
	// Version defines internal version of the job-definition, which is updated when a job is updated. The database
	// stores each version as a separate row but only latest version is used for new jobs.
	Version int32 `yaml:"-" json:"-"`
	// SemVersion - semantic version is used for external version, which can be used for public plugins.
	SemVersion string `yaml:"sem_version" json:"sem_version"`
	// URL defines url for job
	URL string `json:"url"`
	// UserID defines user who updated the job
	UserID string `json:"user_id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// Description of job
	Description string `yaml:"description,omitempty" json:"description"`
	// Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering
	Platform string `yaml:"platform,omitempty" json:"platform"`
	// CronTrigger can be used to run the job periodically
	CronTrigger string `yaml:"cron_trigger,omitempty" json:"cron_trigger"`
	// Timeout defines max time a job should take, otherwise the job is aborted
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	// Retry defines max number of tries a job can be retried where it re-runs failed job
	Retry int `yaml:"retry,omitempty" json:"retry"`
	// HardResetAfterRetries defines retry config when job is rerun and as opposed to re-running only failed tasks, all tasks are executed.
	HardResetAfterRetries int `yaml:"hard_reset_after_retries,omitempty" json:"hard_reset_after_retries"`
	// DelayBetweenRetries defines time between retry of job
	DelayBetweenRetries time.Duration `yaml:"delay_between_retries,omitempty" json:"delay_between_retries"`
	// MaxConcurrency defines max number of jobs that can be run concurrently
	MaxConcurrency int `yaml:"max_concurrency,omitempty" json:"max_concurrency"`
	// Paused is used to stop further processing of job and it can be used during maintenance, upgrade or debugging.
	Paused bool `yaml:"-" json:"paused"`
	// PublicPlugin means job is public plugin
	PublicPlugin bool `yaml:"public_plugin,omitempty" json:"public_plugin"`
	// RequiredParams from job request (and plugin)
	RequiredParams []string `yaml:"required_params,omitempty" json:"required_params" gorm:"-"`
	// UsesTemplate means the task is optional and can fail without failing entire job
	UsesTemplate bool `yaml:"-" json:"-"`
	// Tags are used to use specific followers that support the tags defined by ants.
	// Tags is aggregation of task tags
	Tags string `yaml:"tags,omitempty" json:"tags"`
	// Methods is aggregation of task methods
	Methods string `yaml:"methods,omitempty" json:"methods"`
	// RawYaml stores raw YAML of job definition
	RawYaml string `yaml:"-" json:"-"`
	// Tasks defines one to many relationships between job and tasks, where tasks defines a
	// directed acyclic graph of tasks that are executed for the job.
	Tasks []*TaskDefinition `yaml:"tasks" json:"tasks" gorm:"ForeignKey:JobDefinitionID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// Configs defines config properties of job that are used as parameters for the job template or task request when executing on a remote
	// ant follower. Both config and variables provide similar capabilities but config can be updated for all job versions and can store
	// sensitive data.
	Configs []*JobDefinitionConfig `yaml:"-" json:"-" gorm:"ForeignKey:JobDefinitionID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// Variables defines properties of job that are used as parameters for the job template or task request when executing on a remote
	// ant follower. Both config and variables provide similar capabilities but variables are part of the job yaml definition.
	Variables []*JobDefinitionVariable `yaml:"-" json:"-" gorm:"ForeignKey:JobDefinitionID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
	// Active is used to soft delete job definition
	Active bool `yaml:"-" json:"-"`
	// Following are transient properties -- these are populated when AfterLoad or Validate is called
	CanEdit            bool          `yaml:"-" json:"-" gorm:"-"`
	NameValueVariables interface{}   `yaml:"job_variables,omitempty" json:"job_variables" gorm:"-"`
	Resources          BasicResource `yaml:"resources,omitempty" json:"resources" gorm:"-"`
	filter             string
	lookupTasks        *cutils.SafeMap
	lock               sync.RWMutex
}

// NewJobDefinition creates new instance of job-definition
func NewJobDefinition(jobType string) *JobDefinition {
	return &JobDefinition{
		JobType:            jobType,
		MaxConcurrency:     3,
		Configs:            make([]*JobDefinitionConfig, 0),
		Variables:          make([]*JobDefinitionVariable, 0),
		Tasks:              make([]*TaskDefinition, 0),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		lookupTasks:        cutils.NewSafeMap(),
		NameValueVariables: make(map[string]interface{}),
		RawYaml:            "",
	}
}

// NewJobDefinitionFromYaml creates new instance of job-definition
func NewJobDefinitionFromYaml(b []byte) (job *JobDefinition, err error) {
	var jobTypeRegex *regexp.Regexp
	if jobTypeRegex, err = regexp.Compile(`job_type:\s+(.*)\s+`); err != nil {
		return nil, err
	}
	yamlSource := strings.TrimSpace(string(b))
	names := jobTypeRegex.FindStringSubmatch(yamlSource)
	if len(names) <= 1 {
		return nil, fmt.Errorf("failed to find job-type in %v", names)
	}
	if strings.Contains(yamlSource, "{{") && strings.Contains(yamlSource, "}}") {
		partialYaml, err := removeTemplateVariables(yamlSource)
		if err != nil {
			return nil, err
		}
		job = NewJobDefinition("")
		err = yaml.Unmarshal([]byte(partialYaml), &job)
	} else {
		job = NewJobDefinition("")
		err = yaml.Unmarshal(b, &job)
	}
	for i, task := range job.Tasks {
		task.TaskOrder = i
	}
	if err != nil {
		return nil, err
	}
	job.RawYaml = yamlSource
	if err = job.Validate(); err != nil {
		return nil, err
	}
	return job, nil
}

// TableName overrides default table name
func (*JobDefinition) TableName() string {
	return "formicary_job_definitions"
}

// GetDelayBetweenRetries delay between retries
func (jd *JobDefinition) GetDelayBetweenRetries() time.Duration {
	if jd.DelayBetweenRetries <= 0 {
		jd.DelayBetweenRetries = time.Second * time.Duration(rand.Intn(10)+5)
	}
	return jd.DelayBetweenRetries
}

// GetNextTask next task to run
func (jd *JobDefinition) GetNextTask(
	task *TaskDefinition,
	taskStatus common.RequestState,
	exitCode string) (*TaskDefinition, error) {
	if task.OnExitCode == nil || len(task.OnExitCode) == 0 {
		return nil, nil
	}
	nextTaskName := task.OnExitCode[common.NewRequestState(exitCode)]

	nextTask := jd.GetTask(nextTaskName)
	if nextTask != nil {
		return nextTask, nil
	}
	nextTask = jd.GetTask(task.OnExitCode[common.NewRequestState(string(taskStatus))])
	if nextTask != nil {
		return nextTask, nil
	}
	if task.AllowFailure {
		return jd.GetTask(task.OnExitCode[common.COMPLETED]), nil
	}
	return nil, nil
}

// Filter returns filter tag
func (jd *JobDefinition) Filter() string {
	if !jd.UsesTemplate || jd.filter != "" {
		return jd.filter
	}
	// parse job
	jd.filter = utils.ParseYamlTag(jd.RawYaml, "filter:")
	if jd.filter == "" {
		jd.filter = "none"
	}
	return jd.filter
}

// Filtered checks filter condition
func (jd *JobDefinition) Filtered(data map[string]interface{}) bool {
	if jd.Filter() == "" {
		return false
	}
	resData, err := utils.ParseTemplate(jd.Filter(), data)
	if err != nil {
		return false
	}
	return strings.Contains(resData, "true")
}

// GetDynamicTask next task to run from YAML config
func (jd *JobDefinition) GetDynamicTask(
	taskType string,
	data map[string]interface{}) (task *TaskDefinition, opts *common.ExecutorOptions, err error) {
	if data == nil {
		data = make(map[string]interface{})
	}
	task = jd.GetTask(taskType)
	if task == nil {
		return nil, nil, fmt.Errorf("failed to find task %s", taskType)
	}
	if task.Method == "" {
		task.Method = common.Kubernetes
	}
	for _, v := range task.Variables {
		if parsed, err := v.GetParsedValue(); err == nil {
			data[v.Name] = parsed
		} else {
			return nil, nil, fmt.Errorf("failed to parse value for %v due to %v", v, err)
		}
	}

	// parse job
	serData := utils.ParseYamlTag(jd.RawYaml, fmt.Sprintf("task_type: %s", taskType))
	if serData == "" {
		return nil, nil, fmt.Errorf("failed to find %s from Yaml definition", taskType)
	}
	if jd.UsesTemplate {
		serData, err = utils.ParseTemplate(serData, data)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "JobDefinition",
				"JobType":   jd.JobType,
				"Version":   jd.SemVersion,
				"TaskType":  taskType,
				"Data":      data,
				"Raw":       utils.ParseYamlTag(jd.RawYaml, fmt.Sprintf("task_type: %s", taskType)),
				"Error":     err,
			}).Error("failed to parse yaml task")
			return nil, nil, fmt.Errorf("failed to parse task yaml for %s due to %s",
				taskType, err.Error())
		}
		err = yaml.Unmarshal([]byte(serData), task)
	}

	task = NewTaskDefinition("", "")
	err = yaml.Unmarshal([]byte(serData), task)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobDefinition",
			"JobType":   jd.JobType,
			"Version":   jd.SemVersion,
			"TaskType":  taskType,
			"Data":      data,
			"Raw":       utils.ParseYamlTag(jd.RawYaml, fmt.Sprintf("task_type: %s", taskType)),
			"Error":     err,
		}).Error("failed to parse yaml task")
		return nil, nil, fmt.Errorf("failed to parse %s due to %v", taskType, err)
	}
	_ = task.addVariablesFromNameValueVariables()

	// after-load to add on-exit and other properties
	if err = task.AfterLoad(); err != nil {
		return nil, nil, err
	}

	if err = task.Validate(); err != nil {
		return nil, nil, err
	}

	index := strings.Index(serData, "task_type")
	if index >= 0 {
		serData = strings.TrimSpace(serData[index:])
	}

	opts = common.NewExecutorOptions("", "")
	err = yaml.Unmarshal([]byte(serData), opts)
	if err != nil {
		return nil, nil, err
	}
	opts.Method = task.Method
	if err = opts.Validate(); err != nil {
		return nil, nil, err
	}
	return task, opts, nil
}

// GetDynamicConfig from yaml
func (jd *JobDefinition) GetDynamicConfig(
	data interface{}) (out map[string]interface{}, err error) {
	if !jd.UsesTemplate {
		jd.lock.RLock()
		defer jd.lock.RUnlock()
		return utils.ParseNameValueConfigs(jd.NameValueVariables)
	}
	serVariables := utils.ParseYamlTag(jd.RawYaml, jobVariables)
	if serVariables == "" {
		return nil, fmt.Errorf("failed to job variables")
	}
	serVariablesAfterTemplate, err := utils.ParseTemplate(serVariables, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Yaml for job variables due to %s", err.Error())
	}
	err = yaml.Unmarshal([]byte(serVariablesAfterTemplate), &out)
	if err != nil {
		return nil, err
	}
	for _, c := range jd.Configs {
		if v, err := c.GetParsedValue(); err == nil {
			out[c.Name] = v
		}
	}
	return
}

// String provides short summary of job
func (jd *JobDefinition) String() string {
	return fmt.Sprintf("JobType=%s Variables=%s",
		jd.JobType, jd.VariablesString())
}

// JobTypeAndVersion with version
func (jd *JobDefinition) JobTypeAndVersion() string {
	if jd.SemVersion == "" {
		return jd.JobType
	}
	return jd.JobType + ":" + jd.SemVersion
}

// Yaml config
func (jd *JobDefinition) Yaml() string {
	if jd.RawYaml != "" {
		return jd.RawYaml
	}
	b, _ := yaml.Marshal(jd)
	return string(b)
}

// UpdateRawYaml updates raw yaml
func (jd *JobDefinition) UpdateRawYaml() {
	b, _ := yaml.Marshal(jd)
	jd.RawYaml = string(b)
	jd.filter = ""
}

// TaskNames returns task names
func (jd *JobDefinition) TaskNames() string {
	var b strings.Builder
	for _, t := range jd.Tasks {
		b.WriteString(t.TaskType)
		b.WriteString(" ")
	}
	return b.String()
}

// VariablesString - text view of variables
func (jd *JobDefinition) VariablesString() string {
	var b strings.Builder
	sort.Slice(jd.Variables, func(i, j int) bool { return jd.Variables[i].Name < jd.Variables[j].Name })
	for _, c := range jd.Variables {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	return b.String()
}

// ConfigsString - text view of config
func (jd *JobDefinition) ConfigsString() string {
	var b strings.Builder
	for _, c := range jd.Configs {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	return b.String()
}

// AddTasks adds tasks
func (jd *JobDefinition) AddTasks(tasks ...*TaskDefinition) *JobDefinition {
	for _, t := range tasks {
		jd.AddTask(t)
	}
	return jd
}

// AddTask adds task
func (jd *JobDefinition) AddTask(task *TaskDefinition) *TaskDefinition {
	old := jd.lookupTasks.GetObject(task.TaskType)
	if old == nil {
		jd.Tasks = append(jd.Tasks, task)
		task.TaskOrder = jd.lookupTasks.Len()
	} else {
		task.TaskOrder = old.(*TaskDefinition).TaskOrder
	}
	jd.lookupTasks.SetObject(task.TaskType, task)
	return task
}

// GetTask finds task
func (jd *JobDefinition) GetTask(taskType string) *TaskDefinition {
	old := jd.lookupTasks.GetObject(taskType)
	if old == nil {
		return nil
	}
	return old.(*TaskDefinition)
}

// DeleteFilteredCronJobs using job variable
func (jd *JobDefinition) DeleteFilteredCronJobs() bool {
	if jd.CronTrigger == "" {
		return false
	}
	return jd.GetVariable("DeleteFilteredCronJobs") == true
}

// GetVariable gets job variable
func (jd *JobDefinition) GetVariable(name string) interface{} {
	jd.lock.RLock()
	defer jd.lock.RUnlock()
	return jd.NameValueVariables.(map[string]interface{})[name]
}

// AddVariable adds job variable
func (jd *JobDefinition) AddVariable(
	name string,
	value interface{}) (*JobDefinitionVariable, error) {
	variable, err := NewJobDefinitionVariable(name, value)
	if err != nil {
		return nil, err
	}
	if name == keyResources {
		jd.Resources = value.(BasicResource)
	} else if name == keyRequiredParams {
		jd.RequiredParams = value.([]string)
	} else {
		jd.lock.Lock()
		defer jd.lock.Unlock()
		nameValueVariables := jd.NameValueVariables.(map[string]interface{})
		nameValueVariables[name] = value
	}

	variable.JobDefinitionID = jd.ID
	found := false
	for _, next := range jd.Variables {
		if next.Name == name {
			next.Value = variable.Value
			next.Type = variable.Type
			found = true
		}
	}

	if !found {
		jd.Variables = append(jd.Variables, variable)
	}
	return variable, nil
}

// RemoveVariable adds job variable
func (jd *JobDefinition) RemoveVariable(name string) bool {
	if name == keyResources {
		jd.Resources = BasicResource{}
	} else if name == keyRequiredParams {
		jd.RequiredParams = []string{}
	} else {
		jd.lock.Lock()
		defer jd.lock.Unlock()
		nameValueVariables := jd.NameValueVariables.(map[string]interface{})
		delete(nameValueVariables, name)
	}

	for i, next := range jd.Variables {
		if next.Name == name {
			jd.Variables = append(jd.Variables[:i], jd.Variables[i+1:]...)
			return true
		}
	}
	return false
}

// AddConfig adds config
func (jd *JobDefinition) AddConfig(
	name string,
	value interface{},
	secret bool) (*JobDefinitionConfig, error) {
	config, err := NewJobDefinitionConfig(name, value, secret)
	if err != nil {
		return nil, err
	}

	matched := false
	for _, next := range jd.Configs {
		if next.Name == name {
			next.Value = config.Value
			next.Type = config.Type
			next.Secret = config.Secret
			config = next
			matched = true
			break
		}
	}
	if !matched {
		config.JobDefinitionID = jd.ID
		jd.Configs = append(jd.Configs, config)
	}
	return config, nil
}

// RemoveConfig adds job config
func (jd *JobDefinition) RemoveConfig(name string) bool {
	for i, next := range jd.Configs {
		if next.Name == name {
			jd.Configs = append(jd.Configs[:i], jd.Configs[i+1:]...)
			return true
		}
	}

	return false
}

// GetConfig gets config
func (jd *JobDefinition) GetConfig(name string) *JobDefinitionConfig {
	for _, next := range jd.Configs {
		if next.Name == name {
			return next
		}
	}
	return nil
}

// GetConfigByID gets config
func (jd *JobDefinition) GetConfigByID(configID string) *JobDefinitionConfig {
	for _, next := range jd.Configs {
		if next.ID == configID {
			return next
		}
	}
	return nil
}

// GetOrganizationID returns org
func (jd *JobDefinition) GetOrganizationID() string {
	return jd.OrganizationID
}

// GetUserID returns user-id
func (jd *JobDefinition) GetUserID() string {
	return jd.UserID
}

// GetJobType defines the type of job
func (jd *JobDefinition) GetJobType() string {
	return jd.JobType
}

// GetJobVersion defines the version of job
func (jd *JobDefinition) GetJobVersion() string {
	return jd.SemVersion
}

// GetUserJobTypeKey defines key
func (jd *JobDefinition) GetUserJobTypeKey() string {
	return getUserJobTypeKey(jd.OrganizationID, jd.UserID, jd.JobType, jd.SemVersion)
}

// Equals compares other job-definition for equality
func (jd *JobDefinition) Equals(other *JobDefinition) error {
	if other == nil {
		return fmt.Errorf("other job is  nil")
	}
	if err := jd.Validate(); err != nil { // ValidateBeforeSave
		return err
	}
	if err := other.Validate(); err != nil { // ValidateBeforeSave
		return err
	}

	if jd.JobType != other.JobType {
		return fmt.Errorf("expected jobType %v but was %v", jd.JobType, other.JobType)
	}
	if len(jd.Variables) != len(other.Variables) {
		return fmt.Errorf("expected number of job variable %v but was %v\nvariable: %v\ntheirs: %v",
			len(jd.Variables), len(other.Variables), jd.VariablesString(), other.VariablesString())
	}
	if jd.VariablesString() != other.VariablesString() {
		return fmt.Errorf("expected job variables %s but was %s", jd.VariablesString(), other.VariablesString())
	}
	if len(jd.Tasks) != len(other.Tasks) {
		return fmt.Errorf("expected number of tasks %v but was %v", len(jd.Tasks), len(other.Tasks))
	}
	for _, t := range other.Tasks {
		localTask := jd.lookupTasks.GetObject(t.TaskType)
		if localTask == nil {
			return fmt.Errorf("failed to find task for %s", t.TaskType)
		} else if err := t.Equals(localTask.(*TaskDefinition)); err != nil {
			return err
		}
	}
	return nil
}

// AfterLoad initializes job-definition
func (jd *JobDefinition) AfterLoad(key []byte) (err error) {
	nameValueVariables := make(map[string]interface{})
	jd.lookupTasks = cutils.NewSafeMap()
	jd.filter = ""
	for _, c := range jd.Variables {
		v, err := c.GetParsedValue()
		if err != nil {
			return err
		}
		if c.Name == keyResources {
			err = json.Unmarshal([]byte(c.Value), &jd.Resources)
			if err != nil {
				return err
			}
		} else if c.Name == keyRequiredParams {
			err = json.Unmarshal([]byte(c.Value), &jd.RequiredParams)
			if err != nil {
				return err
			}
		} else {
			nameValueVariables[c.Name] = v
		}
	}
	jd.NameValueVariables = nameValueVariables
	for _, t := range jd.Tasks {
		if err := t.AfterLoad(); err != nil {
			return err
		}
	}
	for _, cfg := range jd.Configs {
		if err = cfg.Decrypt(key); err != nil {
			return err
		}
	}
	if err = jd.Validate(); err != nil {
		return err
	}
	sort.Slice(jd.Tasks, func(i, j int) bool { return jd.Tasks[i].TaskOrder < jd.Tasks[j].TaskOrder })
	return nil
}

// SemanticVersionType type of sem-version
type SemanticVersionType int

// InvalidSemanticVersion - not valid
const InvalidSemanticVersion SemanticVersionType = 0

// ValidSemanticVersion valid
const ValidSemanticVersion SemanticVersionType = 1

// ValidSemanticDevRcVersion rc/dev version
const ValidSemanticDevRcVersion SemanticVersionType = 2

// CheckSemVersion validates sem-version
func (jd *JobDefinition) CheckSemVersion() (SemanticVersionType, error) {
	ver := strings.Split(jd.SemVersion, ".")
	if len(ver) < 2 {
		return InvalidSemanticVersion, fmt.Errorf("no major/minor plugin version, plugin version '%s' must use semantic version such as 1.2 or 1.0.1", jd.SemVersion)
	}
	//numericPattern := regexp.MustCompile(`^\d*-?(dev|rc)-?\d*$`)
	lastDigitNumeric := false
	for i := 0; i < len(ver); i++ {
		if i < len(ver)-1 {
			if digit, err := strconv.Atoi(ver[i]); err != nil || digit < 0 {
				return InvalidSemanticVersion, fmt.Errorf("non-numeric major/minor plugin version (%s), plugin version '%s' must use semantic version such as 1.2 or 1.0.1 (%v)",
					ver[i], jd.SemVersion, err)
			}
		} else {
			if digit, err := strconv.Atoi(ver[i]); err == nil && digit >= 0 {
				lastDigitNumeric = true
			}
		}
	}
	if lastDigitNumeric {
		return ValidSemanticVersion, nil
	}
	numericDevRCPattern := regexp.MustCompile(`^\d*-?(dev|rc)-?\d*$`)
	if !numericDevRCPattern.MatchString(ver[len(ver)-1]) {
		return InvalidSemanticVersion, fmt.Errorf("bad last digit (%s), plugin version '%s' must use semantic version such as 1.2, or 1.0.1 or 1.0.1-dev",
			ver[len(ver)-1], jd.SemVersion)
	}
	return ValidSemanticDevRcVersion, nil
}

// Validate validates job-definition
func (jd *JobDefinition) Validate() (err error) {
	if jd.JobType == "" {
		return errors.New("jobType is not specified")
	}
	if jd.PublicPlugin && len(strings.Split(jd.JobType, ".")) < 3 {
		return errors.New("the plugin jobType must start organization bundle id such as io.formicary.test-job or com.xyz.test-job")
	}
	if jd.SemVersion != "" || jd.PublicPlugin {
		if _, err = jd.CheckSemVersion(); err != nil {
			return err
		}
	}
	if jd.CronTrigger != "" && cronexpr.MustParse(jd.CronTrigger).Next(time.Now()).IsZero() {
		return fmt.Errorf("cron expression %s is invalid", jd.CronTrigger)
	}
	if len(jd.Tasks) == 0 {
		return fmt.Errorf("tasks are not specified for %v", jd.JobType)
	}
	if len(jd.Tasks) > maxTasksPerJob {
		return fmt.Errorf("number of tasks cannot exceed %d %v", maxTasksPerJob, jd.JobType)
	}
	for _, t := range jd.Tasks {
		if err := t.Validate(); err != nil {
			return err
		}
	}
	jd.Tags = jd.buildTags()
	jd.Methods = jd.buildMethods()
	jd.filter = ""
	if jd.Methods == "" {
		return fmt.Errorf("methods not specified for job-definition")
	}
	if jd.RawYaml == "" {
		return fmt.Errorf("raw-yaml not specified")
	}
	jd.lookupTasks = cutils.NewSafeMap()
	if jd.MaxConcurrency <= 1 {
		jd.MaxConcurrency = 3
	}
	jd.UsesTemplate = strings.Contains(jd.RawYaml, "{{") && strings.Contains(jd.RawYaml, "}}")
	if err = jd.validateTaskExitCodes(); err != nil {
		return err
	}

	if _, err = jd.GetFirstTask(); err != nil {
		return err
	}
	return nil
}

// GetLastAlwaysRunTasks There can be multiple always run tasks
func (jd *JobDefinition) GetLastAlwaysRunTasks() (alwaysRun []*TaskDefinition) {
	if jd.Tasks == nil || len(jd.Tasks) == 0 {
		return nil
	}
	alwaysRun = make([]*TaskDefinition, 0)
	for _, t := range jd.Tasks {
		if t.AlwaysRun {
			alwaysRun = append(alwaysRun, t)
		}
	}
	return
}

// GetLastTask returns last task
func (jd *JobDefinition) GetLastTask() (last *TaskDefinition) {
	for _, t := range jd.Tasks {
		if len(t.OnExitCode) == 0 {
			last = t
		}
	}
	return
}

// GetFirstTask returns first task
func (jd *JobDefinition) GetFirstTask() (*TaskDefinition, error) {
	onExitTypes, err := jd.validateReachableTasks()
	if err != nil {
		return nil, err
	}
	return jd.validateFirstTask(onExitTypes)
}

// CronAndScheduleTime returns next schedule time when using cron expression
func (jd *JobDefinition) CronAndScheduleTime() string {
	if jd.CronTrigger == "" {
		return ""
	}
	nextTime, _ := jd.GetCronScheduleTimeAndUserKey()
	return fmt.Sprintf("%s (Next: %s)", jd.CronTrigger, nextTime.Format(time.RFC3339))
}

// GetCronScheduleTimeAndUserKey returns next schedule time when using cron expression
func (jd *JobDefinition) GetCronScheduleTimeAndUserKey() (*time.Time, string) {
	if jd.Paused {
		return nil, ""
	}
	return GetCronScheduleTimeAndUserKey(jd.JobType, jd.CronTrigger)
}

// GetCronScheduleTimeAndUserKey returns next schedule time when using cron expression
func GetCronScheduleTimeAndUserKey(jobType string, cronTrigger string) (*time.Time, string) {
	if cronTrigger == "" {
		return nil, ""
	}
	nextTime := cronexpr.MustParse(cronTrigger).Next(time.Now())
	if nextTime.IsZero() {
		return nil, ""
	}
	return &nextTime, fmt.Sprintf("%s-%s", jobType, nextTime.Format(time.RFC3339))
}

// ValidateBeforeSave validates job-definition
func (jd *JobDefinition) ValidateBeforeSave(key []byte) error {
	if err := jd.Validate(); err != nil {
		return err
	}
	if jd.Resources.ResourceType != "" {
		if _, err := jd.AddVariable(keyResources, jd.Resources); err != nil {
			return err
		}
	}
	if jd.RequiredParams != nil {
		if _, err := jd.AddVariable(keyRequiredParams, jd.RequiredParams); err != nil {
			return err
		}
	}

	// Update configs
	{
		nameValueVariables, err := utils.ParseNameValueConfigs(jd.NameValueVariables)
		if err != nil {
			return err
		}
		jd.NameValueVariables = nameValueVariables
		for n, v := range nameValueVariables {
			if _, err := jd.AddVariable(n, v); err != nil {
				return err
			}
		}
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

	return nil
}

// JobDefinitionConfig defines variables for job definition
type JobDefinitionConfig struct {
	//gorm.Model
	// Inheriting name, value, type
	common.NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobDefinitionID defines foreign key for JobDefinition
	JobDefinitionID string `yaml:"-" json:"job_definition_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides default table name
func (JobDefinitionConfig) TableName() string {
	return "formicary_job_definition_configs"
}

// NewJobDefinitionConfig creates new job variable
func NewJobDefinitionConfig(name string, value interface{}, secret bool) (*JobDefinitionConfig, error) {
	nv, err := common.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionConfig{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

func (u *JobDefinitionConfig) String() string {
	return fmt.Sprintf("%s=%s", u.Name, u.Value)
}

// ValidateBeforeSave validates before save
func (u *JobDefinitionConfig) ValidateBeforeSave(key []byte) error {
	if err := u.Validate(); err != nil {
		return err
	}
	return u.Encrypt(key)
}

// Validate validates job-config
func (u *JobDefinitionConfig) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.ID != "" && u.JobDefinitionID == "" {
		err = errors.New("job-definition-id is not specified")
		u.Errors["JobDefinitionID"] = err.Error()
	}
	if u.Name == "" {
		err = errors.New("name is not specified")
		u.Errors["Name"] = err.Error()
	}
	if u.Type == "" {
		err = errors.New("type is not specified")
		u.Errors["Type"] = err.Error()
	}
	if u.Value == "" {
		err = errors.New("value is not specified")
		u.Errors["Value"] = err.Error()
	}
	if len(u.Value) > maxConfigValueLength {
		err = errors.New("value is too big")
		u.Errors["Value"] = err.Error()
	}
	return
}

// JobDefinitionVariable defines variables for job definition
type JobDefinitionVariable struct {
	//gorm.Model
	// Inheriting name, value, type
	common.NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobDefinitionID defines foreign key for JobDefinition
	JobDefinitionID string `yaml:"-" json:"job_definition_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
}

// TableName overrides default table name
func (JobDefinitionVariable) TableName() string {
	return "formicary_job_definition_variables"
}

// NewJobDefinitionVariable creates new job variable
func NewJobDefinitionVariable(name string, value interface{}) (*JobDefinitionVariable, error) {
	nv, err := common.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionVariable{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (jd *JobDefinition) tasksString() string {
	var b strings.Builder
	for _, t := range jd.Tasks {
		b.WriteString(t.String())
	}
	b.WriteString(";")
	return b.String()
}

func (jd *JobDefinition) validateFirstTask(
	onExitTypes map[string]bool) (firstTask *TaskDefinition, err error) {
	for _, t := range jd.Tasks {
		if !onExitTypes[t.TaskType] && firstTask == nil &&
			(len(jd.Tasks) == 1 || t.HasNext()) {
			firstTask = t
		} else if !onExitTypes[t.TaskType] {
			return nil, fmt.Errorf("task %v is not reachable, first task %v -- %v",
				t.TaskType, firstTask, onExitTypes)
		}
	}
	if firstTask == nil {
		err = fmt.Errorf("no first task found with onExitTypes %v", onExitTypes)
	}
	return
}

func (jd *JobDefinition) validateReachableTasks() (map[string]bool, error) {
	onExitTypes := make(map[string]bool)
	reservedExitCodes := map[string]bool{
		string(common.FATAL): true,
	}
	// validate all tasks are reachable
	for _, t := range jd.Tasks {
		for _, next := range t.OnExitCode {
			if next == "" {
				return nil, fmt.Errorf("empty task target for %v", t.TaskType)
			}
			if reservedExitCodes[next] {
				continue
			}
			if jd.lookupTasks.GetObject(next) == nil {
				return nil, fmt.Errorf("task '%s' refers to '%s' on-exit but it's not defined (%d)",
					t.TaskType, next, jd.lookupTasks.Len())
			}
			onExitTypes[next] = true
		}
	}
	return onExitTypes, nil
}

func (jd *JobDefinition) validateTaskExitCodes() error {
	tasksWithoutExitCodes := make(map[string]bool)
	jd.lock.Lock()
	defer jd.lock.Unlock()
	for _, t := range jd.Tasks {
		if err := t.Validate(); err != nil {
			return err
		}
		if jd.lookupTasks.GetObject(t.TaskType) != nil {
			return fmt.Errorf("duplicate tasks of type %v", t.TaskType)
		}
		if t.OnCompleted != "" {
			t.OnExitCode[common.COMPLETED] = t.OnCompleted
		}
		if t.OnFailed != "" {
			t.OnExitCode[common.FAILED] = t.OnFailed
		}
		// handle optional tasks that can fail without failing entire job
		if t.AllowFailure && t.OnExitCode[common.FAILED] == "" && t.OnExitCode[common.COMPLETED] != "" {
			t.OnExitCode[common.FAILED] = t.OnExitCode[common.COMPLETED]
		}
		jd.lookupTasks.SetObject(t.TaskType, t)
		if !t.HasNext() {
			tasksWithoutExitCodes[t.TaskType] = true
		}
	}

	if len(tasksWithoutExitCodes) == 0 {
		return fmt.Errorf("no leaf task found")
	}

	if len(tasksWithoutExitCodes) > 1 {
		return fmt.Errorf("multiple leaf tasks found %v", tasksWithoutExitCodes)
	}
	return nil
}

// buildMethods sets methods from tasks
func (jd *JobDefinition) buildMethods() string {
	taskMethods := make(map[common.TaskMethod]bool)
	for _, t := range jd.Tasks {
		if t.Method != "" {
			taskMethods[t.Method] = true
		} else if t.Method != "" {
			taskMethods[t.Method] = true
		}
	}
	var buf strings.Builder
	for k := range taskMethods {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("%v", k))
	}
	return buf.String()
}

// buildTags sets tags from tasks
func (jd *JobDefinition) buildTags() string {
	taskTags := make(map[string]bool)
	for _, t := range jd.Tasks {
		if t.Tags != nil {
			for _, c := range t.Tags {
				taskTags[c] = true
			}
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

func removeTemplateVariables(
	yamlSource string) (partialYaml string, err error) {
	var templateRegex, emptyLineRegex *regexp.Regexp
	if templateRegex, err = regexp.Compile(`{{[\d\s\w=\.\?\&_\-\/\+\*\$\^\(\)\[\]\\\|!@#%,;:'"]+}}`); err != nil {
		return "", err
	}
	if emptyLineRegex, err = regexp.Compile(`(?m)^\s*$[\r\n]*|[\r\n]+\s+\z`); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, line := range strings.Split(yamlSource, "\n") {
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	partialYaml = templateRegex.ReplaceAllString(sb.String(), "")
	partialYaml = emptyLineRegex.ReplaceAllString(partialYaml, "")
	return partialYaml, nil
}
