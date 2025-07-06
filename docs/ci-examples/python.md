# CI/CD for Python Projects

This guide provides a complete example of a CI/CD pipeline for a Python application. The pipeline installs dependencies into a virtual environment, caches them, and runs tests.

### Full Job Definition

```yaml:python-ci.yaml
job_type: python-ci
max_concurrency: 1
skip_if: '{{if ne .GitBranch "main"}}true{{end}}'
tasks:
- task_type: test
  method: DOCKER
  working_dir: /app
  container:
    image: python:3.9-buster
  environment:
    PIP_CACHE_DIR: /app/.cache/pip
  cache:
    key: "pip-cache-{{ checksum \"sample/setup.py\" }}" # Or requirements.txt
    paths:
      - .cache/pip
      - venv
  before_script:
    - python -V
    - pip install virtualenv
    - virtualenv venv
    - . venv/bin/activate
    - git clone https://github.com/pypa/sampleproject.git sample
  script:
    - cd sample && python setup.py test
  on_completed: release
- task_type: release
  method: DOCKER
  working_dir: /app
  container:
    image: python:3.9-buster
  dependencies:
    - test
  cache:
    key: "pip-cache-{{ checksum \"sample/setup.py\" }}"
    paths:
      - .cache/pip
      - venv
  script:
    - echo "Release task can now build a wheel or deploy"
    - ls -la .cache/pip venv
```

### Key Concepts Explained

-   **Virtual Environment:** We use `virtualenv` to create an isolated `venv` directory for our dependencies.
-   **`cache`:** We cache both the `pip` cache directory (`.cache/pip`) and the entire `venv` directory. The cache key is generated from the `setup.py` file, so it's invalidated when dependencies change.
-   **`dependencies`:** The `release` task depends on the `test` task, ensuring it runs only after tests pass. It reuses the same cache by using the same `key`.

