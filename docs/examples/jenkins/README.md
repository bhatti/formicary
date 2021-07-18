# Migrating from Jenkins

Jenkins is a popular solution for building CI/CD pipelines and following mapping shows mapping between Jenkins and Formicary:

|     Jenkins  |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| agent | [executor](../../README.md#executor) and [ant](../../README.md#ant) | [Jenkins](https://www.jenkins.io/doc/book/pipeline/syntax) uses agents to accept work from the server and Formicary uses ants to accept remote work, which is then executed by different executors based on method.
| post | [except](../../README.md#except), [allow_failure](../../README.md#allow_failure), [always_run](../../README.md#always_run) and [templates](../../README.md#templates) | Jenkins uses post to execute additional steps afterwards and formicary uses `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| environment variables | [environment](../../README.md#environment) | Jenkins doesn't support this feature but a formicary job can define environment properties to set before executing a task.
| variables | [variables](../../README.md#variables) | Jenkins doesn't support this feature but a formicary job can define variables that can be used when executing a task.
| triggers | [cron_trigger](../../README.md#cron_trigger) | Jenkins uses triggers to execute a periodic job and formicary uses cron_trigger for similar feature.
| stage | [Job](../../README.md#Job) | Jenkins uses stage to define a step in pipeline and formicary uses job to define tasks to execute.
| stages | [Workflow](../../README.md#Workflow) | Jenkins uses stages to define pipeline of steps to execute and formicary uses workflow to define directed-acyclic-graph of tasks to execute.

