# Tutorials: Building CI/CD Pipelines

Formicary is a powerful tool for creating robust and flexible Continuous Integration and Continuous Delivery/Deployment (CI/CD) pipelines. Its architecture allows you to define multi-stage workflows that build, test, and deploy your applications in isolated, reproducible environments.

## CI/CD Building Blocks in Formicary

A typical CI/CD pipeline can be mapped directly to Formicary's core concepts:

-   **Pipeline Stage -> Task:** Each stage in your pipeline (e.g., `lint`, `test`, `build`, `deploy`) is represented by a `task` in your job definition.
-   **Workflow Logic -> `on_completed` / `on_exit_code`:** You define the flow from one stage to the next using these properties, creating a Directed Acyclic Graph (DAG) of your pipeline.
-   **Build Environment -> `container`:** Each task runs in a specific Docker container, ensuring you have the correct versions of languages, compilers, and tools.
-   **Passing Data -> `artifacts` & `dependencies`:** Stages can produce artifacts (like compiled binaries) which are then automatically passed to subsequent stages that declare a dependency.
-   **Speeding up Builds -> `cache`:** You can cache dependency directories (`node_modules`, `vendor`, `.m2/repository`) to avoid re-downloading them on every run.

### Common Patterns

#### Accessing Code Repositories

Most CI jobs start by checking out code. To handle private repositories, you should store your access token (e.g., a GitHub Personal Access Token) as an encrypted **Job Config**.

1.  **Store the Secret:**
    ```bash
    curl -H "Authorization: Bearer <YOUR_API_TOKEN>" \
         -H "Content-Type: application/json" \
         -d '{"Name": "GithubToken", "Value": "ghp_YourSecretTokenHere", "Secret": true}' \
         $SERVER/api/jobs/definitions/<job-id>/configs
    ```

2.  **Use it in your Script:**
    Reference the secret in your `git clone` command using Go template syntax. Formicary will inject the value at runtime.
    ```yaml
    before_script:
      - git clone https://{{.GithubToken}}@github.com/my-org/my-repo.git .
      - git checkout {{.GitBranch}}
    ```
    Formicary automatically redacts secret values from all logs.

#### Triggering from Git Events

You can automate your pipeline by triggering it from a GitHub webhook on every push. See the [Scheduling & Triggers Guide](./08-scheduling-and-triggers.md) for setup instructions. This will automatically provide useful parameters to your job, like:
-   `GitBranch`
-   `GitCommitID`
-   `GitCommitMessage`

## Language-Specific Examples

Explore these detailed guides to see complete, working CI/CD pipelines for popular languages and frameworks.

-   [**Go**](./ci-examples/go.md)
-   [**Node.js**](./ci-examples/node.md)
-   [**Python**](./ci-examples/python.md)
-   [**Ruby**](./ci-examples/ruby.md)
-   [**Maven (Java/Kotlin)**](./ci-examples/maven.md)
-   [**Android (Gradle)**](./ci-examples/android.md)
