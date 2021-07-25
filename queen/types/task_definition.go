package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	cutils "plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/queen/utils"

	common "plexobject.com/formicary/internal/types"
)

const keyHeaders = "headers"
const keyScript = "script"
const keyBeforeScript = "before_script"
const keyAfterScript = "after_script"
const keyExecutorOptions = "executor_options"
const keyResources = "resources"
const keyTags = "tags"
const keyExcept = "except"
const keyJobVersion = "job_version"
const keyDeps = "dependencies"
const keyArtifacts = "artifact_ids"

// TaskDefinition defines a unit of work that is processed by the ant followers.
// The task definition represents definition of the task and instance of the task uses TaskExecution when a new
// job is submitted and executed. Based on the definition, a task request is sent to remote ant follower
// that supports method and tags of the task. A task response is then received and results are saved in
// the database.
type TaskDefinition struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobDefinitionID defines foreign key for JobDefinition
	JobDefinitionID string `yaml:"-" json:"job_definition_id"`
	// TaskType defines type of task
	TaskType string `yaml:"task_type" json:"task_type"`
	// Method TaskMethod defines method of communication
	Method common.TaskMethod `yaml:"method" json:"method"`
	// Description of task
	Description string `yaml:"description,omitempty" json:"description"`
	HostNetwork string `json:"host_network,omitempty" yaml:"host_network,omitempty" gorm:"-"`
	// AllowFailure means the task is optional and can fail without failing entire job
	AllowFailure bool `yaml:"allow_failure,omitempty" json:"allow_failure"`
	// AllowStartIfCompleted  means the task is always run on retry even if it was completed successfully
	AllowStartIfCompleted bool `yaml:"allow_start_if_completed,omitempty" json:"allow_start_if_completed"`
	// AlwaysRun means the task is always run on execution even if the job fails. For example, a required task fails (without
	// AllowFailure), the job is aborted and remaining tasks are skipped but a task defined as `AlwaysRun` is run even if the job fails.
	AlwaysRun bool `yaml:"always_run,omitempty" json:"always_run"`
	// Timeout defines max time a task should take, otherwise the job is aborted
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	// Retry defines max number of tries a task can be retried where it re-runs failed tasks
	Retry int `yaml:"retry,omitempty" json:"retry"`
	// DelayBetweenRetries defines time between retry of task
	DelayBetweenRetries time.Duration `yaml:"delay_between_retries,omitempty" json:"delay_between_retries"`
	// OnExitCodeSerialized defines next task to execute
	OnExitCodeSerialized string                         `yaml:"-" json:"-"`
	OnExitCode           map[common.RequestState]string `yaml:"on_exit_code,omitempty" json:"on_exit_code" gorm:"-"`
	OnCompleted          string                         `yaml:"on_completed,omitempty" json:"on_completed" gorm:"on_completed"`
	OnFailed             string                         `yaml:"on_failed,omitempty" json:"on_failed" gorm:"on_failed"`
	// Variables defines properties of task
	Variables []*TaskDefinitionVariable `yaml:"-" json:"-" gorm:"ForeignKey:TaskDefinitionID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
	TaskOrder int       `yaml:"-" json:"-" gorm:"task_order"`
	// Transient properties -- these are populated when AfterLoad or Validate is called
	NameValueVariables interface{}       `yaml:"variables,omitempty" json:"variables" gorm:"-"`
	Headers            map[string]string `yaml:"headers,omitempty" json:"headers" gorm:"-"`
	BeforeScript       []string          `yaml:"before_script,omitempty" json:"before_script" gorm:"-"`
	AfterScript        []string          `yaml:"after_script,omitempty" json:"after_script" gorm:"-"`
	Script             []string          `yaml:"script,omitempty" json:"script" gorm:"-"`
	Resources          BasicResource     `yaml:"resources,omitempty" json:"resources" gorm:"-"`
	// Tags are used to use specific followers that support the tags defined by ants.
	// For example, you may start a follower that processes payments and the task will be routed to that follower
	Tags            []string `yaml:"tags,omitempty" json:"tags" gorm:"-"`
	Except          string   `yaml:"except,omitempty" json:"except" gorm:"-"`
	JobVersion      string   `yaml:"job_version,omitempty" json:"job_version" gorm:"-"`
	Dependencies    []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty" gorm:"-"`
	ArtifactIDs     []string `json:"artifact_ids,omitempty" yaml:"artifact_ids,omitempty" gorm:"-"`
	unknownKeys     map[string]interface{}
	lookupVariables *cutils.SafeMap
	lock            sync.RWMutex
}

// NewTaskDefinition creates new instance of task-definition
func NewTaskDefinition(
	taskType string,
	method common.TaskMethod) *TaskDefinition {
	return &TaskDefinition{
		TaskType:           taskType,
		Method:             method,
		Variables:          make([]*TaskDefinitionVariable, 0),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		OnExitCode:         make(map[common.RequestState]string, 0),
		lookupVariables:    cutils.NewSafeMap(),
		NameValueVariables: make(map[string]interface{}),
	}
}

// TableName overrides default table name
func (*TaskDefinition) TableName() string {
	return "formicary_task_definitions"
}

// String provides short summary of task
func (td *TaskDefinition) String() string {
	return fmt.Sprintf("TaskType=%s Script=%v Variables=%s OnCompleted=%s OnFailed=%s OnExit=%s",
		td.TaskType, td.ScriptString(), td.VariablesString(), td.OnCompleted, td.OnFailed, td.OnExitCodeSerialized)
}

// ScriptString - text view of script
func (td *TaskDefinition) ScriptString() string {
	var b strings.Builder
	for _, c := range td.Script {
		b.WriteString(c + ",")
	}
	return b.String()
}

// VariablesString - text view of variables
func (td *TaskDefinition) VariablesString() string {
	var b strings.Builder
	for _, c := range td.Variables {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	return b.String()
}

// GetVariable gets job variable
func (td *TaskDefinition) GetVariable(name string) *TaskDefinitionVariable {
	old := td.lookupVariables.GetObject(name)
	if old == nil {
		return nil
	}
	return old.(*TaskDefinitionVariable)
}

// IsExcept evaluates Except
func (td *TaskDefinition) IsExcept() bool {
	return strings.Contains(td.Except, "true")
}

// SetAlwaysRun sets always run
func (td *TaskDefinition) SetAlwaysRun() *TaskDefinition {
	td.AlwaysRun = true
	return td
}

// AddVariable adds task variable
func (td *TaskDefinition) AddVariable(
	name string,
	value interface{}) (*TaskDefinitionVariable, error) {
	variable, err := NewTaskDefinitionVariable(name, value)
	if err != nil {
		return nil, err
	}
	if td.lookupVariables == nil {
		td.lookupVariables = cutils.NewSafeMap()
	}
	if td.NameValueVariables == nil {
		td.NameValueVariables = make(map[string]interface{})
	}
	if name == keyHeaders {
		td.Headers = value.(map[string]string)
	} else if name == keyBeforeScript {
		td.BeforeScript = value.([]string)
	} else if name == keyAfterScript {
		td.AfterScript = value.([]string)
	} else if name == keyScript {
		td.Script = value.([]string)
		//} else if name == keyExecutorOptions {
		//	td.ExecutorOptions = value.(common.ExecutorOptions)
	} else if name == keyResources {
		td.Resources = value.(BasicResource)
	} else if name == keyTags {
		td.Tags = value.([]string)
	} else if name == keyExcept {
		td.Except = fmt.Sprintf("%s", value)
	} else if name == keyJobVersion {
		td.JobVersion = fmt.Sprintf("%s", value)
	} else if name == keyDeps {
		td.Dependencies = value.([]string)
	} else if name == keyArtifacts {
		td.ArtifactIDs = value.([]string)
	} else {
		td.lock.Lock()
		defer td.lock.Unlock()
		nameValueVariables := td.NameValueVariables.(map[string]interface{})
		nameValueVariables[name] = value
	}
	variable.TaskDefinitionID = td.ID
	if td.lookupVariables.GetObject(name) == nil {
		td.Variables = append(td.Variables, variable)
	} else {
		for _, next := range td.Variables {
			if next.Name == name {
				next.Value = variable.Value
			}
		}
	}
	td.lookupVariables.SetObject(name, variable)
	return variable, nil
}

// Equals compares other task-definition for equality
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
		return fmt.Errorf("expected number of task variables %v but was %v\nvariables : %v\ntheirs: %v",
			len(td.Variables), len(other.Variables), td.VariablesString(), other.VariablesString())
	}
	for _, c := range other.Variables {
		localVariable := td.lookupVariables.GetObject(c.Name)
		if localVariable == nil {
			return fmt.Errorf("failed to find task variable for %s as %s", c.Name, c.Value)
		} else if localVariable.(*TaskDefinitionVariable).Value != c.Value {
			return fmt.Errorf("expected task variable for %s as %v but was %s", c.Name, localVariable, c.Value)
		}
	}
	return nil
}

// FilteredVariables returns variables that are not reserved
func (td *TaskDefinition) FilteredVariables() (variables []*TaskDefinitionVariable) {
	variables = make([]*TaskDefinitionVariable, 0)
	for _, c := range td.Variables {
		if !isReservedConfigProperties(c.Name) {
			variables = append(variables, c)
		}
	}
	return
}

// AfterLoad populates variables
func (td *TaskDefinition) AfterLoad() error {
	var err error
	_, err = td.LoadOnExitCode()
	if err != nil {
		return err
	}

	td.lookupVariables = cutils.NewSafeMap()
	nameValueVariables := make(map[string]interface{})

	for _, c := range td.Variables {
		v, err := c.GetParsedValue()
		if err != nil {
			return err
		}
		td.lookupVariables.SetObject(c.Name, c)
		if c.Name == keyHeaders {
			td.Headers = make(map[string]string)
			err = json.Unmarshal([]byte(c.Value), &td.Headers)
			if err != nil {
				return err
			}
		} else if c.Name == keyBeforeScript {
			td.BeforeScript = make([]string, 0)
			err = json.Unmarshal([]byte(c.Value), &td.BeforeScript)
			if err != nil {
				return err
			}
		} else if c.Name == keyAfterScript {
			td.AfterScript = make([]string, 0)
			err = json.Unmarshal([]byte(c.Value), &td.AfterScript)
			if err != nil {
				return err
			}
		} else if c.Name == keyScript {
			td.Script = make([]string, 0)
			err = json.Unmarshal([]byte(c.Value), &td.Script)
			if err != nil {
				return err
			}
		} else if c.Name == keyResources {
			err = json.Unmarshal([]byte(c.Value), &td.Resources)
			if err != nil {
				return err
			}
		} else if c.Name == keyTags {
			err = json.Unmarshal([]byte(c.Value), &td.Tags)
			if err != nil {
				return err
			}
		} else if c.Name == keyExcept {
			err = json.Unmarshal([]byte(c.Value), &td.Except)
			if err != nil {
				return err
			}
		} else if c.Name == keyJobVersion {
			err = json.Unmarshal([]byte(c.Value), &td.JobVersion)
			if err != nil {
				return err
			}
		} else if c.Name == keyDeps {
			err = json.Unmarshal([]byte(c.Value), &td.Dependencies)
			if err != nil {
				return err
			}
		} else if c.Name == keyArtifacts {
			err = json.Unmarshal([]byte(c.Value), &td.ArtifactIDs)
			if err != nil {
				return err
			}
		} else {
			nameValueVariables[c.Name] = v
		}
	}
	td.NameValueVariables = nameValueVariables
	return nil
}

// Validate validates task
func (td *TaskDefinition) Validate() error {
	if td.TaskType == "" {
		return errors.New("taskType is not specified")
	}
	//td.Method = td.ExecutorOptions.Method
	if td.Method == "" {
		td.Method = common.Kubernetes
		//return fmt.Errorf("method is not specified for %s", td.TaskType)
	}
	if !td.Method.IsValid() {
		return fmt.Errorf("method %s is not supported for %s", td.Method, td.TaskType)
	}
	if td.Tags != nil {
		for i := 0; i < len(td.Tags); i++ {
			td.Tags[i] = strings.ToLower(td.Tags[i])
		}
	}
	// TODO added here because deserialization doesn't initialize on-exit
	if td.OnExitCode == nil {
		td.OnExitCode = make(map[common.RequestState]string)
	}
	return nil
}

// ValidateBeforeSave validates task
func (td *TaskDefinition) ValidateBeforeSave() error {
	if err := td.Validate(); err != nil {
		return err
	}
	if td.Headers != nil && len(td.Headers) > 0 {
		_, _ = td.AddVariable(keyHeaders, td.Headers)
	}
	if td.BeforeScript != nil && len(td.BeforeScript) > 0 {
		if _, err := td.AddVariable(keyBeforeScript, td.BeforeScript); err != nil {
			return err
		}
	}
	if td.AfterScript != nil && len(td.AfterScript) > 0 {
		if _, err := td.AddVariable(keyAfterScript, td.AfterScript); err != nil {
			return err
		}
	}
	if td.Script != nil && len(td.Script) > 0 {
		if _, err := td.AddVariable(keyScript, td.Script); err != nil {
			return err
		}
	}
	if td.Resources.ResourceType != "" {
		if _, err := td.AddVariable(keyResources, td.Resources); err != nil {
			return err
		}
	}
	if td.Tags != nil {
		if _, err := td.AddVariable(keyTags, td.Tags); err != nil {
			return err
		}
	}
	if td.Except != "" {
		if _, err := td.AddVariable(keyExcept, td.Except); err != nil {
			return err
		}
	}
	if td.JobVersion != "" {
		if _, err := td.AddVariable(keyJobVersion, td.JobVersion); err != nil {
			return err
		}
	}
	if td.Dependencies != nil {
		if _, err := td.AddVariable(keyDeps, td.Dependencies); err != nil {
			return err
		}
	}
	if td.ArtifactIDs != nil {
		if _, err := td.AddVariable(keyArtifacts, td.ArtifactIDs); err != nil {
			return err
		}
	}
	err := td.addVariablesFromNameValueVariables()
	if err != nil {
		return err
	}
	_, err = td.SaveOnExitCode()
	return err
}

func (td *TaskDefinition) addVariablesFromNameValueVariables() error {
	nameValueVariables, err := utils.ParseNameValueConfigs(td.NameValueVariables)
	if err != nil {
		return fmt.Errorf("failed to parse variables %v due to %v", td.NameValueVariables, err)
	}
	td.NameValueVariables = nameValueVariables
	for n, v := range nameValueVariables {
		if _, err := td.AddVariable(n, v); err != nil {
			return err
		}
	}
	return nil
}

// AddExitCode adds exit code
func (td *TaskDefinition) AddExitCode(status string, task string) *TaskDefinition {
	td.OnExitCode[common.NewRequestState(status)] = task
	return td
}

// LoadOnExitCode initializes OnExitCode from serialized property
func (td *TaskDefinition) LoadOnExitCode() (map[common.RequestState]string, error) {
	if len(td.OnExitCodeSerialized) > 0 {
		onExitCode := make(map[string]string)
		err := json.Unmarshal([]byte(td.OnExitCodeSerialized), &onExitCode)
		if err != nil {
			return nil, err
		}
		td.OnExitCode = make(map[common.RequestState]string)
		for k, v := range onExitCode {
			td.OnExitCode[common.NewRequestState(k)] = v
		}
	} else {
		if td.OnExitCode == nil {
			td.OnExitCode = make(map[common.RequestState]string)
		}
		for k, v := range td.OnExitCode {
			td.OnExitCode[common.NewRequestState(string(k))] = v // upper-case
		}
	}

	if td.OnCompleted != "" {
		td.OnExitCode[common.COMPLETED] = td.OnCompleted
	}
	if td.OnFailed != "" {
		td.OnExitCode[common.FAILED] = td.OnFailed
	}
	return td.OnExitCode, nil
}

// GetDelayBetweenRetries between retries
func (td *TaskDefinition) GetDelayBetweenRetries() time.Duration {
	if td.DelayBetweenRetries <= 0 {
		td.DelayBetweenRetries = time.Second * time.Duration(rand.Intn(2)+1)
	}
	return td.DelayBetweenRetries
}

// HasNext returns true if task has next task to run
func (td *TaskDefinition) HasNext() bool {
	return len(td.OnExitCode) > 0 || td.OnCompleted != "" || td.OnFailed != ""
}

// IsFatalError checks if error is fatal
func (td *TaskDefinition) IsFatalError(exitCode string) bool {
	if td.OnExitCode == nil || len(td.OnExitCode) == 0 {
		return false
	}
	nextTaskName := td.OnExitCode[common.NewRequestState(exitCode)]
	return common.NewRequestState(nextTaskName) == common.FATAL
}

// SaveOnExitCode stores serialized OnExitCode
func (td *TaskDefinition) SaveOnExitCode() (string, error) {
	if len(td.OnExitCode) > 0 {
		b, err := json.Marshal(td.OnExitCode)
		if err != nil {
			return "", err
		}
		td.OnExitCodeSerialized = string(b)
	} else {
		td.OnExitCodeSerialized = ""
	}
	return td.OnExitCodeSerialized, nil
}

// TaskDefinitionVariable defines variable for task definition
type TaskDefinitionVariable struct {
	//gorm.Model
	// Inheriting name, value, type
	common.NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// TaskDefinitionID defines foreign key for task-definition
	TaskDefinitionID string `yaml:"-" json:"task_definition_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
}

// TableName overrides default table name
func (TaskDefinitionVariable) TableName() string {
	return "formicary_task_definition_variables"
}

// NewTaskDefinitionVariable creates new task variable
func NewTaskDefinitionVariable(
	name string,
	value interface{}) (*TaskDefinitionVariable, error) {
	nv, err := common.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &TaskDefinitionVariable{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

func getReservedConfigProperties() []string {
	return []string{
		keyHeaders,
		keyAfterScript,
		keyBeforeScript,
		keyScript,
		keyResources,
		keyRequiredParams,
		keyExecutorOptions,
		keyTags,
		keyExcept,
		keyJobVersion,
		keyDeps,
		keyArtifacts}
}

func isReservedConfigProperties(name string) bool {
	for _, reserved := range getReservedConfigProperties() {
		if reserved == name {
			return true
		}
	}
	return false
}
