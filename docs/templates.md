## Using Templates in Job Definition

Following example shows how you can use templates in job definitions to dynamically define job behavior:
```yaml
job_type: iterate-job
parse_template_on_load: true
tasks:
{{- range $val := Iterate 5 }}
- task_type: task-{{$val}}
  script:
    - echo executing task for {{$val}}
  container:
    image: alpine
  {{ if lt $val 4 }}
  on_completed: task-{{ Add $val 1}}
  {{ end  }}
{{ end  }}
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: iterate-job
```

#### Tasks

The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Range Loop
The `range` keyword defines a loop that is executed 5 times, e.g.
```
{{- range $val := Iterate 5 }}
```

##### Task Type

The `task_type` defines name of the task, however above template defines task-type dynamically, e.g.

```yaml
- task_type: task-{{$val}}
```

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, which in this using template variables, e.g.,

```yaml
  script:
    - echo executing task for {{$val}}
```

##### If condition
The if condition is used to define next task to run dynamically, e.g.
```
  {{ if lt $val 4 }}
  on_completed: task-{{ Add $val 1}}
  {{ end  }}
```

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @iterate-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "iterate-job" }' $SERVER/api/jobs/requests
```

