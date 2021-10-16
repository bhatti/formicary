# Migrating from Gitlab

Gitlab is a popular solution for building CI/CD pipelines and following mapping shows mapping between Gitlab and Formicary:

|     Gitlab |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| pipeline | [Job](definition_options.md#Job) | Gitlab uses [pipeline](https://docs.gitlab.com/ee/ci/pipelines/) to define a jobs & stages and formicary uses job and workflow to define a directed-acyclic-graph of tasks to execute.
| runner | [executor](executors.md) | [Gitlab](https://docs.gitlab.com/runner/) supports runner for execution and Formicary uses executor ants to accept remote work and execute them based on method.
| filters | [filter](definition_options#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | Gitlab allows filtering [pipelines](https://docs.gitlab.com/ee/ci/pipelines/) by branch, status & tag and formicary uses `filter`, `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| environment | [environment](definition_options.md#environment) | Gitlab uses [environment](https://docs.gitlab.com/ee/ci/environments/) to pass environment variables and a formicary job can define environment or configuration options to set properties/variables before executing a task.
| variables | [variables](definition_options.md#variables) | Gitlab uses [variables](https://docs.gitlab.com/ee/ci/variables/) to pass variables and a formicary job can define variables, request parameters or configuration for passing parameters to a task.
| scheduling | [cron_trigger](definition_options.md#cron_trigger) | Gitlab uses [schedule](https://docs.gitlab.com/ee/ci/pipelines/schedules.html) to execute a schedule job and formicary uses cron_trigger for similar feature.
| caching | [caching](definition_options.md#cache) | Gitlab uses [caching](https://docs.gitlab.com/ee/ci/caching/) syntax to cache dependencies whereas formicary uses cache option for similar feature.
| artifacts | [artifacts](definition_options.md#artifacts) | Gitlab uses [artifacts](https://docs.gitlab.com/ee/ci/pipelines/job_artifacts.html) syntax to generate artifacts whereas formicary uses `artifacts` for sharing data between tasks or generating final results.
| services | [services](definition_options.md#services) | Gitlab uses [services](https://docs.gitlab.com/ee/ci/services/) syntax to start database or other services along with the job and you can launch similar services in formicary using `services` configuration option.

## Sample Gitlab example
Here is a sample Gitlab example:
```
image: maven:latest

variables:
  MAVEN_CLI_OPTS: "-s .m2/settings.xml --batch-mode"
  MAVEN_OPTS: "-Dmaven.repo.local=.m2/repository"

cache:
  paths:
    - .m2/repository/
    - target/

build:
  stage: build
  script:
    - mvn $MAVEN_CLI_OPTS compile

test:
  stage: test
  script:
    - mvn $MAVEN_CLI_OPTS test

deploy:
  stage: deploy
  script:
    - mvn $MAVEN_CLI_OPTS deploy
  only:
    - master
```

Following is equivalent workflow in formicary:
```
job_type: maven-ci-job
tasks:
- task_type: build-test-deploy
  working_dir: /sample
  container:
    image: maven:3.8-jdk-11
  before_script:
    - git clone https://github.com/kiat/JavaProjectTemplate.git .
  environment:
    MAVEN_CLI_OPTS: "-s .m2/settings.xml --batch-mode"
    MAVEN_OPTS: "-Dmaven.repo.local=.m2/repository"
  cache:
    keys:
      - pom.xml
    paths:
      - .m2/repository/
      - target/
  script:
    - mvn $MAVEN_CLI_OPTS compile
    - mvn $MAVEN_CLI_OPTS test
    - mvn $MAVEN_CLI_OPTS deploy
```

## Limitations in Gitlab
Following are major limitations of github actions:
 - Gitlab doesn't provide any metrics or queue size whereas formicary provides detailed reporting, metrics and insights into queue size.
 - Gitlab provides limited support for partial restart and retries unlike formicary that provides a number of configuration parameters to recover from the failure.
 - Gitlab does not support priority of the jobs whereas formicary allows specifying priority of jobs for determining execution order of pending jobs.
 - Formicary provides more support for scheduling periodic or cron jobs.
 - Formicary includes several executors such as HTTP, Messaging, Shell, Docker and Kubernetes but Gitlab does not support extending executor protocol.
 - Formicary provides rich support for metrics and reporting on usage on resources and statistics on job failure/success.
 - Formicary provides plugin APIs to share common workflows and jobs among users.
