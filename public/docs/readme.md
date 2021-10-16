# formicary

The formicary is a distributed orchestration engine that allows to execute batch jobs, workflows or CI/CD pipelines based on docker, kubernetes, shell, http or messaging executors.

## Overview

The formicary is a distributed orchestration engine for executing background jobs and workflows that are executed remotely using
Docker/Kubernetes/Shell/HTTP/Messaging or other protocols. A job comprises directed acyclic graph of tasks, where the task 
defines a unit of work. The formicary architecture is based on the *Leader-Follower* (or master/worker) pattern 
where queen-leader schedules and orchestrates execution of the graph of tasks. The task work is distributed among ant-workers 
based on tags executor protocols such as Kubernetes, Docker, Shell, HTTP, etc.
The formicary uses an object-store for persisting or staging intermediate or final artifacts from the tasks, 
which can be used by other tasks as input for their work. This allows building stages of tasks using
*Pipes and Filter* pattern, where artifacts and variables can be passed from one task to another so that output of a task 
can be used as input of another task. The main use-cases for formicary include:
- Processing directed acyclic graphs
- Batch jobs such as ETL or other offline processing
- Scheduled batch processing such as clearing, settlement, etc
- Data Pipelines such as processing large size data in background
- CI/CD Pipelines for building, testing and deploying code
- Automation for repetitive tasks
- Building workflows of tasks that have complex dependencies and can interact with a variety of protocols

## Features:

- Declarative definition of a job consisting of directed acyclic graph (DAG) of tasks using a simple yaml configuration file.
- GO based templates for job-definitions so that you can define customized variables and actions. 
- Persistence of artifacts from tasks that can be used by other tasks or used as output of jobs.
- Extensible Method abstraction for supporting a variety of execution protocols such as Docker, Kubernetes HTTP, Messaging or other customized protocols.
- Caching of dependencies such as npm, maven, gradle, python, etc.
- Encryption for storing secured configuration in the database or while in network communication.
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
- Resource constraints based scheduling and routing where ants register with tags that support special annotations and tasks
  are routed based on tags defined in the job definition.
- Ant executors support multiple protocols that ants can register with queen node such as queue, http, docker, kubernetes, etc.
- Pub/sub based events are used to propagate real-time updates of job/task executions to UI or other parts of the system other parts of the system.
- Streaming of real-time Logs to the UI as job/tasks are processed. 
- Provides email notifications on job completion or failures.
- Authentication and authorization using OAuth, JWT and RBAC standards.
- Graceful shutdown of queen server and ant workers that can receive a shutdown signal and the server/worker processes
  stop accepting new work but waits until completion of in-progress work. Also, supports abrupt shutdown of queen server so that jobs can be resumed from the task that was in the progress. As the task work
  is handled by the ant worker, no work is lost.
- Metrics/auditing/usage of jobs and user actions.

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

#### Installation
[Installing formicary](docs/installation.md)

#### Running
[Running formicary](docs/running.md)

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

