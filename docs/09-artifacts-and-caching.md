# Guide: Artifacts and Caching

Formicary provides two powerful mechanisms for persisting data between tasks and jobs: **Artifacts** and **Caching**. While they seem similar, they serve different purposes.

-   **Artifacts** are for storing the *outputs* of a task, such as compiled binaries, test reports, or processed data. They are unique to each job run.
-   **Caching** is for storing the *dependencies* of a task, like `node_modules` or `vendor` directories. The goal is to speed up subsequent job runs by not re-downloading the same files.

## Artifacts

Artifacts are files or directories produced by a task that you want to save. They are uploaded to the S3-compatible object store at the end of a successful task execution.

### Defining Artifacts

You define artifacts within a task using the `artifacts` key.

| Property | Type | Description |
|---|---|---|
| `paths` | list | **Required.** A list of file or directory paths to be archived and uploaded. |
| `when` | string | Optional. When to upload artifacts. Can be `onSuccess` (default), `onFailure`, or `always`. |
| `expires_after` | duration | Optional. How long the artifact should be stored before being eligible for cleanup (e.g., `7d`, `24h`). Defaults to a system-wide setting. |

**Example:**
```yaml
- task_type: build-app
  script:
    - ./build.sh
    - ./generate-report.sh > report.txt
  artifacts:
    when: always
    paths:
      - build/app.bin
      - report.txt
```

### Using Artifacts in Downstream Tasks

To use artifacts from a previous task, a downstream task must declare a `dependency`. Formicary will automatically download and extract the artifacts from all dependent tasks into the current task's working directory before the `script` runs.

**Example:**
```yaml
- task_type: build
  script:
    - make
  artifacts:
    paths:
      - my-binary

- task_type: deploy
  dependencies: # This makes `my-binary` available
    - build
  script:
    - ./my-binary --deploy-to-production
```

## Caching

Caching is a powerful optimization that can dramatically speed up your jobs by reusing dependency files from previous runs.

### How Caching Works

1.  **At the start of a task:** Formicary calculates a cache key. If a cache archive matching this key exists from a previous successful job, it's downloaded and extracted.
2.  **At the end of a successful task:** Formicary archives the directories specified in the `paths` key and uploads them to the object store using the newly calculated cache key.

The cache is immutable. If the key doesn't match exactly, the cache is not used.

### Defining a Cache

You define a cache within a task using the `cache` key.

| Property | Type | Description |
|---|---|---|
| `paths` | list | **Required.** A list of directories to be cached. |
| `key` | string | A static or templated string to use as the cache key. Use this for sharing caches across different jobs or branches. |
| `key_paths` | list | A list of files whose content will be hashed to generate the cache key. This is the most common method, as it automatically invalidates the cache when dependencies change. |
| `expires_after`| duration | Optional. How long the cache should be stored. Defaults to a system-wide setting (typically longer than artifacts). |

**You must provide either `key` or `key_paths`.**

### Example: Caching Node.js Dependencies

This is a classic use case. We want to cache the `node_modules` directory and invalidate the cache only when `package-lock.json` changes.

```yaml
- task_type: build-node-app
  container:
    image: node:16-buster
  cache:
    key_paths: # The key is a hash of this file's content
      - package-lock.json
    paths: # These directories are what get cached
      - node_modules
      - .npm
  script:
    # `npm ci` is very fast when node_modules is already populated from the cache
    - npm ci --cache .npm --prefer-offline
    - npm run build
```

### Example: Caching Go Dependencies

Similarly, for a Go project, you can cache the vendor directory based on `go.sum`.

```yaml
- task_type: build-go-app
  container:
    image: golang:1.24
  cache:
    key_paths:
      - go.sum
    paths:
      - vendor
  before_script:
    - go mod vendor
  script:
    - make build
```
