# formicary

The formicary is a distributed "job management system" that allows to execute batch jobs, workflows or CI/CD pipelines based on docker, kubernetes, shell or http executors.

## Overview

The formicary is a distributed "job management system" for executing background jobs that are executed remotely using
Docker/Kubernetes/Shell/HTTP and other protocols. A job is comprises directed acyclic graph of tasks and job definition is defined in a yaml configuration file.
The architecture is based on a queen (leader) and ant (follower/worker) pattern
where queen node schedules and orchestrates execution of the graph of tasks. The task work is distributed among ant workers
that match tags specified by the task and executor protocols such as Kubernetes, Docker, Shell, HTTP, etc. The queen server
encompasses resource-manager, job-scheduler, job-launcher, job/task supervisors, where job-scheduler finds next job to 
execute based on resource-manager and hands-off job to job-launcher, which then uses job-supervisor to orchestrate the job.
The job-supervisor delegates job-execution to task-supervisor, which sends the request to a remote ant worker and then waits for the response.
After task completion, the job-supervisor finds the next task to execute based on exit-values of previous task and persists its state. 
The formicary uses an object-store for persisting or staging intermediate or final artifacts from the tasks, 
which can be used by other tasks as input for their work. This allows building stages of tasks using
Pipes and Filter pattern, where artifacts and variables can be passed from one task to another so that output of a task 
can be used as input of another task.

## Features:

- Declarative definition of a job consisting of directed acyclic graph (DAG) of tasks using a simple yaml configuration file.
- GO based templates for job-definitions so that you can define customized variables and actions. 
- Persistence of artifacts from tasks that can be used by other tasks or used as output of jobs.
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
- Resource constraints based scheduling where ants register with tags that support special annotations and tasks
  are routed based on tags defined in the job definition.
- Ant executors with support of multiple protocols that ants can use to connect to queen node such as queue, http, etc.
- Pub/sub based events are used to propagate real-time updates of job/task executions to UI or other parts of the system other parts of the system.
- Streaming of real-time Logs to the UI as job/tasks are processed. 
- Provides email notifications on job completion or failures.
- Provides reports and statistics on job outcomes and resource usage such as storage and CPU.
- Authentication and authorization using OAuth and JWT standards.
- Graceful shutdown of queen server and ant workers that can receive a shutdown signal and the server/worker processes
  stop accepting new work but waits until completion of in-progress work. Also, supports abrupt shutdown of queen server so that jobs can be resumed from the task that was in the progress. As the task work
  is handled by the ant worker, no worker is lost.
- Metrics/auditing/usage of jobs and user actions.

## Requirements:

- GO 1.15+
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
- [Caching](docs/howto.md#Caching)
- [Webhooks](docs/howto.md#Webhooks)
- [PostCommit](docs/howto.md#PostCommit)
- [Multiple Exit Codes](docs/howto.md#OnExitCode)
- [Building Docker images Using Docker-in-Docker](docs/dind.md)
- [Scanning containers using Trivy](docs/trivy-scan.md) for scanning containers.
- [Advanced Kubernetes](docs/advanced_k8.md)

#### Job / Task Definition Configuration Options
[Job / Task Definition Configuration](docs/definition_options.md)

#### API Docs
[API Docs](docs/apidocs.md)

#### Comparison
[Comparison with other frameworks and solutions](docs/comparison.md)

### Code and Design

#### Architecture
[Formicary Architecture](docs/architecture.md)

#### Executors
[Ant Executors](docs/executors.md)

#### Development
[Formicary Development Guide](docs/development.md)

