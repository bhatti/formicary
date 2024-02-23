# formicary
![formicary logo](public/assets/images/formicary.png)

The formicary is a distributed orchestration engine that allows to execute batch jobs, workflows or CI/CD pipelines based on docker, kubernetes, shell, http or messaging executors.

[![GoDoc](https://pkg.go.dev/badge/github.com/bhatti/formicary)](https://pkg.go.dev/github.com/bhatti/formicary)
[![Go Report Card](https://goreportcard.com/badge/github.com/bhatti/formicary)](https://goreportcard.com/report/github.com/bhatti/formicary)
[![Maintainability](https://api.codeclimate.com/v1/badges/99/maintainability)](https://codeclimate.com/github/bhatti/formicary/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/99/test_coverage)](https://codeclimate.com/github/bhatti/formicary/test_coverage)
![Docker Image Version (latest by date)](https://img.shields.io/docker/v/bhatti/formicary?label=Docker%20Image)

## Overview

The formicary is a distributed orchestration engine for executing background jobs and workflows that are executed remotely using
Docker/Kubernetes/Shell/HTTP/Websocket/Messaging or other protocols. A job comprises directed acyclic graph of tasks, where the task 
defines a unit of work. The formicary architecture is based on the *Leader-Follower* (or master/worker), *Pipes-Filter*, *Fork-Join* and *SEDA* deisgn  patterns. 
The queen-leader schedules and orchestrates the graph of tasks and ant-workers execute the work. The task work is distributed among ant-workers 
based on tags executor protocols such as Kubernetes, Docker, Shell, HTTP, etc.
The formicary uses an object-store for persisting or staging intermediate or final artifacts from the tasks, 
which can be used by other tasks as input for their work. This allows building stages of tasks using
*Pipes and Filter* and *SEDA* patterns, where artifacts and variables can be passed from one task to another so that output of a task 
can be used as input of another task. The *Fork/Join* pattern allows executing work in parallel and then joining the results at the end. 
The following is a list of its significant features:

 -   **Declarative Task/Job Definitions**: Tasks and Jobs are defined as DAGs using simple YAML configuration files, with support for GO-based templates for customization.
 -   **Authentication & Authorization:** The access to Formicary is secured using OAuth and OIDC standards.
 -   **Persistence of Artifacts**: Artifacts and outputs from tasks can be stored and used by subsequent tasks or as job inputs.
 -   **Extensible Execution Methods**: Supports a variety of execution protocols, including Docker, Kubernetes, HTTP, and custom protocols.
 -   **Quota:** Limit maximum allowed CPU, memory, and disk quota usage for each task.
 -   **Caching**: Supports caching for dependencies such as npm, maven, gradle, and python.
 -   **Encryption**: Secures confidential configurations in databases and during network communication.
 -   **Scheduling**: Cron-based scheduling for periodic job execution.
 -   **Optional and Finalized Tasks**: Supports optional tasks that may fail and finalized tasks that run regardless of job success or failure.
 -   **Child Jobs:** Supports spawning of child jobs based on Fork/Join patterns.
 -   **Retry Mechanisms**: Supports retrying of tasks or jobs based on error/exit codes.
 -   **Job Filtering and Priority**: Allows job/task execution filtering and prioritization.
 -   Job prioritization, job/task retries, and cancellation.
 -   **Resource based Routing**: Supports constraint-based routing of workloads for computing resources based on tags, labels, execution protocols, etc.
 -   **Monitoring, Alarms and Notifications**: Offers job execution reports, real-time log streaming, and email notifications.
 -   **Other:** Graceful and abrupt shutdown capabilities. Reporting and statistics on job outcomes and resource usage.
 - GO based templates for job-definitions so that you can define customized variables and actions.
 - Cron based scheduled processing where jobs can be executed at specific times or run periodically.
 - Optional tasks that can fail without failing entire job.
 - Finalized or always-run task that are executed regardless if the job fails or succeeds.
 - Child jobs using fork/await so that a job can spawn other jobs that are executed asynchronously and then joins the results later in the job workflow.
 - Job/Task retries where a failed job or task can be rerun for a specified number of times or based on error/exit codes. The job rety supports partial restart so that only failed tasks are rerun upon retries.
 - Filtering of jobs/task execution based on user-defined conditions or parameters.
 - Job priority, where higher priority jobs are executed before the low priority jobs.
 - Job cancellation that can cleanly stop job and task execution.
 - Applies CPU/Memory/Disk quota to tasks for managing available computing resources.
 - Provides reports and statistics on job outcomes and resource usage such as CPU, memory and storage.
 - Ant executors support multiple protocols that ants can register with queen node such as queue, http, websocket, docker, kubernetes, etc.
 - Pub/sub based events are used to propagate real-time updates of job/task executions to UI or other parts of the system other parts of the system.
 - Streaming of real-time Logs to the UI as job/tasks are processed.
 - Provides email notifications on job completion or failures.
 - Graceful shutdown of queen server and ant workers that can receive a shutdown signal and the server/worker processes
   stop accepting new work but waits until completion of in-progress work. Also, supports abrupt shutdown of queen server so that jobs can be resumed from the task that was in the progress. As the task work
   is handled by the ant worker, no work is lost.
 - Metrics/auditing/usage of jobs and user actions.

## Use-Cases
-------------

The [Formicary](https://github.com/bhatti/formicary) is designed for efficient and flexible job and task execution, adaptable to various complex scenarios, and capable of scaling according to the user base and task demands. Following is a list of its major use cases:

 -   **Complex Workflow Orchestration**: [Formicary](https://github.com/bhatti/formicary) is specially designed to run a series of integration tests, code analysis, and deployment tasks that depend on various conditions and outputs of previous tasks. [Formicary](https://github.com/bhatti/formicary) can orchestrate this complex workflow across multiple environments, such as staging and production, with tasks running in parallel or sequence based on conditions.
 -   **Image Processing Pipeline**: [Formicary](https://github.com/bhatti/formicary) supports artifacts management for uploading images to [S3](https://aws.amazon.com/s3/) compatible storage including [Minio](https://min.io/). It allows orchestrating a series of tasks for image resizing, watermarking, and metadata extraction, with the final output stored in an object store.
 -   **Automate Build, Test and Release Workflows**: A DevOps team can use [Formicary](https://github.com/bhatti/formicary) to trigger a workflow that builds the project, runs tests, creates a Release, uploads build artifacts to the release, and publishes the package to a registry like npm or PyPI.
 -   **Scheduled Data ETL Job**: A data engineering team can use [Formicary](https://github.com/bhatti/formicary) to manage scheduled ETL jobs that extract data from multiple sources, transform it, and load it into a data warehouse, with tasks to validate and clean the data at each step.
 -   **Machine Learning Pipeline**: A data science team can use [Formicary](https://github.com/bhatti/formicary) pipeline to preprocess datasets, train machine learning models, evaluate their performance, and, based on certain metrics, decide whether to retrain the models or adjust preprocessing steps.



## Requirements:

- GO 1.16+
- Install Docker https://hub.docker.com/search?type=edition&offering=community
- Kubernetes, e.g. Install minikube - https://minikube.sigs.k8s.io/docs/start/
- Uses Redis https://redis.io/ or Apache pulsar https://pulsar.apache.org for messaging
- Install Minio that is used for object-store and artifacts storage - https://min.io/download

### 3rd party Libraries
- GORM for O/R mapping - https://gorm.io/index.html
- Echo for web framework - https://echo.labstack.com/
- Goose for database migration - https://github.com/pressly/goose
- Viper for configuration - https://github.com/spf13/viper

## Version

- 0.1

## License

- AGPLv3 (GNU Affero General Public License)

## Docs

### Operations

#### Installation and Startup
[Installing formicary](docs/installation.md)

#### Queen/Ants Configuration
[Configuration for Queen (server) and Ants (workers)](docs/configuration.md)

### User Guides

[Getting Started](docs/getting_started.md)

[Building Pipelines](docs/pipelines.md)

[Parallel Pipelines with parent/child](docs/parallel_pipelines.md)

#### CD/CD Pipelines
- [Building CI/CD Pipelines](docs/cicd.md)
- [Building Node.js CI/CD](docs/node_ci.md)
- [Building GO CI/CD](docs/go_ci.md)
- [Building Python CI/CD](docs/python_ci.md)
- [Building Ruby CI/CD](docs/ruby_ci.md)
- [Building Android CI/CD](docs/android_ci.md)
- [Building Maven CI/CD](docs/maven_ci.md)


### Simple ETL Job
[ETL Examples](docs/etl_examples.md)

#### Public Plugins
[Developing Public Plugins](docs/plugins.md)

### Kubernetes
[Kubernetes Examples](docs/advanced_k8.md)

#### How-To Guides
- [How-to Guides](docs/howto.md)
- [Scheduling Jobs](docs/howto.md#Scheduling)
- [Job/Organization Configs](docs/howto.md#Configs)
- [Artifacts Expiration](docs/expire-artifacts.md)
- [Caching](docs/howto.md#Caching)
- [Webhooks](docs/howto.md#Webhooks)
- [PostCommit](docs/howto.md#PostCommit)
- [Multiple Exit Codes](docs/howto.md#OnExitCode)
- [Notifications](docs/howto.md#Notifications)
- [Building Docker images Using Docker-in-Docker](docs/dind.md)
- [Scanning containers using Trivy](docs/trivy-scan.md)
- [Advanced Kubernetes](docs/advanced_k8.md)
- [Using Templates](docs/templates.md)
- [Sensor Jobs](docs/sensor.md)
- [Retry and Exit Codes](docs/retry-exit.md)


#### Job / Task Definition Configuration Options
[Job / Task Definition Configuration](docs/definition_options.md)

#### API Docs
[API Docs](docs/apidocs.md)

#### Comparison
[Comparison with other frameworks and solutions](docs/comparison.md)
- [Migrating from Jenkins](docs/jenkins.md)
- [Migrating from Gitlab](docs/gitlab.md)
- [Migrating from Github Actions](docs/github.md)
- [Migrating from CircleCI](docs/circleci.md)
- [Migrating from Apache Airflow](docs/airflow.md)
 
### Migrating from Airflow
 [Apache Airflow](airflow.md)

### Code and Design

#### Architecture
[Formicary Architecture](docs/architecture.md)

#### Executors
[Ant Executors](docs/executors.md)
- [Docker executors](executors.md#Docker) for using docker containers.
- [Kubernetes executors](executors.md#Kubernetes) for using kubernetes containers.
- [REST executors](executors.md#REST) for invoking external REST APIs when executing a job.
- [Customized executors](executors.md#Customized) for building a customized messaging ant worker.

#### Development
[Formicary Development Guide](docs/development.md)

