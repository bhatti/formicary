## High level Architecture

The [Formicary](https://github.com/bhatti/formicary) architecture is a complex system designed for task orchestration 
and execution, based on the Leader-Follower, SEDA and Fork/Join patterns.

### 3.1 Design Patterns

Here are some common design patterns used in the [Formicary](https://github.com/bhatti/formicary) architecture:

 -  **Microservices Architecture**: [Formicary](https://github.com/bhatti/formicary) architecture is decomposed into smaller, independent services that enhances scalability and facilitates independent deployment and updates.
 -  **Pipeline Pattern**: It structures the processing of tasks in a linear sequence of processing steps (stages).
 -  **Distributed Task Queues**: It manages task distribution among multiple worker nodes. This ensures load balancing and effective utilization of resources.
 -  **Event-Driven Architecture**: [Formicary](https://github.com/bhatti/formicary) components communicate with events, triggering actions based on event occurrence for handling asynchronous processes and integrating various services.
 -  **Load Balancer Pattern**: It distributes incoming requests or tasks evenly across a pool of servers and prevents any single server from becoming a bottleneck.
 -  **Circuit Breaker Pattern**: It prevents a system from repeatedly trying to execute an operation that’s likely to fail.
 -  **Retry Pattern**: It automatically re-attempts failed operations a certain number of times before considering the operation failed.
 -  **Observer Pattern**: [Formicary](https://github.com/bhatti/formicary) uses observer pattern for monitoring, logging, and metrics collection.
 -  **Scheduler-Agent-Supervisor Pattern**: The [Formicary](https://github.com/bhatti/formicary) schedulers trigger tasks, agents to execute them, and supervisors to monitor task execution.
 -   **Immutable Infrastructure**: It treats infrastructure entities as immutable, replacing them for each deployment instead of updating them.
 -   **Fork-Join Pattern**: It decomposes a task into sub-tasks, processes them in parallel, and then combines the results.
 -   **Caching Pattern**: It stores intermediate build artifacts such as npm/maven/gradle libraries in a readily accessible location to reduce latency and improves performance.
 -   **Back-Pressure Pattern**: It controls the rate of task generation or data flow to prevent overwhelming the system.
 -   **Idempotent Operations**: It ensures that an operation produces the same result even if it’s executed multiple times.
 -   **External Configuration Store Pattern**: It manages job configuration and settings in a separate, external location, enabling easier changes and consistency across services.
 -   **Blue-Green Deployment Pattern**: It manages deployment by switching between two identical environments, one running the current version (blue) and one running the new version (green).

### 3.2 High-level Components

The architecture of [Formicary](https://github.com/bhatti/formicary) is designed to manage and execute complex workflows where tasks are organized in a DAG structure. This architecture is inherently scalable and robust, catering to the needs of task scheduling, execution, and monitoring. Here’s an overview of its key functionalities and components:

![components diagram](https://weblog.plexobject.com/images/formicary_components.png)


#### 3.2.1 Functionalities

 -   **Job Processing**: [Formicary](https://github.com/bhatti/formicary) supports defining workflows as Job, where each node represents a task, and edges define dependencies. It ensures that tasks are executed in an order that respects their dependencies.
 -   **Task Distribution**: Tasks, defined as units of work, are distributed among ant-workers based on tags and executor protocols (Kubernetes, Docker, Shell, HTTP, Websockets, etc.).
 -   **Scalability**: Formicary scales to handle a large number of tasks and complex workflows. It supports horizontal scaling where more workers can be added to handle increased load.
 -   **Fault Tolerance and Reliability**: It handles failures and retries of tasks.
 -   **Extensibility**: It provides interfaces and plugins for extending its capabilities.
 -   **Resource Management**: Efficiently allocates resources for task execution, optimizing for performance and cost.
 -   **Resource Quotas**: It define maximum resource quotas for CPU, memory, disk space, and network usage for each job or task. This prevent any single job from over-consuming resources, ensuring fair resource allocation among all jobs.
 -   **Prioritization**: It prioritize jobs based on criticality or predefined rules.
 -   **Job Throttling**: It implement throttling mechanisms to control the rate at which jobs are fed into the system.
 -   **Kubernetes Clusters**: Formicary allows for the creation of kubernetes clusters to supports auto-scaling and termination to optimize resource usage and cost.
 -   **Monitoring and Logging**: It offers extensive monitoring and logging capabilities.
 -   **Authentication and Authorization**: Formicary enforces strict authentication and authorization based on OAuth 2.0 and OIDC protocols before allowing access to the system.
 -   **Multitenancy:** Formicary accommodates multiple tenants, allowing various organizations to sign up with one or more users, ensuring their data is safeguarded through robust authentication and authorization measures.
 -   **Common Plugins:** Formicary allows the sharing of common plugins that function as sub-jobs for reusable features, which other users can then utilize.

#### 3.2.2 Core Components

Following are core components of the [Formicary](https://github.com/bhatti/formicary) system:

##### API Controller

The API controller defines an API that supports the following functions:

 -   Checking the status of current, pending, or completed jobs
 -   Submitting new jobs for execution
 -   Looking up or modifying job specifications
 -   Enrolling ant workers and overseeing resources for processing
 -   Retrieving or uploading job-related artifacts
 -   Handling settings, error codes, and resource allocation
 -   Delivering both real-time and historical data reports

##### UI Controller

The UI controller offers the following features:

 -   Displaying ongoing, queued, or completed jobs
 -   Initiating new job submissions
 -   Reviewing job specifications or introducing new ones
 -   Supervising ant workers and execution units
 -   Accessing or submitting artifacts
 -   Configuring settings, error codes, and resource management
 -   Providing access to both live and archived reports

##### Resource Manager

The resource manager enrolls ant workers and monitors the resources accessible for processing jobs. Ant workers regularly inform the resource manager about their available capacity and current workload. This continuous communication allows the resource manager to assess the maximum number of jobs that can run simultaneously without surpassing the capacity of the workers.

##### Job Scheduler

The job scheduler examines the queue for jobs awaiting execution and consults the resource manager to determine if a job can be allocated for execution. When sufficient resources are confirmed to be available, it dispatches a remote command to the Job-Launcher to initiate the job’s execution. Please note that the formicary architecture allows for multiple server instances, with the scheduler operating on the leader node. Meanwhile, other servers host the job-launcher and executor components, which are responsible for executing and orchestrating jobs.

##### Job Launcher

The job launcher remains attentive to incoming requests for job execution and initiates the process by engaging the Job-Supervisor. The Job-Supervisor then takes on the role of overseeing the execution of the job, ensuring its successful completion.

##### Job Supervisor

The job supervisor initiates a job in an asynchronous manner and manages the job’s execution. It oversees each task through the Task-Supervisor and determines the subsequent task to execute, guided by the status or exit code of the previously completed task.

##### Task Supervisor

The task supervisor initiates task execution by dispatching a remote instruction to the ant worker equipped to handle the specific task method, then stands by for a response. Upon receiving the outcome, the task supervisor records the results in the database for future reference and analysis.

##### Ant Workers

An ant worker registers with the queen server by specifying the types of tasks it can handle, using specific methods or tags for identification. Once registered, it remains vigilant for task requests, processing each one asynchronously according to the execution protocols defined for each task, and then relaying the results back to the server. Before starting on a task, the ant worker ensures all required artifacts are gathered and then uploads them once the task is completed. Moreover, the ant worker is responsible for managing the lifecycle of any external containers, such as those in Docker and Kubernetes systems, from initiation to termination.

To maintain system efficiency and prevent any single worker from becoming overwhelmed, the ant worker consistently updates the queen server with its current workload and capacity. This mechanism allows for a balanced distribution of tasks, ensuring that no worker is overloaded. The architecture is scalable, allowing for the addition of more ant workers to evenly spread the workload. These workers communicate with the queen server through messaging queues, enabling them to:
 - Regularly update the server on their workload and capacity.
 - Download necessary artifacts needed for task execution.
 -   Execute tasks using the appropriate executors, such as Docker, HTTP, Kubernetes, Shell, or Websockets.
 -   Upload the resulting artifacts upon completion of tasks.
 -   Monitor and manage the lifecycle of Docker/Kubernetes containers, reporting back any significant events to the server.

##### Executors

### Executor
An executor abstracts the runtime environment for execution a task. The formicary uses method to define the type
of executor. Following executor methods are supported:

|     Executor |   Method |
| :----------: | :-----------: |
| Kubernetes Pods | KUBERNETES |
| Docker containers | DOCKER |
| Shell | SHELL |
| HTTP (GET POST, PUT, DELETE) | HTTP_GET HTTP_POST_FORM HTTP_POST_JSON HTTP_PUT_FORM HTTP_PUT_JSON HTTP_DELETE WEBSOCKET |
| Fork/Await | JOB_FORK, JOB_FORK_AWAIT |
| Artifact/Expiration | EXPIRE_ARTIFACTS |
| Messaging | MESSAGING |


**Note:** These execution methods can be easily extended to support other executor protocols to provide greater flexibility in how tasks are executed and integrated with different environments.

##### Database

The formicary system employs a relational database to systematically store and manage a wide array of data, including job requests, detailed job definitions, resource allocations, error codes, and various configurations.

##### **Artifacts** and Object Store

The formicary system utilizes an object storage solution to maintain the artifacts produced during task execution, those generated within the image cache, or those uploaded directly by users. This method ensures a scalable and secure way to keep large volumes of unstructured data, facilitating easy access and retrieval of these critical components for operational efficiency and user interaction.

##### **Messaging**

Messaging enables seamless interaction between the scheduler and the workers, guaranteeing dependable dissemination of tasks across distributed settings.

##### **Notification System**

The notification system dispatches alerts and updates regarding the pipeline status to users.

### 3.3 Data Model

Here’s an overview of its key data model in [Formicary](https://github.com/bhatti/formicary) system:

![Domain Classes](https://weblog.plexobject.com/images/formicary_classes.png)



#### 3.3.1 Job Definition

A JobDefinition outlines a set of tasks arranged in a Directed Acyclic Graph (DAG), executed by worker entities. The workflow progresses based on the exit codes of tasks, determining the subsequent task to execute. Each task definition encapsulates a job’s specifics, and upon receiving a new job request, an instance of this job is initiated through JobExecution.
```go

type JobDefinition struct {
// ID defines UUID for primary key
ID string \`yaml:"-" json:"id" gorm:"primary\_key"\`
// JobType defines a unique type of job
JobType string \`yaml:"job\_type" json:"job\_type"\`
// Version defines internal version of the job-definition, which is updated when a job is updated. The database
// stores each version as a separate row but only latest version is used for new jobs.
Version int32 \`yaml:"-" json:"-"\`
// SemVersion - semantic version is used for external version, which can be used for public plugins.
SemVersion string \`yaml:"sem\_version" json:"sem\_version"\`
// URL defines url for job
URL string \`json:"url"\`
// UserID defines user who updated the job
UserID string \`json:"user\_id"\`
// OrganizationID defines org who submitted the job
OrganizationID string \`json:"organization\_id"\`
// Description of job
Description string \`yaml:"description,omitempty" json:"description"\`
// Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering
Platform string \`yaml:"platform,omitempty" json:"platform"\`
// CronTrigger can be used to run the job periodically
CronTrigger string \`yaml:"cron\_trigger,omitempty" json:"cron\_trigger"\`
// Timeout defines max time a job should take, otherwise the job is aborted
Timeout time.Duration \`yaml:"timeout,omitempty" json:"timeout"\`
// Retry defines max number of tries a job can be retried where it re-runs failed job
Retry int \`yaml:"retry,omitempty" json:"retry"\`
// HardResetAfterRetries defines retry config when job is rerun and as opposed to re-running only failed tasks, all tasks are executed.
HardResetAfterRetries int \`yaml:"hard\_reset\_after\_retries,omitempty" json:"hard\_reset\_after\_retries"\`
// DelayBetweenRetries defines time between retry of job
DelayBetweenRetries time.Duration \`yaml:"delay\_between\_retries,omitempty" json:"delay\_between\_retries"\`
// MaxConcurrency defines max number of jobs that can be run concurrently
MaxConcurrency int \`yaml:"max\_concurrency,omitempty" json:"max\_concurrency"\`
// disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging.
Disabled bool \`yaml:"-" json:"disabled"\`
// PublicPlugin means job is public plugin
PublicPlugin bool \`yaml:"public\_plugin,omitempty" json:"public\_plugin"\`
// RequiredParams from job request (and plugin)
RequiredParams \[\]string \`yaml:"required\_params,omitempty" json:"required\_params" gorm:"-"\`
// Tags are used to use specific followers that support the tags defined by ants.
// Tags is aggregation of task tags
Tags string \`yaml:"tags,omitempty" json:"tags"\`
// Methods is aggregation of task methods
Methods string \`yaml:"methods,omitempty" json:"methods"\`
// Tasks defines one to many relationships between job and tasks, where a job defines
// a directed acyclic graph of tasks that are executed for the job.
Tasks \[\]\*TaskDefinition \`yaml:"tasks" json:"tasks" gorm:"ForeignKey:JobDefinitionID" gorm:"auto\_preload" gorm:"constraint:OnUpdate:CASCADE"\`
// Configs defines config properties of job that are used as parameters for the job template or task request when executing on a remote
// ant follower. Both config and variables provide similar capabilities but config can be updated for all job versions and can store
// sensitive data.
Configs \[\]\*JobDefinitionConfig \`yaml:"-" json:"-" gorm:"ForeignKey:JobDefinitionID" gorm:"auto\_preload" gorm:"constraint:OnUpdate:CASCADE"\`
// Variables defines properties of job that are used as parameters for the job template or task request when executing on a remote
// ant follower. Both config and variables provide similar capabilities but variables are part of the job yaml definition.
Variables \[\]\*JobDefinitionVariable \`yaml:"-" json:"-" gorm:"ForeignKey:JobDefinitionID" gorm:"auto\_preload" gorm:"constraint:OnUpdate:CASCADE"\`
// CreatedAt job creation time
CreatedAt time.Time \`yaml:"-" json:"created\_at"\`
// UpdatedAt job update time
UpdatedAt time.Time \`yaml:"-" json:"updated\_at"\`
}
```

#### 3.3.2 Task Definition

A TaskDefinition outlines the work performed by worker entities. It specifies the task’s parameters and, upon a new job request, a TaskExecution instance is initiated to carry out the task. The task details, including its method and tags, guide the dispatch of task requests to a compatible remote worker. Upon task completion, the outcomes are recorded in the database for reference.
```go

type TaskDefinition struct {
// ID defines UUID for primary key
ID string \`yaml:"-" json:"id" gorm:"primary\_key"\`
// JobDefinitionID defines foreign key for JobDefinition
JobDefinitionID string \`yaml:"-" json:"job\_definition\_id"\`
// TaskType defines type of task
TaskType string \`yaml:"task\_type" json:"task\_type"\`
// Method TaskMethod defines method of communication
Method common.TaskMethod \`yaml:"method" json:"method"\`
// Description of task
Description string \`yaml:"description,omitempty" json:"description"\`
// HostNetwork defines kubernetes/docker config for host\_network
HostNetwork string \`json:"host\_network,omitempty" yaml:"host\_network,omitempty" gorm:"-"\`
// AllowFailure means the task is optional and can fail without failing entire job
AllowFailure bool \`yaml:"allow\_failure,omitempty" json:"allow\_failure"\`
// AllowStartIfCompleted  means the task is always run on retry even if it was completed successfully
AllowStartIfCompleted bool \`yaml:"allow\_start\_if\_completed,omitempty" json:"allow\_start\_if\_completed"\`
// AlwaysRun means the task is always run on execution even if the job fails. For example, a required task fails (without
// AllowFailure), the job is aborted and remaining tasks are skipped but a task defined as \`AlwaysRun\` is run even if the job fails.
AlwaysRun bool \`yaml:"always\_run,omitempty" json:"always\_run"\`
// Timeout defines max time a task should take, otherwise the job is aborted
Timeout time.Duration \`yaml:"timeout,omitempty" json:"timeout"\`
// Retry defines max number of tries a task can be retried where it re-runs failed tasks
Retry int \`yaml:"retry,omitempty" json:"retry"\`
// DelayBetweenRetries defines time between retry of task
DelayBetweenRetries time.Duration \`yaml:"delay\_between\_retries,omitempty" json:"delay\_between\_retries"\`
// Webhook config
Webhook \*common.Webhook \`yaml:"webhook,omitempty" json:"webhook" gorm:"-"\`
// OnExitCodeSerialized defines next task to execute
OnExitCodeSerialized string \`yaml:"-" json:"-"\`
// OnExitCode defines next task to run based on exit code
OnExitCode map\[common.RequestState\]string \`yaml:"on\_exit\_code,omitempty" json:"on\_exit\_code" gorm:"-"\`
// OnCompleted defines next task to run based on completion
OnCompleted string \`yaml:"on\_completed,omitempty" json:"on\_completed" gorm:"on\_completed"\`
// OnFailed defines next task to run based on failure
OnFailed string \`yaml:"on\_failed,omitempty" json:"on\_failed" gorm:"on\_failed"\`
// Variables defines properties of task
Variables \[\]\*TaskDefinitionVariable \`yaml:"-" json:"-" gorm:"ForeignKey:TaskDefinitionID" gorm:"auto\_preload" gorm:"constraint:OnUpdate:CASCADE"\`
TaskOrder int       \`yaml:"-" json:"-" gorm:"task\_order"\`
// ReportStdout is used to send stdout as a report
ReportStdout bool \`yaml:"report\_stdout,omitempty" json:"report\_stdout"\`
// Transient properties -- these are populated when AfterLoad or Validate is called
NameValueVariables interface{} \`yaml:"variables,omitempty" json:"variables" gorm:"-"\`
// Header defines HTTP headers
Headers map\[string\]string \`yaml:"headers,omitempty" json:"headers" gorm:"-"\`
// BeforeScript defines list of commands that are executed before main script
BeforeScript \[\]string \`yaml:"before\_script,omitempty" json:"before\_script" gorm:"-"\`
// AfterScript defines list of commands that are executed after main script for cleanup
AfterScript \[\]string \`yaml:"after\_script,omitempty" json:"after\_script" gorm:"-"\`
// Script defines list of commands to execute in container
Script \[\]string \`yaml:"script,omitempty" json:"script" gorm:"-"\`
// Resources defines resources required by the task
Resources BasicResource \`yaml:"resources,omitempty" json:"resources" gorm:"-"\`
// Tags are used to use specific followers that support the tags defined by ants.
// For example, you may start a follower that processes payments and the task will be routed to that follower
Tags \[\]string \`yaml:"tags,omitempty" json:"tags" gorm:"-"\`
// Except is used to filter task execution based on certain condition
Except string \`yaml:"except,omitempty" json:"except" gorm:"-"\`
// JobVersion defines job version
JobVersion string \`yaml:"job\_version,omitempty" json:"job\_version" gorm:"-"\`
// Dependencies defines dependent tasks for downloading artifacts
Dependencies \[\]string \`json:"dependencies,omitempty" yaml:"dependencies,omitempty" gorm:"-"\`
// ArtifactIDs defines id of artifacts that are automatically downloaded for job-execution
ArtifactIDs \[\]string \`json:"artifact\_ids,omitempty" yaml:"artifact\_ids,omitempty" gorm:"-"\`
// ForkJobType defines type of job to work
ForkJobType string \`json:"fork\_job\_type,omitempty" yaml:"fork\_job\_type,omitempty" gorm:"-"\`
// URL to use
URL string \`json:"url,omitempty" yaml:"url,omitempty" gorm:"-"\`
// AwaitForkedTasks defines list of jobs to wait for completion
AwaitForkedTasks      \[\]string \`json:"await\_forked\_tasks,omitempty" yaml:"await\_forked\_tasks,omitempty" gorm:"-"\`
MessagingRequestQueue string   \`json:"messaging\_request\_queue,omitempty" yaml:"messaging\_request\_queue,omitempty" gorm:"-"\`
MessagingReplyQueue   string   \`json:"messaging\_reply\_queue,omitempty" yaml:"messaging\_reply\_queue,omitempty" gorm:"-"\`
// CreatedAt job creation time
CreatedAt time.Time \`yaml:"-" json:"created\_at"\`
// UpdatedAt job update time
UpdatedAt time.Time \`yaml:"-" json:"updated\_at"\`  
}
```

#### 3.3.3 JobExecution

JobExecution refers to a specific instance of a job-definition that gets activated upon the submission of a job-request. When a job is initiated by the job-launcher, this triggers the creation of a job-execution instance, which is also recorded in the database. Following this initiation, the job-launcher transfers responsibility for the job to the job-supervisor, which then commences execution, updating the status of both the job request and execution to EXECUTING. The job supervisor manages the execution process, ultimately altering the status to COMPLETED or FAILED upon completion. Throughout this process, the formicary system emits job lifecycle events to reflect these status changes, which can be monitored by UI or API clients.

For every task outlined within the task-definition associated with the JobExecution, a corresponding TaskExecution instance is generated. This setup tracks the progress and state of both job and task executions within a database, and any outputs generated during the job execution process are preserved in object storage.
```go

type JobExecution struct {
// ID defines UUID for primary key
ID string \`json:"id" gorm:"primary\_key"\`
// JobRequestID defines foreign key for job request
JobRequestID uint64 \`json:"job\_request\_id"\`
// JobType defines type for the job
JobType    string \`json:"job\_type"\`
JobVersion string \`json:"job\_version"\`
// JobState defines state of job that is maintained throughout the lifecycle of a job
JobState types.RequestState \`json:"job\_state"\`
// OrganizationID defines org who submitted the job
OrganizationID string \`json:"organization\_id"\`
// UserID defines user who submitted the job
UserID string \`json:"user\_id"\`
// ExitCode defines exit status from the job execution
ExitCode string \`json:"exit\_code"\`
// ExitMessage defines exit message from the job execution
ExitMessage string \`json:"exit\_message"\`
// ErrorCode captures error code at the end of job execution if it fails
ErrorCode string \`json:"error\_code"\`
// ErrorMessage captures error message at the end of job execution if it fails
ErrorMessage string \`json:"error\_message"\`
// Contexts defines context variables of job
Contexts \[\]\*JobExecutionContext \`json:"contexts" gorm:"ForeignKey:JobExecutionID" gorm:"auto\_preload"\`
// Tasks defines list of tasks that are executed for the job
Tasks \[\]\*TaskExecution \`json:"tasks" gorm:"ForeignKey:JobExecutionID" gorm:"auto\_preload"\`
// StartedAt job execution start time
StartedAt time.Time \`json:"started\_at"\`
// EndedAt job execution end time
EndedAt \*time.Time \`json:"ended\_at"\`
// UpdatedAt job execution last update time
UpdatedAt time.Time \`json:"updated\_at"\`
// CPUSecs execution time
CPUSecs int64 \`json:"cpu\_secs"\`
}
```

The state of job execution includes: PENDING, READY, COMPLETED, FAILED, EXECUTING, STARTED, PAUSED, and CANCELLED.

#### 3.3.4 TaskExecution

TaskExecution records the execution of a task or a unit of work, carried out by ant-workers in accordance with the specifications of the task-definition. It captures the status and the outputs produced by the task execution, storing them in the database and the object-store. When a task begins, it is represented by a task-execution instance, initiated by the task supervisor. This instance is stored in the database by the task supervisor, which then assembles a task request to dispatch to a remote ant worker. The task supervisor awaits the worker’s response before updating the database with the outcome. Task execution concludes with either a COMPLETED or FAILED status, and it also accommodates an exit code provided by the worker. Based on the final status or exit code, orchestration rules determine the subsequent task to execute.
```go

type TaskExecution struct {
// ID defines UUID for primary key
ID string \`json:"id" gorm:"primary\_key"\`
// JobExecutionID defines foreign key for JobExecution
JobExecutionID string \`json:"job\_execution\_id"\`
// TaskType defines type of task
TaskType string \`json:"task\_type"\`
// Method defines method of communication
Method types.TaskMethod \`yaml:"method" json:"method"\`
// TaskState defines state of task that is maintained throughout the lifecycle of a task
TaskState types.RequestState \`json:"task\_state"\`
// AllowFailure means the task is optional and can fail without failing entire job
AllowFailure bool \`json:"allow\_failure"\`
// ExitCode defines exit status from the job execution
ExitCode string \`json:"exit\_code"\`
// ExitMessage defines exit message from the job execution
ExitMessage string \`json:"exit\_message"\`
// ErrorCode captures error code at the end of job execution if it fails
ErrorCode string \`json:"error\_code"\`
// ErrorMessage captures error message at the end of job execution if it fails
ErrorMessage string \`json:"error\_message"\`
// FailedCommand captures command that failed
FailedCommand string \`json:"failed\_command"\`
// AntID - id of ant with version
AntID string \`json:"ant\_id"\`
// AntHost - host where ant ran the task
AntHost string \`json:"ant\_host"\`
// Retried keeps track of retry attempts
Retried int \`json:"retried"\`
// Contexts defines context variables of task
Contexts \[\]\*TaskExecutionContext \`json:"contexts" gorm:"ForeignKey:TaskExecutionID" gorm:"auto\_preload"\`
// Artifacts defines list of artifacts that are generated for the task
Artifacts \[\]\*types.Artifact \`json:"artifacts" gorm:"ForeignKey:TaskExecutionID"\`
// TaskOrder
TaskOrder int \`json:"task\_order"\`
// CountServices
CountServices int \`json:"count\_services"\`
// CostFactor
CostFactor float64 \`json:"cost\_factor"\`
Stdout \[\]string \`json:"stdout" gorm:"-"\`
// StartedAt job creation time
StartedAt time.Time \`json:"started\_at"\`
// EndedAt job update time
EndedAt \*time.Time \`json:"ended\_at"\`
// UpdatedAt job execution last update time
UpdatedAt time.Time \`json:"updated\_at"\`
}
```

The state of TaskExecution includes READY, STARTED, EXECUTING, COMPLETED, and FAILED.

#### 3.3.5 JobRequest

JobRequest outlines a user’s request to execute a job as per its job-definition. Upon submission, a job-request is marked as PENDING in the database and later, it is asynchronously scheduled for execution by the job scheduler, depending on resource availability. It’s important to note that users have the option to schedule a job for a future date to avoid immediate execution. Additionally, a job definition can include a cron property, which automatically generates job requests at predetermined times for execution. Besides user-initiated requests, a job request might also be issued by a parent job to execute a child job in a fork/join manner.
```go

type JobRequest struct {
//gorm.Model
// ID defines UUID for primary key
ID uint64 \`json:"id" gorm:"primary\_key"\`
// ParentID defines id for parent job
ParentID uint64 \`json:"parent\_id"\`
// UserKey defines user-defined UUID and can be used to detect duplicate jobs
UserKey string \`json:"user\_key"\`
// JobDefinitionID points to the job-definition version
JobDefinitionID string \`json:"job\_definition\_id"\`
// JobExecutionID defines foreign key for JobExecution
JobExecutionID string \`json:"job\_execution\_id"\`
// LastJobExecutionID defines foreign key for JobExecution
LastJobExecutionID string \`json:"last\_job\_execution\_id"\`
// OrganizationID defines org who submitted the job
OrganizationID string \`json:"organization\_id"\`
// UserID defines user who submitted the job
UserID string \`json:"user\_id"\`
// Permissions provides who can access this request 0 - all, 1 - Org must match, 2 - UserID must match from authentication
Permissions int \`json:"permissions"\`
// Description of the request
Description string \`json:"description"\`
// Platform overrides platform property for targeting job to a specific follower
Platform string \`json:"platform"\`
// JobType defines type for the job
JobType    string \`json:"job\_type"\`
JobVersion string \`json:"job\_version"\`
// JobState defines state of job that is maintained throughout the lifecycle of a job
JobState types.RequestState \`json:"job\_state"\`
// JobGroup defines a property for grouping related job
JobGroup string \`json:"job\_group"\`
// JobPriority defines priority of the job
JobPriority int \`json:"job\_priority"\`
// Timeout defines max time a job should take, otherwise the job is aborted
Timeout time.Duration \`yaml:"timeout,omitempty" json:"timeout"\`
// ScheduleAttempts defines attempts of schedule
ScheduleAttempts int \`json:"schedule\_attempts" gorm:"schedule\_attempts"\`
// Retried keeps track of retry attempts
Retried int \`json:"retried"\`
// CronTriggered is true if request was triggered by cron
CronTriggered bool \`json:"cron\_triggered"\`
// QuickSearch provides quick search to search a request by params
QuickSearch string \`json:"quick\_search"\`
// ErrorCode captures error code at the end of job execution if it fails
ErrorCode string \`json:"error\_code"\`
// ErrorMessage captures error message at the end of job execution if it fails
ErrorMessage string \`json:"error\_message"\`
// Params are passed with job request
Params \[\]\*JobRequestParam \`yaml:"-" json:"-" gorm:"ForeignKey:JobRequestID" gorm:"auto\_preload" gorm:"constraint:OnUpdate:CASCADE"\`
// Execution refers to job-Execution
Execution       \*JobExecution          \`yaml:"-" json:"execution" gorm:"-"\`
Errors          map\[string\]string      \`yaml:"-" json:"-" gorm:"-"\`  
// ScheduledAt defines schedule time when job will be submitted so that you can submit a job
// that will be executed later
ScheduledAt time.Time \`json:"scheduled\_at"\`
// CreatedAt job creation time
CreatedAt time.Time \`json:"created\_at"\`
// UpdatedAt job update time
UpdatedAt time.Time \`json:"updated\_at" gorm:"updated\_at"\`
}
```

#### 3.3.6 TaskRequest

TaskRequest specifies the parameters for a task that is dispatched to a remote ant-worker for execution. This request is transmitted through a messaging middleware to the most appropriate ant-worker, selected based on its resource availability and capacity to handle the task efficiently.
```go

type TaskRequest struct {
UserID          string                   \`json:"user\_id" yaml:"user\_id"\`
OrganizationID  string                   \`json:"organization\_id" yaml:"organization\_id"\`
JobDefinitionID string                   \`json:"job\_definition\_id" yaml:"job\_definition\_id"\`
JobRequestID    uint64                   \`json:"job\_request\_id" yaml:"job\_request\_id"\`
JobType         string                   \`json:"job\_type" yaml:"job\_type"\`
JobTypeVersion  string                   \`json:"job\_type\_version" yaml:"job\_type\_version"\`
JobExecutionID  string                   \`json:"job\_execution\_id" yaml:"job\_execution\_id"\`
TaskExecutionID string                   \`json:"task\_execution\_id" yaml:"task\_execution\_id"\`
TaskType        string                   \`json:"task\_type" yaml:"task\_type"\`
CoRelationID    string                   \`json:"co\_relation\_id"\`
Platform        string                   \`json:"platform" yaml:"platform"\`
Action          TaskAction               \`json:"action" yaml:"action"\`
JobRetry        int                      \`json:"job\_retry" yaml:"job\_retry"\`
TaskRetry       int                      \`json:"task\_retry" yaml:"task\_retry"\`
AllowFailure    bool                     \`json:"allow\_failure" yaml:"allow\_failure"\`
Tags            \[\]string                 \`json:"tags" yaml:"tags"\`
BeforeScript    \[\]string                 \`json:"before\_script" yaml:"before\_script"\`
AfterScript     \[\]string                 \`json:"after\_script" yaml:"after\_script"\`
Script          \[\]string                 \`json:"script" yaml:"script"\`
Timeout         time.Duration            \`json:"timeout" yaml:"timeout"\`
Variables       map\[string\]VariableValue \`json:"variables" yaml:"variables"\`
ExecutorOpts    \*ExecutorOptions         \`json:"executor\_opts" yaml:"executor\_opts"\`
}
```

#### 3.3.7 ExecutorOptions

ExecutorOptions specify the settings for the underlying executor, including Docker, Kubernetes, Shell, HTTP, etc., ensuring tasks are carried out using the suitable computational resources.
```go
type ExecutorOptions struct {
Name                       string                  \`json:"name" yaml:"name"\`
Method                     TaskMethod              \`json:"method" yaml:"method"\`
Environment                EnvironmentMap          \`json:"environment,omitempty" yaml:"environment,omitempty"\`
HelperEnvironment          EnvironmentMap          \`json:"helper\_environment,omitempty" yaml:"helper\_environment,omitempty"\`
WorkingDirectory           string                  \`json:"working\_dir,omitempty" yaml:"working\_dir,omitempty"\`
ArtifactsDirectory         string                  \`json:"artifacts\_dir,omitempty" yaml:"artifacts\_dir,omitempty"\`
Artifacts                  ArtifactsConfig         \`json:"artifacts,omitempty" yaml:"artifacts,omitempty"\`
CacheDirectory             string                  \`json:"cache\_dir,omitempty" yaml:"cache\_dir,omitempty"\`
Cache                      CacheConfig             \`json:"cache,omitempty" yaml:"cache,omitempty"\`
DependentArtifactIDs       \[\]string                \`json:"dependent\_artifact\_ids,omitempty" yaml:"dependent\_artifact\_ids,omitempty"\`
MainContainer              \*ContainerDefinition    \`json:"container,omitempty" yaml:"container,omitempty"\`
HelperContainer            \*ContainerDefinition    \`json:"helper,omitempty" yaml:"helper,omitempty"\`
Services                   \[\]Service               \`json:"services,omitempty" yaml:"services,omitempty"\`
Privileged                 bool                    \`json:"privileged,omitempty" yaml:"privileged,omitempty"\`
Affinity                   \*KubernetesNodeAffinity \`json:"affinity,omitempty" yaml:"affinity,omitempty"\`
NodeSelector               map\[string\]string       \`json:"node\_selector,omitempty" yaml:"node\_selector,omitempty"\`
NodeTolerations            NodeTolerations         \`json:"node\_tolerations,omitempty" yaml:"node\_tolerations,omitempty"\`
PodLabels                  map\[string\]string       \`json:"pod\_labels,omitempty" yaml:"pod\_labels,omitempty"\`
PodAnnotations             map\[string\]string       \`json:"pod\_annotations,omitempty" yaml:"pod\_annotations,omitempty"\`
NetworkMode                string                  \`json:"network\_mode,omitempty" yaml:"network\_mode,omitempty"\`
HostNetwork                bool                    \`json:"host\_network,omitempty" yaml:"host\_network,omitempty"\`
Headers                    map\[string\]string       \`yaml:"headers,omitempty" json:"headers"\`
QueryParams                map\[string\]string       \`yaml:"query,omitempty" json:"query"\`
MessagingRequestQueue      string                  \`json:"messaging\_request\_queue,omitempty" yaml:"messaging\_request\_queue,omitempty"\`
MessagingReplyQueue        string                  \`json:"messaging\_reply\_queue,omitempty" yaml:"messaging\_reply\_queue,omitempty"\`
ForkJobType                string                  \`json:"fork\_job\_type,omitempty" yaml:"fork\_job\_type,omitempty"\`
ForkJobVersion             string                  \`json:"fork\_job\_version,omitempty" yaml:"fork\_job\_version,omitempty"\`
ArtifactKeyPrefix          string                  \`json:"artifact\_key\_prefix,omitempty" yaml:"artifact\_key\_prefix,omitempty"\`
AwaitForkedTasks           \[\]string                \`json:"await\_forked\_tasks,omitempty" yaml:"await\_forked\_tasks,omitempty"\`
CostFactor                 float64                 \`json:"cost\_factor,omitempty" yaml:"cost\_factor,omitempty"\`
}
```

#### 3.3.8 TaskResponse

TaskResponse outlines the outcome of a task execution, encompassing its status, context, generated artifacts, and additional outputs.
```go
type TaskResponse struct {
JobRequestID    uint64                 \`json:"job\_request\_id"\`
TaskExecutionID string                 \`json:"task\_execution\_id"\`
JobType         string                 \`json:"job\_type"\`
JobTypeVersion  string                 \`json:"job\_type\_version"\`
TaskType        string                 \`json:"task\_type"\`
CoRelationID    string                 \`json:"co\_relation\_id"\`
Status          RequestState           \`json:"status"\`
AntID           string                 \`json:"ant\_id"\`
Host            string                 \`json:"host"\`
Namespace       string                 \`json:"namespace"\`
Tags            \[\]string               \`json:"tags"\`
ErrorMessage    string                 \`json:"error\_message"\`
ErrorCode       string                 \`json:"error\_code"\`
ExitCode        string                 \`json:"exit\_code"\`
ExitMessage     string                 \`json:"exit\_message"\`
FailedCommand   string                 \`json:"failed\_command"\`
TaskContext     map\[string\]interface{} \`json:"task\_context"\`
JobContext      map\[string\]interface{} \`json:"job\_context"\`
Artifacts       \[\]\*Artifact            \`json:"artifacts"\`
Warnings        \[\]string               \`json:"warnings"\`
Stdout          \[\]string               \`json:"stdout"\`
CostFactor      float64                \`json:"cost\_factor"\`
Timings         TaskResponseTimings    \`json:"timings"\`
}
```

### 3.4 Events Model

Here’s a summary of the principal events model within the [Formicary](https://github.com/bhatti/formicary) system, which facilitates communication among the main components:

![Formicary Events](https://weblog.plexobject.com/images/formicary_events.png)



In above diagram, the lifecycle events are published upon start and completion of a job-request, job-execution, task-execution, and containers. Other events are propagated upon health errors, logging and leader election for the job scheduler.

### 3.5 Physical Architecture

Following diagram depicts the physical architecture of the [Formicary](https://github.com/bhatti/formicary) system:

![physical architecture](https://weblog.plexobject.com/images/formicary_physical.png)

The physical architecture of a [Formicary](https://github.com/bhatti/formicary) system is structured as follows:

 -  **Queen Server:** It manages task scheduling, resource allocation, and system monitoring. The job requests, definitions, user data, and configuration settings are maintained in the database.
 -  **Ant Workers:** These are distributed computing resources that execute the tasks assigned by the central server. Each ant worker is equipped with the necessary software to perform various tasks, such as processing data, running applications, or handling web requests. Worker nodes report their status, capacity, and workload back to the central server to facilitate efficient task distribution.
 -  **Storage Systems:** Relational databases are used to store structured data such as job definitions, user accounts, and system configurations. Object storage systems hold unstructured data, including task artifacts, logs, and binary data.
 -  **Messaging Middleware:** Messaging queues and APIs facilitate asynchronous communication and integration with other systems.
 -  **Execution Environments:** Consist of container orchestration systems like Kubernetes and Docker for isolating and managing task executions. They provide scalable and flexible environments that support various execution methods, including shell scripts, HTTP requests, and custom executables.
 -  **Monitoring and Alerting Tools:** [Formicary](https://github.com/bhatti/formicary) system integrates with Prometheus for monitoring solutions to track the health, performance, and resource usage of both the central server and worker nodes. Alerting mechanisms notify administrators and users about system events, performance bottlenecks, and potential issues.
 -  **Security Infrastructure:** Authentication and authorization mechanisms control access to resources and tasks based on user roles and permissions.

This architecture allows the [Formicary](https://github.com/bhatti/formicary) system to scale horizontally by adding more worker nodes as needed to handle increased workloads, and vertically by enhancing the capabilities of the central server and worker nodes. The system’s design emphasizes reliability, scalability, and efficiency, making it suitable for a wide range of applications, from data processing and analysis to web hosting and content delivery.