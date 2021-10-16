## CI/CD Pipelines

### Code Repository
The source code repository provides a single source of truth for the software code and manages code versions,
branches and users who can update the code.

### Artifact Management
A build process may generate several artifacts that are needed for the CI/CD pipeline or for deploying software.

### Continuous Integration
The CI is a software development process where new code changes are regularly built, tested and merged into source code
repository by the engineering team. The code checkin to the source code kicks an automated process to validate build
using automated tests to identify bugs and integration issues. Thus, it reduces amount of time for manual testing and
expedites shipping high quality code to the production environment.

### Continuous Delivery and Deployment
The continuous delivery & deployment automates the release to production instead of manual approval 
and deployment process.

## Building CI/CD with Formicary
You can implement CI/CD by defining a [job configuration](definition_options.md#job) and then uploading it to the server.
The steps in a build processes can be mapped to the tasks in the job configuration where each task can map to the stage
in build process such as `compile`, `test`, `deploy`, etc.

### Job Parameters and Variables
See [Variables](definition_options.md#variables) and [Request Parameters](definition_options.md#Params) for
setting up variables and parameters for the job configuration and request parameters, e.g.
```yaml
job_variables:
  Target: world
```
The job configuration uses GO templates, so you can use parameters or variables to replace the values, e.g.
```yaml
    - echo "{{.Target}}" > world.txt
```

### Environment Variables
See [Environment Variables](definition_options.md#environment) for configuring environment variables 
that you can access them inside the container, e.g.
```yaml
   environment:
     REGION: seattle
```


### Job / Organization Configs
See [Job / Organization Configs](howto.md#Configs) for managing secure configurations at job and organization level.

### Access Tokens for Source Code Repositories
See [Accessing Source Code Repositories](howto.md#Repositories_Access_Tokens) for accessing source code repositories.

### Starting Job Manually
See [Scheduling Manually](howto.md#Scheduling) for scheduling job manually.
You can submit a job as follows:

### Scheduling Job in future
See [Job Scheduling](howto.md#Scheduling_Future) for submitting a job at scheduled.

### Scheduling Job with regular interval
See [Job Filtering](definition_options.md#cron_trigger) for scheduling job at a regular interval.

### Github-Webhooks
See [Github-Webhooks](howto.md#Webhooks) for scheduling job using GitHub webhooks.

### PostCommit Hooks
See [Post-commit hooks](howto.md#PostCommit) for scheduling job using git post-commit hooks.

### Filtering Job Request
See [Job Filtering](definition_options.md#filter) for filtering scheduled job.

### CI/CD Pipelines with Formicary
Following examples show how you can use artifact-store, docker/kubernetes and directed acyclic graph support in formicary to
build CI/CD solutions:

- [Node.js CI/CD](node_ci.md)
- [GO CI/CD](go_ci.md)
- [Python CI/CD](python_ci.md)
- [Ruby CI/CD](ruby_ci.md)
- [Android CI/CD](android_ci.md)
- [Maven CI/CD](maven_ci.md)

