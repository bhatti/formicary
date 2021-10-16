# Migrating from Github Actions

Github actions is a popular solution for building CI/CD pipelines and following mapping shows mapping between Github and Formicary:

|     Github |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| runner | [executor](executors.md) | [Github](https://docs.github.com/en/actions/hosting-your-own-runners/adding-self-hosted-runners) supports runner for execution and Formicary uses executor ants to accept remote work and execute them based on method.
| workflow | [workflow](definition_options#workflow) | Github uses [workflows](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions) to define automated process for executing jobs whereas formicary uses workflow/dag to define a single job with dependencies between tasks.
| actions | [task](definition_options#tasks) | Github uses [actions](https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions) to execute tasks based on event. 
| job | [job](definition_options#job) | Github uses [job](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions#jobs) to define one or more steps whereas the formicary jobs serve similar purpose.
| step | [task](definition_options#tasks) | Github uses [step](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions#jobsjob_idsteps) within a job to run commands, whereas the formicary tasks serve similar purpose.
| environment | [environment](definition_options.md#environment) | Github uses [env](https://docs.github.com/en/actions/learn-github-actions/environment-variables/) syntax to pass environment variables and a formicary job can define environment or configuration options to set properties/variables before executing a task.
| variables | [variables](definition_options.md#variables) | Github uses [variables](https://docs.github.com/en/actions/learn-github-actions/essential-features-of-github-actions#using-variables-in-your-workflows) to pass variables whereas a formicary job can define variables, request parameters or configuration for passing parameters to a task.
| sharing data | [artifacts](definition_options.md#artifacts) | Github uses [uses/with](https://docs.github.com/en/actions/learn-github-actions/essential-features-of-github-actions#sharing-data-between-jobs) syntax to share data between jobs whereas formicary uses `artifacts` for similar feature.
| caching | [caching](definition_options.md#cache) | Github uses [caching](https://docs.github.com/en/actions/learn-github-actions/managing-complex-workflows#caching-dependencies) syntax to cache dependencies whereas formicary uses cache option for similar feature.
| services | [services](definition_options.md#services) | Github uses [services](https://docs.github.com/en/actions/learn-github-actions/managing-complex-workflows#using-databases-and-service-containers) syntax to start database or other services along with the job and you can launch similar services in formicary using `services` configuration option.
| labels/routing | [tags](definition_options.md#tags) | Github uses [labels](https://docs.github.com/en/actions/learn-github-actions/managing-complex-workflows#using-labels-to-route-workflows) for routing jobs to specific runners, whereas formicary uses `tags` syntax to route tasks to specific ant workers.
| expression| [filter](definition_options#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | Github allows filtering jobs via [expressions](https://docs.github.com/en/actions/learn-github-actions/expressions) and formicary uses `filter`, `except`, `allow_failure`, `always_run` and GO templates to execute any conditional or post-processing tasks.
| scheduling | [cron_trigger](definition_options.md#cron_trigger) | Gitlab uses [schedule](https://docs.gitlab.com/ee/ci/pipelines/schedules.html) to execute a schedule job and formicary uses cron_trigger for similar feature.

## Sample Github action
Here is a sample action that is stored under .github/workflows/github-actions-demo.yml:
```
name: GitHub Actions Demo
on: [push]
jobs:
  Explore-GitHub-Actions:
    runs-on: ubuntu-latest
    env:
      MY_VAR: Hi there!
    steps:
      - run: echo "The job was automatically triggered by a ${{ github.event_name }} event."
      - run: echo "This job is now running on a ${{ runner.os }} server hosted by GitHub!"
      - run: echo "The name of your branch is ${{ github.ref }} and your repository is ${{ github.repository }}."
      - name: Check out repository code
        uses: actions/checkout@v2
      - run: echo "The ${{ github.repository }} repository has been cloned to the runner."
      - name: List files in the repository
        run: |
          ls ${{ github.workspace }}
        run: |
          echo $MY_VAR variable.
```

Following is equivalent workflow in formicary:
```
job_type: sample-actions
tasks:
- task_type: clone
  working_dir: /sample
  container:
    image: golang:1.16-buster
  before_script:
    - git clone https://{{.GithubToken}}@github.com/bhatti/git-actions.git .
  environment:
    MY_VAR: Hi there!
  script:
    - echo "The job was automatically triggered by {{ .UserID }} uesr."
    - echo "This job is now running on:"
    - uname -a
    - echo "The name of your branch is:"
    - git rev-parse --symbolic-full-name --abbrev-ref HEAD
    - git describe --always --long --dirty
    - git rev-parse --verify HEAD
    - git log -1 --pretty=format:'%an'
    - git log -1 --pretty=format:'%ae'
    - git log -1 --pretty=%B
    - "echo List files in the repository"
    - ls -l 
    - echo $MY_VAR variable.
```

## Limitations in Github
Following are major limitations of github actions:
 - caching of artifacts using actions/cache@v2 is not available on GitHub Enterprise Server (GHES). 
 - Github actions also don't support default environment for manually triggered jobs whereas formicary allows passing variables for providing environment and other parameters.
 - You can't invoke other actions from the actions unlike fork/await support in formicary.
 - Github doesn't provide any metrics or queue size whereas formicary provides detailed reporting, metrics and insights into queue size.
 - Github provides limited support for partial restart and retries unlike formicary that provides a number of configuration parameters to recover from the failure.
 - Formicary provides better support for optional and always-run tasks.
 - Though, Github actions are tied to actions on the Github repos, formicary provides more flexible options to share workflows and trigger jobs using APIs, post-commit, webhooks, etc.
 - Github actions do not allow specifying cpu, memory and storage limits whereas formicary allows these limits when using Kubernetes executors. 
 - Github actions do not support priority of the jobs whereas formicary allows specifying priority of jobs for determining execution order of pending jobs.
 - Formicary provides more support for scheduling periodic or cron jobs.
 - Formicary includes several executors such as HTTP, Messaging, Shell, Docker and Kubernetes but Github does not support extending executor protocol.
 - Formicary provides rich support for metrics and reporting on usage on resources and statistics on job failure/success.
 - Formicary provides plugin APIs to share common workflows and jobs among users.
