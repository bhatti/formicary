# CI/CD for Android Projects

This guide provides a complete example of a CI/CD pipeline for an Android application using Gradle.

### Full Job Definition

```yaml:android-ci.yaml
job_type: android-build-ci
max_concurrency: 1
tasks:
- task_type: build
  method: DOCKER
  working_dir: /app
  container:
    image: gradle:6.8.3-jdk8
  cache:
    key_paths:
      - build.gradle
      - app/build.gradle
    paths:
      - .gradle
      - /root/.gradle
  before_script:
    - git clone https://{{.GithubToken}}@github.com/android/sunflower.git .
  script:
    - ./gradlew build
  artifacts:
    paths:
      - app/build/outputs/apk/
  on_completed: unit-tests

- task_type: unit-tests
  method: DOCKER
  working_dir: /app
  container:
    image: gradle:6.8.3-jdk8
  dependencies:
    - build
  cache:
    key_paths:
      - build.gradle
      - app/build.gradle
    paths:
      - .gradle
      - /root/.gradle
  script:
    - ./gradlew test
  artifacts:
    paths:
      - app/build/reports/tests/
```

### Key Concepts Explained

-   **`container`:** We use a `gradle` image that includes a compatible JDK for Android development.
-   **`cache`:** Gradle caches dependencies in a `.gradle` directory. We cache both the project-local and the user-home cache directories. The cache key is derived from the project's `build.gradle` files.
-   **`artifacts`:** The `build` task saves the generated APK files, while the `test` task saves the HTML test reports.
-   **`dependencies`:** The `unit-tests` task depends on `build`, ensuring it runs on a successfully built codebase.






