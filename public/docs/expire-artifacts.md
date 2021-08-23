## Expire Artifacts

Following example shows how to use `EXPIRE_ARTIFACTS` method to expire old artifacts:
```yaml
job_type: artifacts-expiration
cron_trigger: 0 * * * *
tasks:
- task_type: expire
  method: EXPIRE_ARTIFACTS
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: artifacts-expiration
```

#### Cron Schedule

The `cron_trigger` defines schedule to run this job periodically, e.g. it will run above job every hour.

```yaml
cron_trigger: 0 * * * *
```

#### Tasks

The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type

The `task_type` defines name of the task, e.g.

```yaml
- task_type: expire
```

##### Method

The `method` defines type of tasklet to run, e.g.

```yaml
  method: EXPIRE_ARTIFACTS
```

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @artifacts-expiration.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "artifacts-expiration" }' $SERVER/api/jobs/requests
```

