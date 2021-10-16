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

## Sample Jenkins pipeline
Here is a sample pipeline of Jenkins job:
```
pipeline {
    agent any
    tools {
        maven 'MAVEN_PATH'
        jdk 'jdk8'
    }
    stages {
        stage("Tools initialization") {
            steps {
                sh "mvn --version"
                sh "java -version"
            }
        }
        stage("Checkout Code") {
            steps {
                git branch: 'master',
                url: "https://github.com/iamvickyav/spring-boot-data-H2-embedded.git"
            }
        }
        stage("Building Application") {
            steps {
               sh "mvn clean package"
            }
        }
 }
```

Following is equivalent workflow in formicary:
```
job_type: maven-ci-job
tasks:
- task_type: info
  container:
    image: maven:3.8-jdk-11
  script:
    - mvn --version
    - java --version
  on_completed: build
- task_type: build
  working_dir: /sample
  container:
    image: maven:3.8-jdk-11
  before_script:
    - git clone https://github.com/iamvickyav/spring-boot-data-H2-embedded.git .
  environment:
    MAVEN_OPTS: "-Dmaven.repo.local=.m2/repository"
  cache:
    keys:
      - pom.xml
    paths:
      - .m2/repository/
      - target/
  script:
    - mvn clean package
```

## Limitations in Jenkins
Following are major limitations of github actions:
 - Jenkins provides limited support for partial restart and retries unlike formicary that provides a number of configuration parameters to recover from the failure.
 - Jenkins does not provide support for optional or always-run tasks.
 - Jenkins actions do not allow specifying cpu, memory and storage limits whereas formicary allows these limits when using Kubernetes executors. 
 - Jenkins actions do not support priority of the jobs whereas formicary allows specifying priority of jobs for determining execution order of pending jobs.
 - Formicary includes several executors such as HTTP, Messaging, Shell, Docker and Kubernetes but Github does not support extending executor protocol.
 - Formicary provides rich support for metrics and reporting on usage on resources and statistics on job failure/success.
 - Formicary provides plugin APIs to share common workflows and jobs among users.
