### Comparison with other Actions, DAG, Jobs, Pipeline or Workflow Solutions

| Feature               | Airflow   | Azure     | CircleCI  | Formicary    | GitHub    | Gitlab    | Jenkins     |
|-----------------------|-----------|-----------|-----------|--------------|-----------|-----------|-------------|
| **Tool Category**     | DAG       | CI-CD     | CD-CD     | DAG-Workflow | CI-CD     | CI-CD     | CI-CD       |
| **Definition Format** | Python    | UI        | YAML      | YAML         | YAML      | TOML      | DSL         |
| **Docker Images**     | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Indirectly  |
| **K8 Svc/Vols/labels**| Yes       | No        | No        | Yes          | No        | Yes       | No          |
| **K8 Lmt CPU/Memory** | Yes       | No        | No        | Yes          | No        | Yes       | No          |
| **Job/Task API**      | Partial   | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Request Params**    | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Artifacts**         | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **NPM/Mvn/.. Caching**| No        | Yes       | Yes       | Yes          | No        | Yes       | No          |
| **Dynamic Workflow**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Extensible Protocols**  | Yes/Operator | No | No        | Yes/Method   | No        | No        | No          |
| **Job Priority**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Task/Workers Tags** | Yes       | No        | No        | Yes          | No        | Yes       | Yes (Agents)|
| **Conditional Flow**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Cron/Scheduling**   | Yes       | Yes       | Yes       | Yes          | No        | Yes       | Yes         |
| **Resource Pools**    | Yes       | No        | No        | Yes          | No        | Yes       | No          |
| **Streaming Logs**    | No        | Yes       | Yes       | Yes          | No        | Yes       | Yes         |
| **Task Variables**    | Yes       | No        | Yes       | Yes          | NO        | Yes       | No          |
| **Job Config/Secrets**| Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Org Config/Secrets**| No        | No        | Yes       | Yes          | Yes       | No        | No          |
| **Encrypted Config**  | Yes/Fernet| No        | Yes       | Yes          | No        | Yes       | Yes         |
| **Template Support**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Shell Execution**   | Yes       | Yes       | No        | Yes          | No        | No        | Yes         |
| **HTTP Execution**    | via Python| via Script| via Script| Native Exec. | via Script| via Script| via Script  |
| **Messaging Exec**    | No        | No        | No        | Yes          | No        | No        | No          |
| **Queuing Library**   | Celery    | Internal  | Internal  | Redis, Kafka or Pulsar| Internal | Internal  | NA  |
| **Lifecycle Events**  | No        | No        | No        | Yes          | No        | No        | No          |
| **Job Retries**       | Yes       | Yes       | No        | Yes          | No        | Yes       | No          |
| **Job/Task Filtering**| No        | No        | No        | Yes          | No        | Yes       | No          |
| **Job Timeout**       | Yes       | Yes       | No        | Yes          | No        | Yes       | No          |
| **Task Retries**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Task Timeout**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Optional Tasks**    | No        | No        | No        | Yes          | No        | Yes       | No          |
| **Always-run Tasks**  | No        | No        | No        | Yes          | No        | Yes       | No          |
| **Delay before Retry**| Yes       | No        | No        | Yes          | No        | No        | No          |
| **JWT Auth**          | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Email Notifications** | Yes     | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Slack Notifications** | No      | No        | No        | Yes          | No        | Yes       | No          |
| **Tags based routing**| No        | No        | No        | Yes          | No        | Yes       | No          |
| **Webhooks**          | No        | No        | No        | Yes          | No        | No        | No          |
| **CPU/Disk Usage Reports** | No   | No        | No        | Yes          | No        | No        | No          |
| **CPU/Disk Quota **   | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Admin Dashboard**   | No        | No        | No        | Yes          | No        | No        | No          |
| **Jobs Stats**        | No        | No        | No        | Yes          | No        | No        | No          |
| **User Mgmt**         | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **RBAC/ACL Security** | No        | No        | No        | Yes          | No        | No        | No          |
| **Fork / Plugins**    | Yes       | Yes       | Yes       | Yes          | Yes       | No        | Yes         |
| **Multi-tenancy**     | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |


### Migrating from Jenkins
[Jenkins](jenkins.md)

### Migrating from Gitlab
 [Gitlab](gitlab.md)
 
### Migrating from Github Actions
 [Github](github.md)
 
### Migrating from CircleCI
 [CircleCI](circleci.md)
 
### Migrating from Airflow
 [Apache Airflow](airflow.md)
 
