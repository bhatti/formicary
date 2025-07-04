## Configuration

### Define Test config for queen server

.formicary-queen.yaml

```
common:
    id: id
    user_agent: ""
    proxy_url: ""
    external_base_url: http://localhost:7777
    block_user_agents:
        - Slackbot-LinkExpanding
    public_dir: ./public/
    http_port: 0
    queue:
        provider: REDIS_MESSAGING
        endpoints: []
        topic_tenant: public
        topic_namespace: default
        pulsar:
            connection_timeout: 30ns
            channel_buffer: 0
            oauth: {}
            max_reconnect_to_broker: 0
        kafka:
            brokers:
                - 192.168.1.102:19092
                - 192.168.1.102:29092
            username: ""
            password: ""
            certificate: ""
        username: ""
        password: ""
        token: ""
        tls: null
        defaultoptions: null
        retrymax: 5
        retrydelay: 1s
    s3:
        endpoint: 192.168.1.102:9000
        access_key_id: admin
        secret_access_key: password
        token: ""
        region: us-west-2
        prefix: formicary/
        bucket: formicary-artifacts
        encryption_password: ""
        useSSL: false
    redis:
        host: 192.168.1.102
        port: 6379
        password: ""
        ttl_minutes: 5ns
        pool_size: 0
        max_subscription_wait: 1m0s
    auth:
        enabled: false
        cookie_name: formicary-session
        jwt_secret: changeme
        secure: false
        google_client_id: ""
        google_client_secret: ""
        google_callback_host: localhost
        github_client_id: ""
        github_client_secret: ""
        github_callback_host: localhost
    container_reaper_interval: 1m0s
    monitor_interval: 2s
    registration_interval: 5s
    rate_limit_sec: 1
    development: true
    encryptionkey: ""
    debug: true
db:
    data_source: formicary_db.sqlite
    type: ""
    encryption_key: my-db-key
    max_idle_connections: 10
    max_open_connections: 20
jobs:
    job_scheduler_leader_interval: 15s
    job_scheduler_check_pending_jobs_interval: 1s
smtp:
    from_email: support@formicary.io
    from_name: ""
    provider: ""
    api_key: ""
    username: support@formicary.io
    password: sCFPimJ2bkPqhEE
    host: mail.formicary.io
    port: 0
notify:
    email_jobs_template_file: views/notify/email_notify_job.html
    slack_jobs_template_file: views/notify/slack_notify_job.txt
    verify_email_template_file: views/notify/verify_email.html
    user_invitation_template_file: views/notify/user_invitation.html
embedded_ant:
    tags:
        - docker
        - containers
        - embedded
    methods:
        - KUBERNETES
        - DOCKER
        - SHELL
        - HTTP_POST
    docker:
        registry:
            registry: ""
            username: ""
            password: ""
            pull_policy: ""
        host: ""
        labels:
            purpose: embedded-worker
            type: docker
        environment: {}
        helper_image: amazon/aws-cli
    kubernetes:
        registry:
            registry: ""
            username: ""
            password: ""
            pull_policy: ""
        namespace: default
        cluster_name: ""
        kubeconfig: ""
        host: ""
        helper_image: amazon/aws-cli
        service_account: formicary-ant
        image_pull_secrets:
            - docker-registry-secret
        init_containers: []
        allow_privilege_escalation: false
        dns_policy: ""
gateway_subscriptions:
    JobExecutionLifecycleEvent: true
    TaskExecutionLifecycleEvent: true
url_presigned_expiration_minutes: 720ns
default_artifact_expiration: 24000h0m0s
default_artifact_limit: 100000
subscription_quota_enabled: false
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
    id: ant-worker-id
    user_agent: "formicary-agent"
    proxy_url: ""
    external_base_url: ""
    block_user_agents: []
    public_dir: ./public/
    http_port: 0
    queue:
        provider: ""
        endpoints: []
        topic_tenant: ""
        topic_namespace: ""
        pulsar: null
        kafka: null
        username: ""
        password: ""
        token: ""
        tls: null
        defaultoptions: null
        maxconnections: 0
        maxmessagesize: 0
        maxfetchsize: 0
        connectiontimeout: null
        operationtimeout: null
        committimeout: null
        retrymax: 0
        retrydelay: null
    s3:
        endpoint: 192.168.1.102:9000
        access_key_id: admin
        secret_access_key: password
        token: ""
        region: us-west-2
        prefix: formicary/
        bucket: formicary-artifacts
        encryption_password: ""
        useSSL: false
    redis:
        host: 192.168.1.102
        port: 6379
        password: ""
        ttl_minutes: 0s
        pool_size: 0
        max_subscription_wait: 0s
    auth:
        enabled: true
        cookie_name: ""
        jwt_secret: ""
        max_age: 0s
        token_max_age: 0s
        secure: false
        google_client_id: ""
        google_client_secret: ""
        google_callback_host: ""
        github_client_id: ""
        github_client_secret: ""
        github_callback_host: ""
    container_reaper_interval: 0s
    monitor_interval: 0s
    monitoring_urls:
      docker: "192.168.1.102:2375"
    registration_interval: 0s
    dead_job_ids_events_interval: 0s
    max_streaming_log_message_size: 0
    max_job_timeout: 0s
    max_task_timeout: 0s
    rate_limit_sec: 0
    development: true
    encryptionkey: ""
    debug: true
tags: []
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
docker:
  host: "tcp://192.168.1.102:2375"
kubernetes:
    registry:
        registry: ""
        username: '@bhatti'
        password: "test"
        pull_policy: ""
    namespace: default
    cluster_name: ""
    kubeconfig: ""
    host: ""
    helper_image: ""
    allow_privilege_escalation: true
    pod_security_context:
      run_as_user: 0
```

Note: The configuration properties can also be specified via environment variables
per https://github.com/spf13/viper#working-with-environment-variables.
