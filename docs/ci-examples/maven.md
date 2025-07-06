# CI/CD for Maven Projects

This guide provides a complete example of a CI/CD pipeline for a Java project using Maven.

### Full Job Definition

```yaml:maven-ci.yaml
job_type: maven-build-ci
max_concurrency: 1
tasks:
- task_type: build
  method: DOCKER
  container:
    image: maven:3.8-jdk-11
  working_dir: /app
  cache:
    key_paths:
      - pom.xml
    paths:
      - .m2/repository
      - target
  before_script:
    - git clone https://github.com/kiat/JavaProjectTemplate.git .
  script:
    - mvn clean package
  artifacts:
    paths:
      - target/*.jar
```

### Key Concepts Explained

-   **`container`:** We use an official `maven` image which comes with both Maven and a JDK pre-installed.
-   **`cache`:**
    -   We use `pom.xml` to generate the cache key. Any change to dependencies in the `pom.xml` will invalidate the cache.
    -   We cache the local Maven repository (`.m2/repository`) where all downloaded JARs are stored.
    -   We also cache the `target` directory.
-   **`artifacts`:** After the build, we save the resulting `.jar` file from the `target` directory as a job artifact.

