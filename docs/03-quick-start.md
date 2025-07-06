# Quick Start: Running Your First Job

This guide will walk you through defining, uploading, and running a simple "Hello World" job in Formicary. By the end, you'll understand the basic workflow of the system.

This guide assumes you have Formicary running via the Docker Compose method described in the [Installation](./02-installation.md) guide.

## Step 1: The Job Definition

A Formicary **Job Definition** is a YAML file that describes the workflow. Let's create a simple one with two tasks: one creates a `hello.txt` file, and the second creates a `world.txt` file.

Create a file named `hello-world.yaml` with the following content:

```yaml:hello-world.yaml
job_type: hello-world
description: A simple getting started example
max_concurrency: 1
tasks:
- task_type: hello
  container:
    image: alpine:latest
  script:
    - echo "Hello" > hello.txt
  artifacts:
    paths:
      - hello.txt
  on_completed: world
- task_type: world
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

Let's break this down:
-   `job_type`: A unique name for this workflow.
-   `tasks`: A list of the steps in our job.
-   `task_type`: The name of a specific step.
-   `container`: Specifies the Docker image to run the task in. We're using the lightweight `alpine` image.
-   `script`: The shell commands to execute inside the container.
-   `artifacts`: Declares which files should be saved as output from the task.
-   `on_completed`: This is the key to our workflow. It tells Formicary to run the `world` task after the `hello` task completes successfully.
-   `dependencies`: This tells the `world` task that it needs the artifacts from the `hello` task. Formicary will automatically download `hello.txt` into the `world` task's working directory.

## Step 2: Upload the Job Definition

With the Formicary services running, use `curl` to upload your new job definition to the Queen server.

```bash
curl -X POST http://localhost:7777/api/jobs/definitions \
     -H "Content-Type: application/yaml" \
     --data-binary @hello-world.yaml
```

You should receive a JSON response confirming that the job definition was created. You can also verify this by navigating to the **Job Definitions** page in the dashboard at [http://localhost:7777/dashboard/jobs/definitions](http://localhost:7777/dashboard/jobs/definitions).

## Step 3: Run the Job

Now that Formicary knows *what* to do, let's tell it to *do it*. We submit a **Job Request**.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{"job_type": "hello-world"}'
```

This command submits a request to run the `hello-world` job.

## Step 4: Observe the Results

1.  **Go to the Dashboard:** Open [http://localhost:7777](http://localhost:7777) in your browser. You should see your new job request appear on the main page, transition from `PENDING` to `EXECUTING`, and finally to `COMPLETED`.

2.  **View Job Details:** Click on the Job ID to see the details. You will see the two tasks, `hello` and `world`, and their status.

3.  **Check the Logs:** Click on a task to view its execution logs. You'll see the output of the `echo` commands.

4.  **Download the Artifact:** In the job details page, find the "Artifacts" section. You will see the `output.txt` file produced by the `world` task. You can download it to verify its content is "Hello World".

## Congratulations!

You have successfully run your first job in Formicary. You've learned the fundamental workflow:
1.  **Define** a job in YAML.
2.  **Upload** the definition to the Queen.
3.  **Request** an execution of the job.
4.  **Monitor** the results.

### Next Steps

-   Explore more [Examples & Tutorials](./11-ci-cd-pipelines.md) to see more complex workflows.
-   Dive into the [Core Concepts](./05-concepts.md) to understand the system better.
-   Read about all the available options in the [Job Definitions](./06-job-definitions.md) guide.

