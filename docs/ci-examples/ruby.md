# CI/CD for Ruby Projects

This guide provides a complete example of a CI/CD pipeline for a Ruby on Rails application, including setting up a database service.

### Full Job Definition

```yaml:ruby-ci.yaml
job_type: ruby-build-ci
max_concurrency: 1
tasks:
- task_type: build-and-test
  method: DOCKER
  working_dir: /app
  container:
    image: circleci/ruby:2.5.0-node-browsers
  services:
    - name: postgres-db
      alias: postgres
      image: postgres:10.1-alpine
      environment:
        POSTGRES_USER: administrate
        POSTGRES_DB: ruby_test_db
        POSTGRES_PASSWORD: ""
  cache:
    key_paths:
      - Gemfile.lock
    paths:
      - vendor/bundle
  environment:
    PGHOST: postgres
    PGUSER: administrate
    RAILS_ENV: test
  before_script:
    - git clone https://github.com/Shopify/example-ruby-app.git .
    - bundle install --path vendor/bundle
    - |
      echo "Waiting for PostgreSQL to be ready..."
      apt-get update && apt-get install -y postgresql-client
      until pg_isready -h postgres -p 5432 -U administrate; do
        sleep 1;
      done
      echo "PostgreSQL is ready!"
  script:
    - cp .sample.env .env
    - bundle exec appraisal install
    - bundle exec rake db:setup
    - bundle exec rake
```

### Key Concepts Explained

-   **`services`:** We define a PostgreSQL database as a service. The main container can access it using the hostname `postgres` (its alias).
-   **`environment`:** We set environment variables (`PGHOST`, `PGUSER`) so that Rails knows how to connect to the database service.
-   **`cache`:** Gems are installed to `vendor/bundle` and this directory is cached. The cache is invalidated when `Gemfile.lock` changes.
-   **Health Check:** The `before_script` includes a loop that waits for the PostgreSQL service to become available before proceeding, ensuring tests don't fail due to connection errors.
