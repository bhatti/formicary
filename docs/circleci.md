# Migrating from CircleCI

CircleCI is a popular solution for building CI/CD pipelines and following mapping shows mapping between CircleCI and Formicary:

|     CircleCI |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| executor | [executor](executors.md) | [CircleCI](https://circleci.com/docs/2.0/executor-intro/) supports executors based on Linux, Mac & Windows and Formicary uses executor ants to accept remote work and execute them based on method.
| filters | [filter](definition_options#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | CircleCI uses filters to restrict execution by branch and formicary uses `filter`, `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| context | [environment](definition_options.md#environment) | CircleCI uses [context](https://circleci.com/docs/2.0/contexts/) to securely pass environment variables and a formicary job can define environment or configuration options to set properties/variables before executing a task.
| variables | [variables](definition_options.md#variables) | CircleCI uses [context](https://circleci.com/docs/2.0/contexts/) to securely pass variables and a formicary job can define variables that can be used when executing a task.
| triggers | [cron_trigger](definition_options.md#cron_trigger) | CircleCI uses [triggers](https://circleci.com/docs/2.0/triggers/) to execute a periodic job and formicary uses cron_trigger for similar feature.
| workflow | [Job](definition_options.md#Job) | CircleCI uses [workflow](https://circleci.com/docs/2.0/workflows/) to define a step in pipeline and formicary uses job and workflow to define a directed-acyclic-graph of tasks to execute.

