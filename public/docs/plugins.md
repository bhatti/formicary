## Formicary Plugins

A plugin is a public job definition that can be invoked by other jobs. The plugin may define a variety of functionality
such as security testing, data analysis, etc. Though, A job definition can be shared by anyone in the organization but 
public plugin allows you to define a job that can be shared by any other user. A plugin can be uploaded by an 
organization by defining a job definition where the job-type begins with the organization bundle and it defines a semantic version 
such as 1.0 or 1.2.1.

### Example Plugin:
```yaml
job_type: io.formicary.stock-plugin
description: Simple Plugin example
public_plugin: true
sem_version: 1.0-dev
max_concurrency: 1
tasks:
  - task_type: extract
    method: KUBERNETES
    container:
      image: python:3.8-buster
    before_script:
      - pip install yfinance --upgrade --no-cache-dir
    script:
      - python -c 'import yfinance as yf;import json;stock = yf.Ticker("{{.Symbol}}");j = json.dumps(stock.info);print(j);' > stock.json
    artifacts:
      paths:
        - stock.json
    on_completed: transform
  - task_type: transform
    method: KUBERNETES
    tags:
      - builder
    container:
      image: alpine
    dependencies:
      - extract
    before_script:
      - apk --update add jq && rm -rf /var/lib/apt/lists/* && rm /var/cache/apk/*
    script:
      - jq '.ask,.bid' > askbid.txt
    artifacts:
      paths:
        - askbid.txt
    on_completed: load
  - task_type: load
    method: KUBERNETES
    tags:
      - builder
    dependencies:
      - transform
    script:
      - awk '{ sum += $1; n++ } END { if (n > 0) print sum / n; }' askbid.txt > avg.txt
    after_script:
      - ls -l
    container:
      image: alpine
    artifacts:
      paths:
        - avg.txt
```

### Plugin Name
The plugin name must start with the bundle id of the organization such as io.formicary.stock-plugin.

### Plugin Version
The plugin is identified by its name and a version. The plugin version uses semantic format such as major.minor.patch,
such as 1.2.5 for production release or 1.2-dev or 1.4.5-rc1 for the test release.

### Spawning Plugin Job
The formicary allows spawning other plugins from a job using `FORK_JOB` method, e.g.

```yaml
 - task_type: spawn-plugin
   method: FORK_JOB
   fork_job_type: io.formicary.stock-plugin
   fork_job_version: 1.0-dev
   variables:
     Symbol: {{.Symbol}}
   on_completed: wait-plugin
```

### Waiting for completion of Plugin Job
The formicary allows waiting for the plugins from a job using ``AWAIT_FORKED_JOB`, e.g.,
```yaml
 - task_type: wait-plugin
   method: AWAIT_FORKED_JOB
   await_forked_tasks:
     - spawn-plugin
```

### Uploading Plugin
The plugin can be uploaded just like any other job such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @io.formicary.stock-plugin.yaml $SERVER/api/jobs/definitions
```

### Invoking public plugin

Here is a sample job that invokes above public plugin:
```yaml
job_type: plugin-client
description: Client of a public plugin
max_concurrency: 1
tasks:
- task_type: call-plugin
  method: FORK_JOB
  fork_job_type: io.formicary.stock-plugin
  fork_job_version: 1.0-dev
  variables:
    Symbol: {{.Symbol}}
  on_completed: wait-plugin
- task_type: wait-plugin
  method: AWAIT_FORKED_JOB
  await_forked_tasks:
    - call-pulugin
```

In above example, `call-plugin` task spawns a job for `io.formicary.stock-plugin` passing symbol
as a parameter and then `wait-plugin` waits for its completion. 
*Note*: The artifacts of child or plugin jobs are automatically added to the parent job so that you can access them easily.


### Uploading Client Job
You can upload above client job as follows:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    --data-binary @plugin-client.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Client Job
You can then submit the client job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "plugin-client", "params": {"Symbol": "MSFT"}}' $SERVER/api/jobs/requests
```
