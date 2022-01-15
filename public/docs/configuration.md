## Configuration

### Define Test config for queen server

.formicary-queen.yaml

```
common:
    id: "queen-server-id"
    user_agent: "formicary-agent"
    proxy_url: ""
    http_port: 0
    pulsar:
        url: test
        connection_timeout: 0s
        channel_buffer: 0
        oauth: {}
        topic_suffix: ""
        topic_tenant: ""
        topic_namespace: ""
        max_reconnect_to_broker: 0
    kafka:
        brokers:
        - localhost:9092
    s3:
        endpoint: "localhost:9000"
        accessKeyID: "admin"
        secretAccessKey: "password"
        region: "us-west-2"
        bucket: "formicary-artifacts"
        prefix: "formicary/"
        encryptionPassword: ""
        useSSL: false
    redis:
        host: test
        port: 6379
        password: ""
        ttl_minutes: 5
        pool_size: 0
        max_subscription_wait: 1m0s
    messaging_provider: REDIS_MESSAGING
    container_reaper_interval: 60s
    monitor_interval: 2s
    monitoring_urls:
        docker: "localhost:2375"
    registration_interval: 5s
    max_streaming_log_message_size: 1024
    max_job_timeout: 4h
    max_task_timeout: 1h
db:
    data_source: .formicary_db.sqlite
    db_type: sqlite
    max_idle_connections: 10
    max_open_connections: 20
    connection_max_idle_time: 1h0m0s
    connection_max_life_time: 4h0m0s
    encryption_key: my-db-key
jobs:
    ant_reservation_timeout: 30s
    ant_registration_alive_timeout: 15s
    job_scheduler_leader_interval: 15s
    job_scheduler_check_pending_jobs_interval: 1s
    db_object_cache: 60s
    orphan_requests_timeout: 60s
    orphan_requests_update_interval: 15s
    not_ready_max_wait: 30
    max_schedule_attempts: 1000
    disable_job_scheduler: false
    max_fork_tasklet_capacity: 100
    max_fork_await_tasklet_capacity: 100
    expire_artifacts_tasklet_capacity: 100
    max_messaging_tasklet_capacity: 100
gateway_subscriptions:
    JobExecutionLifecycleEvent: true
    LogEvent: true
    TaskExecutionLifecycleEvent: true
url_presigned_expiration_minutes: 720s
```

Note: The formicary config uses sqlite by default where database file is stored in current directory as `.formicary_db.sqlite` but you can change it to other relational database such as mysql, e.g.,
```
db_type: mysql
data_source: formicary_user_dev:formicary_pass@tcp(localhost:3306)/formicary_dev?charset=utf8mb4&parseTime=true&loc=Local
```

### Define Test config for ant worker

.formicary-ant.yaml

```
common:
    id: my-ant-id
    user_agent: "formicary-agent"
    proxy_url: ""
    http_port: 0
    public_dir: "./public/"
    s3:
        endpoint: 127.0.0.1:9000
        accessKeyID: admin
        secretAccessKey: password
        token: ""
        region: us-west-2
        prefix: formicary/
        bucket: formicary-artifacts
        encryptionPassword: ""
        useSSL: false
    pulsar:
        url: ""
        connection_timeout: 0s
        channel_buffer: 0
        oauth: {}
        topic_suffix: ""
        topic_tenant: ""
        topic_namespace: ""
        max_reconnect_to_broker: 0
    kafka:
        brokers:
        - localhost:9092
    redis:
        host: 127.0.0.1
        port: 6379
        password: ""
        ttl_minutes: 0s
        pool_size: 0
        max_subscription_wait: 0s
    messaging_provider: REDIS_MESSAGING
    container_reaper_interval: 0s
    monitor_interval: 0s
    monitoring_urls:
        docker: "localhost:2375"
    registration_interval: 0s
    max_streaming_log_message_size: 0
    max_job_timeout: 0s
    max_task_timeout: 0s
tags: 
  - tag1
  - tag2
methods:
  - DOCKER
  - KUBERNETES
  - SHELL
  - HTTP_GET
  - HTTP_POST_FORM
  - HTTP_POST_JSON
  - HTTP_PUT_FORM
  - HTTP_PUT_JSON
  - HTTP_DELETE
  - WEBSOCKET
docker:
    registry:
        registry: ""
        username: ""
        password: ""
        pull_policy: ""
    host: tcp://192.168.1.104:2375
    labels: {}
    environment: {}
    helper_image: ""
kubernetes:
    registry:
        registry: ""
        username: ""
        password: ""
        pull_policy: if-not-present
    host: ""
    namespace: default
    helper_image: ""
    service_account: default
    init_containers: []
    image_pull_secrets: []
    allow_privilege_escalation: false
    dns_policy: "none"
    dns_config: null
    volumes:
        host_path: []
        pvc: []
        config_map: []
        secret: []
        empty_dir: []
    pod_security_context:
        fs_group: 100
        run_as_group: 100
        run_as_non_root: true
        run_as_user: 1000
        supplemental_groups:
          - 200
          - 300
    cap_add:
      - NET_RAW
      - CAP1
    cap_drop:
      - CAP2
    environment: {}
output_limit: 67108864
max_capacity: 10
termination_grace_period_seconds: 10
await_running_period_seconds: 60
poll_interval: 3
poll_timeout: 100
poll_interval_before_shutdown: 0s
poll_attempts_before_shutdown: 3
```

Note: The configuration properties can also be specified via environment variables
per https://github.com/spf13/viper#working-with-environment-variables.
