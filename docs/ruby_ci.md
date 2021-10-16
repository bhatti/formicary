## Ruby CI/CD Examples


### CI Job Configuration
Following is an example of job configuration for a simple Ruby project:
```yaml
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

#### Job Type
The `job_type` defines type of the job, e.g.
```yaml
job_type: ruby-build-ci
```


#### Tasks
The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type
The `task_type` defines name of the task, e.g.
```yaml
- task_type: clone
```

##### Task method
The `method` defines executor type such as KUBENETES, DOCKER, SHELL, etc:
```yaml
  method: KUBERNETES
```

##### Docker Image
The `image` tag within `container` defines docker-image to use for execution commands, which is `golang:1.16-buster` for node application, e.g.
```yaml
  container:
    image: circleci/ruby:2.5.0-node-browsers
```

##### Working Directory
The `working_dir` tag specifies the working directory within the container, e.g.,
```yaml
  working_dir: /sample
```

##### Before Script Commands
The `before_script` defines an array of shell commands that are executed before the main script, e.g. `build`
task checks out code in the `before_script`.
```yaml
  before_script:
    - git clone https://github.com/Shopify/example-ruby-app.git .
    - bundle install --path vendor/bundle
    - dockerize -wait tcp://localhost:5432 -timeout 1m
```

##### Script Commands
The `script` defines an array of shell commands that are executed inside container, e.g.,
```yaml
  script:
    - cp sample.env .env
    - bundle exec appraisal install
    - bundle exec appraisal rake
```

##### Vendor Caching
Formicary also provides caching for directories that store 3rd party dependencies, e.g. 
following example shows how all ruby libraries can be cached:

```yaml
  cache:
    key: Gemfile.lock
    paths:
      - vendor/bundle
```

##### Environment Variables
The `environment` section defines environment variables to disable interactive git session so that git checkout
won't ask for the user prompt.

```yaml
   environment:
    PGHOST: localhost
    PGUSER: administrate
    RAILS_ENV: test
```

### Uploading Job Definition
You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    --data-binary @go-build-ci.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "ruby-build-ci", "params": {"GitCommitID": "$COMMIT", "GitBranch": "$BRANCH", "GitCommitMessage": "$COMMIT_MESSAGE"}}' $SERVER/api/jobs/requests
```
The above example kicks off `ruby-build-ci` job that you can see on the dashboard UI.

