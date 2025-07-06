# Comparison with Other Workflow & CI/CD Systems

The landscape of orchestration and CI/CD tools is vast. This guide helps you understand where Formicary fits in, its key differentiators, and how it compares to other popular solutions.

## At a Glance: Where Does Formicary Fit?

Formicary is best described as a **hybrid orchestration engine**. It blends the powerful Directed Acyclic Graph (DAG) capabilities of workflow managers like Apache Airflow with the practical CI/CD features of tools like GitLab CI, GitHub Actions, and Jenkins.

-   If you need **complex, dynamic, non-linear workflows** that go beyond simple build-and-test stages, Formicary is a strong choice.
-   If you want **native support for diverse execution environments** (Kubernetes, Docker, Shell, HTTP) under a single, unified declarative model, Formicary excels.
-   If you need a **self-hosted, extensible platform** with deep observability and fine-grained control over job execution, Formicary provides that.

## Feature Comparison Matrix

This table provides a high-level comparison of features across popular platforms.

| Feature | Formicary | Airflow | GitLab CI | GitHub Actions | CircleCI | Jenkins |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: |
| **Primary Use Case** | DAG/Workflow/CI | DAG | CI/CD | CI/CD | CI/CD | CI/CD |
| **Definition Format** | YAML | Python | YAML | YAML | YAML | Groovy DSL |
| **Kubernetes Native** | ✅ | ✅ | ✅ (Runner) | ✅ (Runner) | ✅ | Via Plugin |
| **Fine-Grained K8s Control** | ✅ | Partial | ❌ | ❌ | ❌ | ❌ |
| **Docker Native** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Shell Native Executor** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **HTTP Native Executor** | ✅ | Via Python | ❌ | ❌ | ❌ | ❌ |
| **Dynamic DAGs/Templates**| ✅ | ✅ | ❌ | ❌ | ❌ | Partial |
| **Artifacts** | ✅ | No | ✅ | ✅ | ✅ | ✅ |
| **Dependency Caching** | ✅ | No | ✅ | ✅ | ✅ | ✅ |
| **Job/Task Priority** | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Worker/Runner Tagging**| ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (Labels) |
| **Advanced Flow Control**| ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Job/Task Retries** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Optional/Always-Run** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Encrypted Secrets** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Real-time Logs** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Extensible Protocols** | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ (Plugins) |
| **Fork/Join Parallelism** | ✅ | No | No | No | No | No |

---

## Detailed Comparisons

### Apache Airflow

**Airflow** is the industry standard for authoring, scheduling, and monitoring data engineering workflows.

| Airflow Concept | Formicary Equivalent |
| :--- | :--- |
| Python DAG File | YAML Job Definition |
| Operator / Hook | `method` (Executor) |
| Sensor | `EXECUTING` state with `on_exit_code` |
| `schedule_interval` | `cron_trigger` |
| `params` / `default_args` | `job_variables` / request `params` |

**Key Differences & Formicary Advantages:**
-   **Declarative vs. Programmatic:** Formicary's YAML definitions are simpler and more accessible to a wider audience (DevOps, SREs) than Airflow's Python-based DAGs.
-   **CI/CD Native:** Formicary has first-class support for CI/CD concepts like artifact passing and dependency caching, which are not native to Airflow.
-   **Executor Flexibility:** While Airflow has a Kubernetes executor, Formicary's model makes it trivial to mix and match executors (`DOCKER`, `SHELL`, `HTTP`) within the same workflow without writing custom Python operators.

### GitLab CI

**GitLab CI** is a powerful and mature CI/CD tool tightly integrated with the GitLab platform.

| GitLab CI Concept | Formicary Equivalent |
| :--- | :--- |
| `.gitlab-ci.yml` | YAML Job Definition |
| `stage` | `task` (conceptually) |
| `job` | `task` (specifically) |
| `rules` / `only`/`except`| `skip_if` / `on_exit_code` |
| `services` | `services` |
| `cache` / `artifacts` | `cache` / `artifacts` |

**Key Differences & Formicary Advantages:**
-   **Workflow Complexity:** GitLab CI is primarily linear (defined by `stages`). Formicary is a true DAG, allowing for complex, non-linear workflows with multiple branches and joins based on granular exit codes.
-   **Extensibility:** Formicary's executor protocol is open, allowing for custom `methods` like `MESSAGING` or `WEBSOCKET`, which is not possible in GitLab CI.
-   **Platform Agnostic:** Formicary is a standalone system that can be triggered by any Git provider (GitHub, Bitbucket, etc.), not just GitLab.

### GitHub Actions

**GitHub Actions** is a hugely popular, event-driven CI/CD platform deeply integrated into the GitHub ecosystem.

| GitHub Actions Concept | Formicary Equivalent |
| :--- | :--- |
| Workflow `.yml` file | YAML Job Definition |
| `job` | `task` |
| `step` | `script` entry |
| `on` (event trigger) | Webhooks / `cron_trigger` |
| `services` | `services` |
| `actions/cache` | Native `cache` block |

**Key Differences & Formicary Advantages:**
-   **Stateful Orchestration:** Formicary is a stateful system with a central server, enabling advanced features like job prioritization, concurrency management, and a detailed dashboard with historical stats. GitHub Actions runners are more ephemeral.
-   **Complex Workflows:** Similar to GitLab, GitHub Actions is primarily suited for linear or simple fan-out/fan-in workflows. Formicary's DAG model supports much more complex orchestration.
-   **Native Features:** Caching and artifact management are native concepts in Formicary, whereas they are often handled by community actions (e.g., `actions/cache`) in GitHub.

### Jenkins

**Jenkins** is the original, highly versatile CI server, known for its vast plugin ecosystem.

| Jenkins Concept | Formicary Equivalent |
| :--- | :--- |
| Jenkinsfile | YAML Job Definition |
| `stage` | `Job` (conceptually) / `task` |
| `agent` | `method` + `container` + `tags` |
| `post` block | `on_completed` / `on_failed` / `always_run` |
| `triggers` | `cron_trigger` |
| `environment` | `environment` / `job_variables` |

**Key Differences & Formicary Advantages:**
-   **Declarative vs. Scripted:** Jenkins pipelines are written in a Groovy DSL, which is code that gets executed. This offers flexibility but can lead to complex, hard-to-maintain pipelines. Formicary's declarative YAML is easier to read, lint, and reason about.
-   **Configuration as Data:** In Formicary, the entire workflow is data. This makes it easier for the system to provide features like DAG visualization, wait-time estimation, and programmatic analysis, which are more difficult with the scripted nature of Jenkinsfiles.
-   **Modern Architecture:** Formicary is built on modern cloud-native principles with decoupled components, message queues, and native container support, often leading to a more streamlined and manageable setup than a Jenkins instance with many plugins.

**Migration Example: Jenkins to Formicary**

*A typical Jenkinsfile:*
```groovy
pipeline {
    agent any
    tools {
        maven 'MAVEN_PATH'
        jdk 'jdk8'
    }
    stages {
        stage("Checkout Code") {
            steps {
                git branch: 'master',
                url: "https://github.com/some/repo.git"
            }
        }
        stage("Building Application") {
            steps {
               sh "mvn clean package"
            }
        }
    }
}
```

*The equivalent in Formicary:*
```yaml
job_type: maven-ci-job
tasks:
- task_type: build
  method: DOCKER
  working_dir: /app
  container:
    image: maven:3.8-jdk-11
  cache:
    key_paths: [ "pom.xml" ]
    paths: [ ".m2/repository" ]
  before_script:
    - git clone https://github.com/some/repo.git .
  script:
    - mvn clean package
```
