# Quick Start: Running Your First Job

This guide walks you through defining, uploading, and running a simple "Hello World" job in Formicary.

This guide assumes Formicary is running via Kubernetes as described in the [Installation](./02-installation.md) guide. If it isn't running yet:

```bash
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)"
kubectl apply -f k8s.yaml
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

## Step 1: The Job Definition

A Formicary **Job Definition** is a YAML file that describes the workflow. Create a file named `hello-world.yaml`:

```yaml
job_type: hello-world
description: A simple getting started example
max_concurrency: 1
tasks:
- task_type: hello
  method: KUBERNETES
  container:
    image: alpine:latest
  script:
    - echo "Hello" > hello.txt
  artifacts:
    paths:
      - hello.txt
  on_completed: world
- task_type: world
  method: KUBERNETES
  container:
    image: alpine:latest
  dependencies:
    - hello
  script:
    - cat hello.txt > output.txt
    - echo " World" >> output.txt
  artifacts:
    paths:
      - output.txt
```

Key fields:
- `job_type` — unique name for this workflow
- `method: KUBERNETES` — run each task in a fresh Kubernetes pod
- `container.image` — Docker image for the pod
- `script` — shell commands to run inside the container
- `artifacts` — files to save and pass to downstream tasks
- `on_completed` — next task to run after this one succeeds
- `dependencies` — tasks whose artifacts this task needs

## Step 2: Get an API Token

Log in at [http://localhost:7777](http://localhost:7777), go to **Profile → API Token**, and copy the JWT:

```bash
export FORMICARY_TOKEN="<token-from-ui>"
```

## Step 3: Upload the Job Definition

```bash
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/yaml" \
  --data-binary @hello-world.yaml
```

You can also verify it in the dashboard under **Job Definitions**.

## Step 4: Run the Job

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"job_type": "hello-world"}'
```

## Step 5: Observe the Results

1. Open [http://localhost:7777](http://localhost:7777) — the job appears on the dashboard, transitions `PENDING → EXECUTING → COMPLETED`.
2. Click the Job ID to see both tasks and their status.
3. Click a task to view its execution logs.
4. Find the `output.txt` artifact in the job details page and download it — it should contain "Hello World".

## Congratulations!

You have successfully run your first job in Formicary. The fundamental workflow:

1. **Define** a job in YAML
2. **Upload** the definition to the Queen
3. **Request** an execution
4. **Monitor** the results

### Next Steps

- See [AI Agents](./ai-agents.md) to set up autonomous GitHub/Jira workflows
- Read [Core Concepts](./05-concepts.md) to understand the system in depth
- Browse [Job Definitions](./06-job-definitions.md) for the full YAML reference
- See [Executors](./07-executors.md) for Kubernetes, Docker, Shell, and HTTP executor options
