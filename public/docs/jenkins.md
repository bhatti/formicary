# Migrating from Jenkins

Jenkins is a popular solution for building CI/CD pipelines and following mapping shows mapping between Jenkins and Formicary:

|     Jenkins  |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| agent | [executor](executors.md) | [Jenkins](https://www.jenkins.io/doc/book/pipeline/syntax) uses agents to accept work from the server and Formicary uses executor ants to accept remote work, which is then executed based on method.
| post | [filter](definition_options.md#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | Jenkins uses post to execute additional steps afterwards and formicary uses `filter`, `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| environment variables | [environment](definition_options.md#environment) | Jenkins doesn't support this feature but a formicary job can define environment properties to set before executing a task.
| variables | [variables](definition_options.md#variables) | Jenkins doesn't support this feature but a formicary job can define variables that can be used when executing a task.
| triggers | [cron_trigger](definition_options.md#cron_trigger) | Jenkins uses triggers to execute a periodic job and formicary uses cron_trigger for similar feature.
| stage | [Job](definition_options.md#Job) | Jenkins uses stage to define a step in pipeline and formicary uses job to define tasks to execute.
| stages | [Workflow](definition_options.md#Workflow) | Jenkins uses stages to define pipeline of steps to execute and formicary uses a job workflow to define directed-acyclic-graph of tasks to execute.

