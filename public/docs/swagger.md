# Formicary API
The formicary is a distributed orchestration engine based on `Leader-Follower`, `Pipes-Filter`, `Fork-Join` and `SEDA` design principles for
executing a directed acyclic graph of tasks, which is also referred as a job workflow. A task represents a unit of work and a job definition is used to specify the task
dependencies in the graph/workflow including configuration parameters and conditional logic.

## Version: 0.0.1

**Contact information:**  
Support  
<https://formicary.io>  
support@formicary.io  

**License:** [AGPL](https://opensource.org/licenses/AGPL-3.0)

### Security
**api_key**  

|apiKey|*API Key*|
|---|---|
|In|header|
|Name|Authorization|

### /api/ants

#### GET
##### Summary

Queries ant registration.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of ant-registrations matching query | object |

### /api/ants/{id}

#### GET
##### Summary

Retrieves ant-registration by its id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Ant Registration body | [AntRegistration](#antregistration) |

### /api/artifacts

#### GET
##### Summary

Queries artifacts by name, task-type, etc.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| order | query |  | No | string |
| name | query | Name - name of artifact for display | No | string |
| group | query | Group of artifact | No | string |
| kind | query | Kind of artifact | No | string |
| job_request_id | query | JobRequestID refers to request-id being processed | No | integer (uint64) |
| task_type | query | TaskType defines type of task | No | string |
| sha256 | query | SHA256 refers hash of the contents | No | string |
| content_type | query | ContentType refers to content-type of artifact | No | string |
| content_length | query | ContentLength refers to content-length of artifact | No | long |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of artifacts matching query | object |

#### POST
##### Summary

Uploads artifact data from the request body and returns metadata for the uploaded data.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [ integer (uint8) ] |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Artifact body | [Artifact](#artifact) |

### /api/artifacts/{id}

#### DELETE
##### Description

Deletes artifact by its id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Description

Retrieves artifact by its id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Artifact body | [Artifact](#artifact) |

### /api/artifacts/{id}/download

#### GET
##### Description

Download artifact by its id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Byte Array response body | [ integer (uint8) ] |

### /api/artifacts/{id}/logs

#### GET
##### Description

Download artifact by its id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Byte Array response body | [ integer (uint8) ] |

### /api/audits

#### GET
##### Summary

Queries audits within the organization that is allowed.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| target_id | query | TargetID defines target id | No | string |
| user_id | query | UserID defines user who submitted the job | No | string |
| organization_id | query | OrganizationID defines org who submitted the job | No | string |
| kind | query | Kind defines type of audit record | No | string |
| job_type | query | JobType - job-type | No | string |
| q | query | Q wild search | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of audits matching query | object |

### /api/configs

#### GET
##### Description

Queries system configs
`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| scope | query | Scope defines scope such as default or org-unit | No | string |
| kind | query | Kind defines kind of config property | No | string |
| name | query | Name defines name of config property | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Query results of system-configs | object |

#### POST
##### Summary

Creates new system config based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [SystemConfig](#systemconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | SystemConfig body for update | [SystemConfig](#systemconfig) |

### /api/configs/{id}

#### DELETE
##### Summary

Deletes an existing system config based on id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Deletes an existing system config based on id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### PUT
##### Summary

Updates an existing system config based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| Body | body |  | No | [SystemConfig](#systemconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | SystemConfig body for update | [SystemConfig](#systemconfig) |

### /api/errors

#### GET
##### Summary

Queries error-codes by type, regex.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| regex | query |  | No | string |
| exit_code | query | ExitCode defines exit-code for error | No | long |
| error_code | query | ErrorCode defines error code | No | string |
| job_type | query | JobType defines type for the job | No | string |
| task_type_scope | query | TaskTypeScope only applies error code for task_type | No | string |
| platform_scope | query | PlatformScope only applies error code for platform | No | string |
| hard_failure | query | HardFailure determines if this error can be retried or is hard failure | No | boolean |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Query results of error-codes | object |

#### POST
##### Summary

Creates new error code based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [ErrorCode](#errorcode) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | ErrorCode body for update | [ErrorCode](#errorcode) |

#### PUT
##### Summary

Updates new error code based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [ErrorCode](#errorcode) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | ErrorCode body for update | [ErrorCode](#errorcode) |

### /api/errors/{id}

#### DELETE
##### Summary

Deletes error code by id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds error code by id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | ErrorCode body for update | [ErrorCode](#errorcode) |

### /api/executors

#### GET
##### Summary

Queries container executions.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of container-executions matching query | object |

### /api/executors/{id}

#### GET
##### Summary

Deletes container-executor by its id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/health

#### GET
##### Summary

Returns health status.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 |  | [HealthQueryResponse](#healthqueryresponse) |

### /api/jobs/definitions

#### GET
##### Summary

Queries job definitions by criteria such as type, platform, etc.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| job_type | query | JobType defines a unique type of job | No | string |
| platform | query | Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering | No | string |
| disabled | query | disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging. | No | boolean |
| public_plugin | query | PublicPlugin means job is public plugin | No | boolean |
| tags | query | Tags is aggregation of task tags, and it can be searched via `tags:in` | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of jobDefinitions matching query | object |

#### POST
##### Summary

Uploads job definitions using JSON or YAML body based on content-type header.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [JobDefinition](#jobdefinition) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-definition defines DAG (directed acyclic graph) of tasks, which are executed by ant followers. The workflow of job uses task exit codes to define next task to execute. | [JobDefinition](#jobdefinition) |

### /api/jobs/definitions/{id}

#### DELETE
##### Summary

Deletes the job-definition by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds the job-definition by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-definition defines DAG (directed acyclic graph) of tasks, which are executed by ant followers. The workflow of job uses task exit codes to define next task to execute. | [JobDefinition](#jobdefinition) |

### /api/jobs/definitions/{id}/concurrency

#### PUT
##### Summary

Updates the concurrency for job-definition by id to limit the maximum jobs that can be executed at the same time.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| concurrency | formData |  | No | long |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/definitions/{id}/disable

#### POST
##### Summary

disables job-definition so that no new requests are executed while in-progress jobs are allowed to complete.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/definitions/{id}/dot

#### GET
##### Summary

Returns Graphviz DOT definition for the graph of tasks defined in the job.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | String response body |

### /api/jobs/definitions/{id}/dot.png

#### GET
##### Summary

Returns Real-time statistics of jobs running.

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 |  | [ [JobStats](#jobstats) ] |

### /api/jobs/definitions/{id}/enable

#### POST
##### Summary

Enables job-definition so that new requests can start processing.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/definitions/{jobId}/configs

#### GET
##### Summary

Queries job configs by criteria such as name, type, etc.

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of jobConfigs matching query | object |

#### POST
##### Summary

Adds a config for the job.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [JobDefinitionConfig](#jobdefinitionconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [JobDefinitionConfig](#jobdefinitionconfig) |

### /api/jobs/definitions/{jobId}/configs/{id}

#### DELETE
##### Summary

Deletes a config for the job by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| jobId | path |  | Yes | string |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds a config for the job by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| jobId | path |  | Yes | string |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [JobDefinitionConfig](#jobdefinitionconfig) |

#### PUT
##### Summary

Updates a config for the job.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| jobId | path |  | Yes | string |
| id | path |  | Yes | string |
| Body | body |  | No | [JobDefinitionConfig](#jobdefinitionconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [JobDefinitionConfig](#jobdefinitionconfig) |

### /api/jobs/definitions/{type}/yaml

#### GET
##### Summary

Finds job-definition by type and returns response YAML format.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| type | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-definition defines DAG (directed acyclic graph) of tasks, which are executed by ant followers. The workflow of job uses task exit codes to define next task to execute. | [JobDefinition](#jobdefinition) |

### /api/jobs/plugins

#### GET
##### Summary

Queries job definitions by criteria such as type, platform, etc.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| job_type | query | JobType defines a unique type of job | No | string |
| platform | query | Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering | No | string |
| disabled | query | disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging. | No | boolean |
| public_plugin | query | PublicPlugin means job is public plugin | No | boolean |
| tags | query | Tags is aggregation of task tags, and it can be searched via `tags:in` | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of jobDefinitions matching query | object |

### /api/jobs/requests

#### GET
##### Summary

Queries job requests by criteria such as type, platform, etc.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| job_type | query | JobType defines a unique type of job | No | string |
| platform | query | Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering | No | string |
| job_state | query | JobState defines state of job that is maintained throughout the lifecycle of a job | No | string |
| job_group | query | JobGroup defines a property for grouping related job | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of jobRequests matching query | object |

#### POST
##### Summary

Submits a job-request for processing, which is saved in the database and is then scheduled for execution.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [JobRequest](#jobrequest) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | JobRequest defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [JobRequest](#jobrequest) |

### /api/jobs/requests/{id}

#### GET
##### Summary

Finds the job-request by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | JobRequest defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [JobRequest](#jobrequest) |

### /api/jobs/requests/{id}/cancel

#### POST
##### Summary

Cancels a job-request that is pending for execution or already executing.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/requests/{id}/dot

#### GET
##### Summary

Returns Graphviz DOT request for the graph of tasks defined in the job request.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | String response body |

### /api/jobs/requests/{id}/dot.png

#### GET
##### Summary

Returns Graphviz DOT image for the graph of tasks defined in the job.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Byte Array response body | [ integer (uint8) ] |

### /api/jobs/requests/{id}/restart

#### POST
##### Description

Restarts a previously failed job so that it can re-executed, the restart may perform soft-restart where only
failed tasks are executed or hard-restart where all tasks are executed.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/requests/{id}/trigger

#### POST
##### Description

Triggers a scheduled job

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/requests/{id}/wait_time

#### GET
##### Summary

Returns wait time for the job-request.

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-request wait times based on average of previously executed jobs and pending jobs in the queue. | [JobWaitEstimate](#jobwaitestimate) |

### /api/jobs/requests/dead_ids

#### GET
##### Summary

Returns job-request ids for recently completed jobs.

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-request ids | [ integer (uint64) ] |

### /api/jobs/requests/stats

#### GET
##### Summary

Returns statistics for the job-request such as success rate, latency, etc.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | The job-request statistics about success-rate, latency, etc. | [ [JobCounts](#jobcounts) ] |

### /api/jobs/resources

#### GET
##### Summary

Queries job resources by criteria such as type, platform, etc.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| resource_type | query | ResourceType defines type of resource such as Device, CPU, Memory | No | string |
| description | query | Description of resource | No | string |
| platform | query | Platform can be OS platform or target runtime | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of jobResources matching query | object |

#### POST
##### Summary

Adds a job-resource that can be used for managing internal or external constraints.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [JobResource](#jobresource) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | JobResource represents a virtual resource, which can be used to implement mutex/semaphores for a job. | [JobResource](#jobresource) |

### /api/jobs/resources/{id}

#### GET
##### Summary

Finds the job-resource by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | JobResource represents a virtual resource, which can be used to implement mutex/semaphores for a job. | [JobResource](#jobresource) |

#### PUT
##### Summary

Updates a job-resource that can be used for managing internal or external constraints.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| Body | body |  | No | [JobResource](#jobresource) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | JobResource represents a virtual resource, which can be used to implement mutex/semaphores for a job. | [JobResource](#jobresource) |

### /api/jobs/resources/{id}/configs

#### POST
##### Description

Save the job-resource config

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | jobResourceConfig represents config for the resource | [JobResourceConfig](#jobresourceconfig) |

### /api/jobs/resources/{id}/configs/{configId}

#### DELETE
##### Description

Deletes the job-resource config by id of job-resource and config-id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| config_id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/jobs/resources/{id}/disable

#### POST
##### Description

Deletes the job-resource by id

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/logs

#### POST
##### Description

Post log event

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [ integer (uint8) ] |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | logResponse defines response of log event |

### /api/metrics

#### GET
##### Summary

Returns prometheus health metrics.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Results of prometheus-metrics | [ string ] |

### /api/orgs

#### GET
##### Summary

Queries organizations by criteria such as org-unit, bundle, etc.

##### Description

`This requires admin access`

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of orgs matching query | object |

#### POST
##### Summary

Creates new organization.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [Organization](#organization) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Org defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [Organization](#organization) |

### /api/orgs/{id}

#### DELETE
##### Summary

Deletes the organization by its id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds the organization by its id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Org defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [Organization](#organization) |

#### PUT
##### Summary

Updates the organization profile.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| Body | body |  | No | [Organization](#organization) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Org defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [Organization](#organization) |

### /api/orgs/{id}/invite

#### POST
##### Description

Invite user to the organization

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User invitation | [UserInvitation](#userinvitation) |

### /api/orgs/{orgId}/configs

#### GET
##### Summary

Queries organization configs by criteria such as name, type, etc.

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of orgConfigs matching query | object |

#### POST
##### Summary

Adds a config for the organization.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [OrganizationConfig](#organizationconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [OrganizationConfig](#organizationconfig) |

### /api/orgs/{orgId}/configs/{id}

#### DELETE
##### Summary

Deletes a config for the organization by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| orgId | path |  | Yes | string |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds a config for the organization by id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| orgId | path |  | Yes | string |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [OrganizationConfig](#organizationconfig) |

#### PUT
##### Summary

Updates a config for the organization.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| orgId | path |  | Yes | string |
| id | path |  | Yes | string |
| Body | body |  | No | [OrganizationConfig](#organizationconfig) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OrgConfig defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution. | [OrganizationConfig](#organizationconfig) |

### /api/orgs/usage_report

#### POST
##### Description

`This requires admin access`
Shows usage report by organization and user

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| from | query |  | No | string |
| to | query | TO ISO date | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Usage Report | [ [CombinedResourceUsage](#combinedresourceusage) ] |

### /api/subscriptions

#### GET
##### Description

Queries system subscriptions
`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| scope | query | Scope defines scope such as default or org-unit | No | string |
| kind | query | Kind defines kind of subscription property | No | string |
| name | query | Name defines name of subscription property | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Query results of subscriptions | object |

#### POST
##### Summary

Creates new system subscription based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [Subscription](#subscription) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Subscription body for update | [Subscription](#subscription) |

### /api/subscriptions/{id}

#### DELETE
##### Summary

Deletes an existing system subscription based on id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Deletes an existing system subscription based on id.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### PUT
##### Summary

Updates an existing system subscription based on request body.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| Body | body |  | No | [Subscription](#subscription) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Subscription body for update | [Subscription](#subscription) |

### /api/users

#### GET
##### Summary

Queries users within the organization that is allowed.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| name | query | Name of user | No | string |
| username | query | Username defines username | No | string |
| email | query | Email defines email | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of users matching query | object |

#### POST
##### Summary

Creates new user.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| Body | body |  | No | [User](#user) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User of the system who can create job-definitions, submit and execute jobs. | [User](#user) |

### /api/users/:id/verify_email

#### POST
##### Summary

Creates new emailVerification.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| code | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | EmailVerification is used for email verification | [EmailVerification](#emailverification) |

### /api/users/:id/verify_email/:code

#### PUT
##### Summary

Creates new emailVerification.

##### Description

`This requires admin access`

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| code | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | EmailVerification is used for email verification | [EmailVerification](#emailverification) |

### /api/users/{id}

#### DELETE
##### Summary

Deletes the user profile by its id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

#### GET
##### Summary

Finds user profile by its id.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User of the system who can create job-definitions, submit and execute jobs. | [User](#user) |

#### PUT
##### Summary

Updates user profile.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| Body | body |  | No | [User](#user) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User of the system who can create job-definitions, submit and execute jobs. | [User](#user) |

### /api/users/{id}/notify

#### PUT
##### Summary

Updates user notification.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| id | path |  | Yes | string |
| email | formData |  | No | string |
| slackChannel | query |  | No | string |
| when | query |  | No | string |
| Body | body |  | No | [User](#user) |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User of the system who can create job-definitions, submit and execute jobs. | [User](#user) |

### /api/users/{userId}/tokens

#### GET
##### Summary

Queries user-tokens for the API access.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| userId | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Results of user-tokens | [ [UserToken](#usertoken) ] |

#### POST
##### Summary

Creates new user-token for the API access.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| userId | path |  | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | User-token for the API access. | [UserToken](#usertoken) |

### /api/users/{userId}/tokens/{id}

#### DELETE
##### Summary

Deletes user-token by its id so that it cannot be used for the API access.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| userId | path |  | Yes | string |
| id | path |  | Yes | string |

##### Responses

| Code | Description |
| ---- | ----------- |
| 200 | Empty response body |

### /api/users/email_verifications

#### GET
##### Summary

Queries email-verifications within the organization that is allowed.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| page | query |  | No | long |
| page_size | query |  | No | long |
| name | query | Name of emailVerification | No | string |
| email_code | query | EmailCode defines email code | No | string |
| email | query | Email defines email | No | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | Paginated results of email-verifications matching query | object |

### Models

#### AntRegistration

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| allocations |  |  | No |
| ant_id | string |  | No |
| ant_started_at | dateTime |  | No |
| ant_topic | string |  | No |
| created_at | dateTime |  | No |
| current_load | long |  | No |
| encryption_key | string |  | No |
| max_capacity | long |  | No |
| methods | [ [TaskMethod](#taskmethod) ] |  | No |
| persistent_connection | boolean |  | No |
| tags | [ string ] |  | No |

#### Artifact

The metadata defines properties to associate artifact with a task or job and can be used to query artifacts
related for a job or an organization.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| artifact_order | long | ArtifactOrder of artifact in group | No |
| bucket | string | Bucket defines bucket where artifact is stored | No |
| content_length | long | ContentLength refers to content-length of artifact | No |
| content_type | string | ContentType refers to content-type of artifact | No |
| created_at | dateTime | CreatedAt job creation time | No |
| etag | string | ETag stores ETag from underlying storage such as Minio/S3 | No |
| expires_at | dateTime | ExpiresAt - expiration time | No |
| group | string | Group of artifact | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_execution_id | string | JobExecutionID refers to job-execution-id being processed | No |
| job_request_id | integer (uint64) | JobRequestID refers to request-id being processed | No |
| kind | string | Kind of artifact | No |
| metadata | object | MetadataMap - transient map of properties - deserialized from MetadataSerialized | No |
| name | string | Name defines name of artifact for display | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| permissions | integer | Permissions of artifact | No |
| sha256 | string | SHA256 defines hash of the contents using SHA-256 algorithm | No |
| tags | object |  | No |
| task_execution_id | string | TaskExecutionID refers to task-execution-id being processed | No |
| task_type | string | TaskType defines type of task | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| url | string |  | No |
| user_id | string | UserID defines user who submitted the job | No |

#### AuditKind

AuditKind defines enum for state of request or execution

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| AuditKind | string | AuditKind defines enum for state of request or execution |  |

#### AuditRecord

AuditRecord defines audit-record

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| error | string | Error message | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_type | string | JobType - job-type | No |
| kind | [AuditKind](#auditkind) |  | No |
| message | string | Message defines audit message | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| remote_ip | string | RemoteIP defines remote ip-address | No |
| target_id | string | TargetID defines target id | No |
| url | string | URL defines access url | No |
| user_id | string | UserID defines user who submitted the job | No |

#### BasicResource

These mutex/semaphores can represent external resources that job requires and can be used to determine
concurrency of jobs. For example, a job may need a license key to connect to a third party service, and
it may only accept upto five connections that can be allocated via resources.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| category | string | Category can be used to represent grouping of resources | No |
| description | string | Description of resource | No |
| extract_config | [ResourceCriteriaConfig](#resourcecriteriaconfig) |  | No |
| platform | string | Platform can be OS platform or target runtime | No |
| resource_type | string | ResourceType defines type of resource such as Device, CPU, Memory | No |
| tags | [ string ] | Tags can be used as tags for resource matching | No |
| value | long | Value consumed, e.g. it will be 1 for mutex, semaphore but can be higher number for other quota system | No |

#### CombinedResourceUsage

CombinedResourceUsage defines use of a combined resource

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| cpu_resource | [ResourceUsage](#resourceusage) |  | No |
| disk_resource | [ResourceUsage](#resourceusage) |  | No |
| organization_id | string |  | No |
| user_id | string |  | No |

#### ContainerLifecycleEvent

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ant_id | string |  | No |
| container_id | string | ContainerID | No |
| container_name | string | ContainerName | No |
| container_state | [RequestState](#requeststate) |  | No |
| created_at | dateTime |  | No |
| ended_at | dateTime | EndedAt | No |
| event_type | string |  | No |
| id | string |  | No |
| labels | object | Labels | No |
| method | [TaskMethod](#taskmethod) |  | No |
| source | string |  | No |
| started_at | dateTime | StartedAt | No |
| user_id | string |  | No |

#### Duration

A Duration represents the elapsed time between two instants
as an int64 nanosecond count. The representation limits the
largest representable duration to approximately 290 years.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Duration | integer | A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years. |  |

#### EmailVerification

EmailVerification represents verified email

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt created time | No |
| email | string | Email defines invitee | No |
| email_code | string | EmailCode defines code | No |
| expires_at | dateTime | ExpiresAt expiration time | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| user_id | string | UserID defines foreign key | No |
| verified_at | dateTime | VerifiedAt verification time | No |

#### ErrorCode

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| action | [ErrorCodeAction](#errorcodeaction) |  | No |
| command_scope | string | CommandScope only applies error code for command | No |
| created_at | dateTime | CreatedAt job creation time | No |
| description | string | Description of error | No |
| display_code | string | DisplayCode defines user code for error | No |
| display_message | string | DisplayMessage defines user message for error | No |
| error_code | string | ErrorCode defines error code | No |
| exit_code | long | ExitCode defines exit-code for error | No |
| hard_failure | boolean | HardFailure determines if this error can be retried or is hard failure | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_type | string | JobType defines type for the job | No |
| organization_id | string | OrganizationID defines org who owns the error code | No |
| platform_scope | string | PlatformScope only applies error code for platform | No |
| regex | string | Regex matches error-code | No |
| retry | long | Retry defines number of tries if task is failed with this error code | No |
| task_type_scope | string | TaskTypeScope only applies error code for task_type | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| user_id | string | UserID defines user who owns the error code | No |

#### ErrorCodeAction

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ErrorCodeAction | string |  |  |

#### HealthQueryResponse

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| dependent_service_statuses | [ [ServiceStatus](#servicestatus) ] |  | No |
| overall_status | [ServiceStatus](#servicestatus) |  | No |

#### JobCounts

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| counts | long | Counts defines total number of records matching stats | No |
| end_time | dateTime | EndTime stores last occurrence of the stats | No |
| end_time_string | string | EndTime stores last occurrence of the stats for sqlite | No |
| error_code | string | ErrorCode defines error code if job failed | No |
| job_state | [RequestState](#requeststate) |  | No |
| job_type | string | JobType defines type for the job | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| start_time | dateTime | StartTime stores first occurrence of the stats | No |
| start_time_stirng | string | StartTime stores first occurrence of the stats for sqlite | No |
| user_id | string | UserID defines user who submitted the job | No |

#### JobDefinition

The workflow of job uses task exit codes to define next task to execute. The task definition
represents definition of a job and instance of the job is created using JobExecution when a new job request is
submitted.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| cron_trigger | string | CronTrigger can be used to run the job periodically | No |
| delay_between_retries | [Duration](#duration) |  | No |
| description | string | Description of job | No |
| disabled | boolean | disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging. | No |
| hard_reset_after_retries | long | HardResetAfterRetries defines retry config when job is rerun and as opposed to re-running only failed tasks, all tasks are executed. | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_type | string | JobType defines a unique type of job | No |
| job_variables | object |  | No |
| max_concurrency | long | MaxConcurrency defines max number of jobs that can be run concurrently | No |
| methods | string | Methods is aggregation of task methods | No |
| notify | object |  | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| platform | string | Platform can be OS platform or target runtime and a job can be targeted for specific platform that can be used for filtering | No |
| public_plugin | boolean | PublicPlugin means job is public plugin | No |
| required_params | [ string ] | RequiredParams from job request (and plugin) | No |
| resources | [BasicResource](#basicresource) |  | No |
| retry | long | Retry defines max number of tries a job can be retried where it re-runs failed job | No |
| sem_version | string | SemVersion - semantic version is used for external version, which can be used for public plugins. | No |
| tags | string | Tags are used to use specific followers that support the tags defined by ants. Tags is aggregation of task tags | No |
| tasks | [ [TaskDefinition](#taskdefinition) ] | Tasks defines one to many relationships between job and tasks, where a job defines a directed acyclic graph of tasks that are executed for the job. | No |
| timeout | [Duration](#duration) |  | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| url | string | URL defines url for job | No |
| user_id | string | UserID defines user who updated the job | No |

#### JobDefinitionConfig

JobDefinitionConfig defines variables for job definition

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| id | string | ID defines UUID for primary key | No |
| job_definition_id | string | JobDefinitionID defines foreign key for JobDefinition | No |
| name | string | Name defines name of property | No |
| secret | boolean | Secret for encryption | No |
| type | string | Type defines type of property value | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| value | string | Value defines value of property that can be string, number, boolean or JSON structure | No |

#### JobExecution

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| contexts | [ [JobExecutionContext](#jobexecutioncontext) ] | Contexts defines context variables of job | No |
| cpu_secs | long | CPUSecs execution time | No |
| ended_at | dateTime | EndedAt job execution end time | No |
| error_code | string | ErrorCode captures error code at the end of job execution if it fails | No |
| error_message | string | ErrorMessage captures error message at the end of job execution if it fails | No |
| exit_code | string | ExitCode defines exit status from the job execution | No |
| exit_message | string | ExitMessage defines exit message from the job execution | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_request_id | integer (uint64) | JobRequestID defines foreign key for job request | No |
| job_state | [RequestState](#requeststate) |  | No |
| job_type | string | JobType defines type for the job | No |
| job_version | string |  | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| started_at | dateTime | StartedAt job execution start time | No |
| tasks | [ [TaskExecution](#taskexecution) ] | Tasks defines list of tasks that are executed for the job | No |
| updated_at | dateTime | UpdatedAt job execution last update time | No |
| user_id | string | UserID defines user who submitted the job | No |

#### JobExecutionContext

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job context creation time | No |
| id | string | ID defines UUID for primary key | No |
| job_execution_id | string | gorm.Model JobExecutionID defines foreign key for JobExecution | No |
| name | string | Name defines name of property | No |
| secret | boolean | Secret for encryption | No |
| type | string | Type defines type of property value | No |
| value | string | Value defines value of property that can be string, number, boolean or JSON structure | No |

#### JobNotifyConfig

JobNotifyConfig structure for notification config

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| recipients | [ string ] |  | No |
| when | [NotifyWhen](#notifywhen) |  | No |

#### JobRequest

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| cron_triggered | boolean | CronTriggered is true if request was triggered by cron | No |
| description | string | Description of the request | No |
| error_code | string | ErrorCode captures error code at the end of job execution if it fails | No |
| error_message | string | ErrorMessage captures error message at the end of job execution if it fails | No |
| execution | [JobExecution](#jobexecution) |  | No |
| id | integer (uint64) | gorm.Model ID defines UUID for primary key | No |
| job_definition_id | string | JobDefinitionID points to the job-definition version | No |
| job_execution_id | string | JobExecutionID defines foreign key for JobExecution | No |
| job_group | string | JobGroup defines a property for grouping related job | No |
| job_priority | long | JobPriority defines priority of the job | No |
| job_state | [RequestState](#requeststate) |  | No |
| job_type | string | JobType defines type for the job | No |
| job_version | string |  | No |
| last_job_execution_id | string | LastJobExecutionID defines foreign key for JobExecution | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| params | object |  | No |
| parent_id | integer (uint64) | ParentID defines id for parent job | No |
| permissions | long | Permissions provides who can access this request 0 - all, 1 - Org must match, 2 - UserID must match from authentication | No |
| platform | string | Platform overrides platform property for targeting job to a specific follower | No |
| quick_search | string | QuickSearch provides quick search to search a request by params | No |
| retried | long | Retried keeps track of retry attempts | No |
| schedule_attempts | long | ScheduleAttempts defines attempts of schedule | No |
| scheduled_at | dateTime | ScheduledAt defines schedule time when job will be submitted so that you can submit a job that will be executed later | No |
| timeout | [Duration](#duration) |  | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| user_id | string | UserID defines user who submitted the job | No |
| user_key | string | UserKey defines user-defined UUID and can be used to detect duplicate jobs | No |

#### JobRequestInfo

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| cron_triggered | boolean | CronTriggered is true if request was triggered by cron | No |
| id | integer (uint64) | ID defines UUID for primary key | No |
| job_definition_id | string | JobDefinitionID points to the job-definition version | No |
| job_execution_id | string | JobExecutionID | No |
| job_priority | long | JobPriority defines priority of the job | No |
| job_state | [RequestState](#requeststate) |  | No |
| job_type | string | JobType defines type for the job | No |
| job_version | string |  | No |
| last_job_execution_id | string | LastJobExecutionID defines foreign key for JobExecution | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| schedule_attempts | long | ScheduleAttempts defines attempts of schedule | No |
| scheduled_at | dateTime | ScheduledAt defines schedule time | No |
| tried | long | Retried job tries | No |
| user_id | string | UserID defines user who submitted the job | No |

#### JobResource

Job Resources can be used for allocating computing resources such as devices, CPUs, memory, connections, licences, etc.
You can use them as mutex, semaphores or quota system to determine concurrency of jobs.
For example, a job may need a license key to connect to a third party service, and it may only accept upto
five connections that can be allocated via resources.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| category | string | Category can be used to represent grouping of resources | No |
| created_at | dateTime | CreatedAt job creation time | No |
| description | string | Description of resource | No |
| disabled | boolean | Disabled is used to stop further processing of job, and it can be used during maintenance, upgrade or debugging. | No |
| external_id | string | ExternalID defines external-id of the resource if exists | No |
| extract_config | [ResourceCriteriaConfig](#resourcecriteriaconfig) |  | No |
| id | string | ID defines UUID for primary key | No |
| lease_timeout | [Duration](#duration) |  | No |
| organization_id | string | OrganizationID defines org who submitted the job | No |
| platform | string | Platform can be OS platform or target runtime | No |
| quota | long | Quota can be used to represent mutex (max 1), semaphores (max limit) or other kind of quota. Note: mutex/semaphores can only take one resource by quota may take any value | No |
| resource_type | string | ResourceType defines type of resource such as Device, CPU, Memory | No |
| resource_variables | object | Following are transient properties | No |
| tags | [ string ] | Tags can be used as tags for resource matching | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| user_id | string | UserID defines user who submitted the job | No |
| valid_status | boolean | ValidStatus - health status of resource | No |
| value | long | Value consumed, e.g. it will be 1 for mutex, semaphore but can be higher number for other quota system | No |

#### JobResourceConfig

JobResourceConfig defines configuration for job resource

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| id | string | ID defines UUID for primary key | No |
| job_resource_id | string | JobResourceID defines foreign key for JobResource | No |
| name | string | Name defines name of property | No |
| secret | boolean | Secret for encryption | No |
| type | string | Type defines type of property value | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| value | string | Value defines value of property that can be string, number, boolean or JSON structure | No |

#### JobStats

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ant_unavailable_error | string | AntUnavailableError error | No |
| ants_available | boolean | AntsAvailable flag | No |
| ants_capacity | long | AntsCapacity | No |
| executing_jobs | integer | ExecutingJobs count | No |
| failed_jobs | long | FailedJobs count | No |
| failed_jobs_average_latency | double | FailedJobsAverage average | No |
| failed_jobs_max_latency | long | FailedJobsMax max | No |
| failed_jobs_min_latency | long | SailedJobsMin min | No |
| first_job_at | dateTime | FirstJobAt time of job start | No |
| job_disabled | boolean | JobDisabled disabled flag | No |
| job_key | [UserJobTypeKey](#userjobtypekey) |  | No |
| last_job_at | dateTime | LastJobAt update time of last job | No |
| succeeded_jobs | long | SucceededJobs count | No |
| succeeded_jobs_average_latency | double | SucceededJobsAverage average | No |
| succeeded_jobs_max_latency | long | SucceededJobsMax max | No |
| succeeded_jobs_min_latency | long | SucceededJobsMin min | No |
| succeeded_jobs_percentage | long | SucceededJobsPercentages | No |

#### JobWaitEstimate

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| JobRequest | [JobRequestInfo](#jobrequestinfo) |  | No |
| JobStats | [JobStats](#jobstats) |  | No |
| error_message | string | ErrorMessage | No |
| estimated_wait | [Duration](#duration) |  | No |
| pending_job_ids | [ integer (uint64) ] | PendingJobIDs | No |
| queue_number | long | QueueNumber number in queue | No |
| scheduled_at | dateTime | ScheduledAt - schedule time | No |

#### Kind

Kind defines enum for kind of subscription

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Kind | string | Kind defines enum for kind of subscription |  |

#### Monitorable

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Name | string |  | No |

#### NotifyWhen

NotifyWhen type alias for when notify should be used

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| NotifyWhen | string | NotifyWhen type alias for when notify should be used |  |

#### Organization

It is used multi-tenancy support in the platform.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| bundle_id | string | BundleID defines package or bundle | No |
| created_at | dateTime | CreatedAt created time | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| license_policy | string | LicensePolicy defines license policy | No |
| max_concurrency | long | MaxConcurrency defines max number of jobs that can be run concurrently by org | No |
| org_unit | string | OrgUnit defines org-unit | No |
| owner_user_id | string | OwnerUserID defines owner user | No |
| parent_id | string | ParentID defines parent org | No |
| salt | string | Salt for password | No |
| sticky_message | string | StickyMessage defines an error message that needs user attention | No |
| subscription | [Subscription](#subscription) |  | No |
| updated_at | dateTime | UpdatedAt update time | No |

#### OrganizationConfig

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| id | string | ID defines UUID for primary key | No |
| name | string | Name defines name of property | No |
| organization_id | string | OrganizationID defines foreign key for Organization | No |
| secret | boolean | Secret for encryption | No |
| type | string | Type defines type of property value | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| value | string | Value defines value of property that can be string, number, boolean or JSON structure | No |

#### Period

Period defines enum for period of subscription

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Period | string | Period defines enum for period of subscription |  |

#### Policy

Policy defines enum for policy of subscription

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Policy | string | Policy defines enum for policy of subscription |  |

#### RequestState

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| RequestState | string |  |  |

#### ResourceCriteriaConfig

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ResourceCriteriaConfig |  |  |  |

#### ResourceUsage

ResourceUsage defines use of a resource

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| count | long |  | No |
| end_date | dateTime |  | No |
| organization_id | string |  | No |
| remaining_quota | long |  | No |
| resource_type | [ResourceUsageType](#resourceusagetype) |  | No |
| start_date | dateTime |  | No |
| user_id | string |  | No |
| value | long |  | No |
| value_unit | string |  | No |

#### ResourceUsageType

ResourceUsageType type of usage

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ResourceUsageType | string | ResourceUsageType type of usage |  |

#### ServiceStatus

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| ConsecutiveFailures | integer (uint64) |  | No |
| ConsecutiveSuccess | integer (uint64) |  | No |
| HealthError | string |  | No |
| LastCheck | dateTime |  | No |
| Monitored | [Monitorable](#monitorable) |  | No |
| TotalFailures | integer (uint64) |  | No |
| TotalSuccess | integer (uint64) |  | No |

#### Subscription

Subscription defines subscription

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| active | boolean | Active flag | No |
| cpu_quota | long | CPUQuota  allowed cpu seconds | No |
| created_at | dateTime | CreatedAt creation time | No |
| disk_quota | long | DiskQuota allowed disk Mbytes | No |
| ended_at | dateTime | EndedAt ended-at | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| kind | [Kind](#kind) |  | No |
| organization_id | string | OrganizationID defines org | No |
| period | [Period](#period) |  | No |
| policy | [Policy](#policy) |  | No |
| price | long | Price of subscription in cents | No |
| remaining_cpu_quota | long | RemainingCPUQuota  cpu seconds | No |
| remaining_disk_quota | long | RemainingDiskQuota disk Mbytes | No |
| started_at | dateTime | StartedAt started-at | No |
| updated_at | dateTime | UpdatedAt update time | No |
| user_id | string | UserID defines user | No |

#### SystemConfig

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt job creation time | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| kind | string | Kind defines kind of config property | No |
| name | string | Name defines name of config property | No |
| scope | string | Scope defines scope such as default or org-unit | No |
| secret | boolean | Secret for encryption | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| value | string | Value defines value of config property | No |

#### TaskDefinition

The task definition represents definition of the task and instance of the task uses TaskExecution when a new
job is submitted and executed. Based on the definition, a task request is sent to remote ant follower
that supports method and tags of the task. A task response is then received and results are saved in
the database.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| after_script | [ string ] | AfterScript defines list of commands that are executed after main script for cleanup | No |
| allow_failure | boolean | AllowFailure means the task is optional and can fail without failing entire job | No |
| allow_start_if_completed | boolean | AllowStartIfCompleted  means the task is always run on retry even if it was completed successfully | No |
| always_run | boolean | AlwaysRun means the task is always run on execution even if the job fails. For example, a required task fails (without AllowFailure), the job is aborted and remaining tasks are skipped but a task defined as `AlwaysRun` is run even if the job fails. | No |
| artifact_ids | [ string ] | ArtifactIDs defines id of artifacts that are automatically downloaded for job-execution | No |
| await_forked_tasks | [ string ] | AwaitForkedTasks defines list of jobs to wait for completion | No |
| before_script | [ string ] | BeforeScript defines list of commands that are executed before main script | No |
| created_at | dateTime | CreatedAt job creation time | No |
| delay_between_retries | [Duration](#duration) |  | No |
| dependencies | [ string ] | Dependencies defines dependent tasks for downloading artifacts | No |
| description | string | Description of task | No |
| except | string | Except is used to filter task execution based on certain condition | No |
| fork_job_type | string | ForkJobType defines type of job to work | No |
| headers | object | Header defines HTTP headers | No |
| host_network | string | HostNetwork defines kubernetes/docker config for host_network | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_definition_id | string | JobDefinitionID defines foreign key for JobDefinition | No |
| job_version | string | JobVersion defines job version | No |
| messaging_reply_queue | string |  | No |
| messaging_request_queue | string |  | No |
| method | [TaskMethod](#taskmethod) |  | No |
| on_completed | string | OnCompleted defines next task to run based on completion | No |
| on_exit_code | object | OnExitCode defines next task to run based on exit code | No |
| on_failed | string | OnFailed defines next task to run based on failure | No |
| report_stdout | boolean | ReportStdout is used to send stdout as a report | No |
| resources | [BasicResource](#basicresource) |  | No |
| retry | long | Retry defines max number of tries a task can be retried where it re-runs failed tasks | No |
| script | [ string ] | Script defines list of commands to execute in container | No |
| tags | [ string ] | Tags are used to use specific followers that support the tags defined by ants. For example, you may start a follower that processes payments and the task will be routed to that follower | No |
| task_type | string | TaskType defines type of task | No |
| timeout | [Duration](#duration) |  | No |
| updated_at | dateTime | UpdatedAt job update time | No |
| url | string | URL to use | No |
| variables | object | Transient properties -- these are populated when AfterLoad or Validate is called | No |
| webhook | [Webhook](#webhook) |  | No |

#### TaskExecution

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| allow_failure | boolean | AllowFailure means the task is optional and can fail without failing entire job | No |
| ant_host | string | AntHost - host where ant ran the task | No |
| ant_id | string | AntID - id of ant with version | No |
| artifacts | [ [Artifact](#artifact) ] | Artifacts defines list of artifacts that are generated for the task | No |
| contexts | [ [TaskExecutionContext](#taskexecutioncontext) ] | Contexts defines context variables of task | No |
| cost_factor | double | CostFactor | No |
| count_services | long | CountServices | No |
| ended_at | dateTime | EndedAt job update time | No |
| error_code | string | ErrorCode captures error code at the end of job execution if it fails | No |
| error_message | string | ErrorMessage captures error message at the end of job execution if it fails | No |
| exit_code | string | ExitCode defines exit status from the job execution | No |
| exit_message | string | ExitMessage defines exit message from the job execution | No |
| failed_command | string | FailedCommand captures command that failed | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| job_execution_id | string | JobExecutionID defines foreign key for JobExecution | No |
| method | [TaskMethod](#taskmethod) |  | No |
| retried | long | Retried keeps track of retry attempts | No |
| started_at | dateTime | StartedAt job creation time | No |
| stdout | [ string ] |  | No |
| task_order | long | TaskOrder | No |
| task_state | [RequestState](#requeststate) |  | No |
| task_type | string | TaskType defines type of task | No |
| updated_at | dateTime | UpdatedAt job execution last update time | No |

#### TaskExecutionContext

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt task context creation time | No |
| id | string | ID defines UUID for primary key | No |
| name | string | Name defines name of property | No |
| secret | boolean | Secret for encryption | No |
| task_execution_id | string | TaskExecutionID defines foreign key for task-execution | No |
| type | string | Type defines type of property value | No |
| value | string | Value defines value of property that can be string, number, boolean or JSON structure | No |

#### TaskMethod

The ant followers registers with the methods that they support and the task is then routed
based on method, tags and concurrency of the ant follower.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| TaskMethod | string | The ant followers registers with the methods that they support and the task is then routed based on method, tags and concurrency of the ant follower. |  |

#### User

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Organization | [Organization](#organization) |  | No |
| auth_id | string | AuthID defines id from external auth provider | No |
| auth_provider | string | AuthProvider defines provider for external auth provider | No |
| bundle_id | string | BundleID defines package or bundle | No |
| created_at | dateTime | CreatedAt created time | No |
| email | string | Email defines email | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| max_concurrency | long | MaxConcurrency defines max number of jobs that can be run concurrently by org | No |
| name | string | Name of user | No |
| notify | object |  | No |
| organization_id | string | OrganizationID defines foreign key for Organization | No |
| picture_url | string | PictureURL defines URL for picture | No |
| salt | string | Salt for password | No |
| sticky_message | string | StickyMessage defines an error message that needs user attention | No |
| subscription | [Subscription](#subscription) |  | No |
| updated_at | dateTime | UpdatedAt update time | No |
| url | string | URL defines url | No |
| username | string | Username defines username | No |

#### UserInvitation

UserInvitation represents a user session

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt created time | No |
| email | string | Email defines invitee | No |
| expires_at | dateTime | ExpiresAt expiration time | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| invitation_code | string | InvitationCode defines code | No |
| invited_by_user_id | string | InvitedByUserID defines foreign key | No |
| org_unit | string | OrgUnit defines org-unit | No |
| organization_id | string | OrganizationID defines foreign key | No |

#### UserJobTypeKey

UserJobTypeKey defines key for job-type by user/org

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| GetJobType | string | GetJobType defines the type of job | No |
| GetJobVersion | string | GetJobVersion defines the version of job | No |
| GetOrganizationID | string | GetOrganizationID defines the organization-id of the job creator | No |
| GetUserID | string | GetUserID defines the user-id of the job creator | No |
| GetUserJobTypeKey | string | GetUserJobTypeKey defines a unique key for the user and job | No |

#### UserToken

Note: The JWT token is not directly stored in the database, just its hash and expiration.
Also, this can be used to revoke API tokens.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| created_at | dateTime | CreatedAt created time | No |
| expires_at | dateTime | ExpiresAt expiration time | No |
| id | string | gorm.Model ID defines UUID for primary key | No |
| organization_id | string | OrganizationID defines foreign key | No |
| sha256 | string | SHA256 defines sha of token | No |
| token_name | string | TokenName defines name of token | No |
| user_id | string | UserID `defines foreign key | No |

#### Webhook

Webhook structure defines config options for callback webhook

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| headers | object | Header defines http headers | No |
| query | object | Query defines URL query params | No |
| url | string | URL defines POST url | No |
