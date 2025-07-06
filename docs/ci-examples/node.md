# CI/CD for Node.js Projects

This guide provides a complete example of a CI/CD pipeline for a Node.js application. The pipeline installs dependencies from cache, runs tests, and packages the application.

### Full Job Definition

```yaml:node-ci.yaml
job_type: node-build-ci
max_concurrency: 1
tasks:
- task_type: build
  method: DOCKER
  working_dir: /app
  container:
    image: node:16-buster
  cache:
    key_paths:
      - package-lock.json
    paths:
      - node_modules
      - .npm
  before_script:
    - git clone https://{{.GithubToken}}@github.com/bhatti/node-crud.git .
    - git checkout {{.GitBranch}}
    - npm ci --cache .npm --prefer-offline
  script:
    - npm install
    - tar -czf app.tgz .
  artifacts:
    paths:
      - app.tgz
  on_completed: test

- task_type: test
  method: DOCKER
  container:
    image: node:16-buster
  working_dir: /app
  dependencies:
    - build
  script:
    - tar -xzf app.tgz
    - npm install mocha chai supertest
    - chmod +x ./node_modules/.bin/*
    - npm test
```

### Key Concepts Explained

-   **`cache`:** This is the most critical part for Node.js performance. We cache `node_modules` and the `.npm` cache directory. The key is generated from `package-lock.json`, so the cache is only invalidated when dependencies change.
-   **`npm ci`:** In the `before_script`, we use `npm ci`. This command is optimized for CI environments. It installs dependencies exactly as defined in `package-lock.json` and is significantly faster than `npm install` when a `node_modules` folder already exists from the cache.
-   **`dependencies` & `artifacts`:** The `build` task creates a tarball (`app.tgz`) of the entire application, including installed dependencies. The `test` task declares a dependency on `build`, downloads this tarball, and runs the tests in a clean environment. This ensures the test environment is identical to the build environment.

### Running the Job

1.  **Store Your Git Token:** Securely store your GitHub token as a Job Config named `GithubToken`. See the main [CI/CD Pipelines Guide](../11-ci-cd-pipelines.md) for instructions.

2.  **Upload the Definition:**
    ```bash
    curl -H "Authorization: Bearer <API_TOKEN>" \
         -H "Content-Type: application/yaml" \
         --data-binary @node-ci.yaml \
         http://localhost:7777/api/jobs/definitions
    ```

3.  **Submit a Job Request:**
    ```bash
    curl -H "Authorization: Bearer <API_TOKEN>" \
         -H "Content-Type: application/json" \
         -d '{
               "job_type": "node-build-ci",
               "params": { "GitBranch": "main" }
             }' \
         http://localhost:7777/api/jobs/requests
    ```
