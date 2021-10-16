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
| caching | [caching](definition_options.md#cache) | CircleCI uses [caching](https://circleci.com/docs/2.0/caching/) syntax to cache dependencies whereas formicary uses cache option for similar feature.
| artifacts | [artifacts](definition_options.md#artifacts) | CircleCI uses [artifacts](https://circleci.com/docs/2.0/artifacts/#uploading-artifacts) syntax to share data between jobs whereas formicary uses `artifacts` for similar feature.
| containers | [services](definition_options.md#services) | CircleCI uses [containers](https://circleci.com/docs/2.0/containers/) syntax to start database or other services along with the job and you can launch similar services in formicary using `services` configuration option.

## Sample CircleCI workflow
Here is a sample workflow for circle-ci job:
```
---
version: 2.1

commands:
  shared_steps:
    steps:
      - checkout

      # Restore Cached Dependencies
      - restore_cache:
          name: Restore bundle cache
          key: administrate-{{ checksum "Gemfile.lock" }}

      # Bundle install dependencies
      - run: bundle install --path vendor/bundle

      # Cache Dependencies
      - save_cache:
          name: Store bundle cache
          key: administrate-{{ checksum "Gemfile.lock" }}
          paths:
            - vendor/bundle

      # Wait for DB
      - run: dockerize -wait tcp://localhost:5432 -timeout 1m

      # Setup the environment
      - run: cp .sample.env .env

      # Setup the database
      - run: bundle exec rake db:setup

      # Run the tests
      - run: bundle exec rake

default_job: &default_job
  working_directory: ~/administrate
  steps:
    - shared_steps
    # Run the tests against multiple versions of Rails
    - run: bundle exec appraisal install
    - run: bundle exec appraisal rake

jobs:
  ruby-25:
    <<: *default_job
    docker:
      - image: circleci/ruby:2.5.0-node-browsers
        environment:
          PGHOST: localhost
          PGUSER: administrate
          RAILS_ENV: test
      - image: postgres:10.1-alpine
        environment:
          POSTGRES_USER: administrate
          POSTGRES_DB: ruby25
          POSTGRES_PASSWORD: ""

workflows:
  version: 2
  multiple-rubies:
    jobs:
      - ruby-25
```

Following is equivalent workflow in formicary:
```
job_type: ruby-build-ci
tasks:
- task_type: clone
  working_dir: /sample
  container:
    image: circleci/ruby:2.5.0-node-browsers
  cache:
    key: Gemfile.lock
    paths:
      - vendor/bundle
  privileged: true
  environment:
    PGHOST: localhost
    PGUSER: administrate
    RAILS_ENV: test
  before_script:
    - git clone https://github.com/Shopify/example-ruby-app.git .
    - bundle install --path vendor/bundle
    - dockerize -wait tcp://localhost:5432 -timeout 1m
  script:
    - cp sample.env .env
    - bundle exec appraisal install
    - bundle exec appraisal rake
    # Setup the database
    - bundle exec rake db:setup
    # Run the tests
    - bundle exec rake
  services:
    - name: postgres
      alias: postgres
      image: postgres:10.1-alpine
```

## Limitations in CircleCI
Following are major limitations of circle-ci:
 - CircleCI doesn't provide any metrics or queue size whereas formicary provides detailed reporting, metrics and insights into queue size.
 - CircleCI provides limited support for partial restart and retries unlike formicary that provides a number of configuration parameters to recover from the failure.
 - Formicary provides better support for optional and always-run tasks.
 - CircleCI does not allow specifying cpu, memory and storage limits whereas formicary allows these limits when using Kubernetes executors. 
 - CircleCI does not support priority of the jobs whereas formicary allows specifying priority of jobs for determining execution order of pending jobs.
 - Formicary provides more support for scheduling periodic or cron jobs.
 - Formicary includes several executors such as HTTP, Messaging, Shell, Docker and Kubernetes but Github does not support extending executor protocol.
 - Formicary provides better support for retries, timeout, optional and always-run tasks.
 - Formicary provides rich support for metrics and reporting on usage on resources and statistics on job failure/success.
 - Formicary provides plugin APIs to share common workflows and jobs among users.
