# Migrating from Gitlab

CGitlab is a popular solution for building CI/CD pipelines and following mapping shows mapping between Gitlab and Formicary:

|     CircleCI |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| runner | [executor](executors.md) | [Gitlab](https://docs.gitlab.com/runner/) supports runner for execution and Formicary uses executor ants to accept remote work and execute them based on method.
| filters | [filter](definition_options#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | Gitlab allows filtering [pipelines](https://docs.gitlab.com/ee/ci/pipelines/) by branch, status & tag and formicary uses `filter`, `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| environment | [environment](definition_options.md#environment) | Gitlab uses [environment](https://docs.gitlab.com/ee/ci/environments/) to pass environment variables and a formicary job can define environment or configuration options to set properties/variables before executing a task.
| variables | [variables](definition_options.md#variables) | Gitlab uses [variables](https://docs.gitlab.com/ee/ci/variables/) to pass variables and a formicary job can define variables, request parameters or configuration for passing parameters to a task.
| scheduling | [cron_trigger](definition_options.md#cron_trigger) | Gitlab uses [schedule](https://docs.gitlab.com/ee/ci/pipelines/schedules.html) to execute a schedule job and formicary uses cron_trigger for similar feature.
| pipeline | [Job](definition_options.md#Job) | Gitlab uses [pipeline](https://docs.gitlab.com/ee/ci/pipelines/) to define a jobs & stages and formicary uses job and workflow to define a directed-acyclic-graph of tasks to execute.

