## Job Definition
A job workflow of tasks defines directed-ayclic-graph of tasks to execute where each task is executed based on status of prior task.

### Job
A job defines following properties:

### Task definition
A task defines following properties:

#### task_type
The task_type defines type or name of the task.

#### method
The method defines executor to use for the task such as KUBERNETES, DOCKER, SHELL, etc.

#### environment
The environment defines environment variables that are set before execution of a task.

#### variables
The variables defines variables that can be used when executing a task.

#### cron_trigger
The cron_trigger defines a cron syntax that is used to execute the job periodically.

#### except
The task skips execution if except is true

#### allow_failure
The allow_failure marks the task as optional, so the job is not marked as FAILED if the task fails.

#### always_run
The always_run marks the task as finalized task, so it is always run regardless if the job succeeds or fails. It can be used to execute any cleanup at the end of job.

### templates
The job and task definitions support GO templates for defining variables that are initialized using user-defined parameters such as:

