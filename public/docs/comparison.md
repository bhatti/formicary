### Comparison with other Actions, DAG, Jobs, Pipeline or Workflow Solutions

| Feature               | Airflow  | Azure     | CircleCI  | Formicary    | GitHub    | Gitlab    | Jenkins     |
|-----------------------|-----------|-----------|-----------|--------------|-----------|-----------|-------------|
| **Tool Category**     | DAG       | CI-CD     | CD-CD     | DAG-Workflow | CI-CD     | CI-CD     | CI-CD       |
| **Docker Images**     | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Indirectly  |
| **Definition Format** | Python    | API       | YAML      | YAML         | YAML      | TOML      | DSL         |
| **Artifacts**         | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **NPM/Maven/etc Caching** | No        | Yes       | Yes       | Yes      | No        | Yes       | No          |
| **Dynamic Workflow**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Job Priority**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Task/Workers Tags** | Yes       | No        | No        | Yes          | No        | Yes       | Yes (Agents)|
| **Conditional Flow**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Cron/Scheduling**   | Yes       | Yes       | Yes       | Yes          | No        | Yes       | Yes         |
| **Job/Task API**      | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Limit CPU/Memory**  | Yes       | No        | No        | Yes          | No        | Yes       | No          |
| **Request Params**    | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Task Variables**    | Yes       | No        | Yes       | Yes          | NO        | Yes       | No          |
| **Job Config/Secrets**| Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Org Config/Secrets**| No        | No        | Yes       | Yes          | No        | No        | No          |
| **Encrypted Config**  | Yes/Fernet| No        | Yes       | Yes          | No        | No        | Yes         |
| **Template Support**  | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Shell Execution**   | Yes       | Yes       | No        | Yes          | No        | No        | Yes         |
| **HTTP Execution**    | via Python| via Script| via Script| Native Exec. | via Script| via Script| via Script  |
| **Queuing Library**   | Celery    | Internal  | Internal  | Redis, Kafka or Pulsar| Internal | Internal  | NA  |
| **Lifecycle Events**  | No        | No        | No        | Yes          | No        | No        | No          |
| **Job Retries**       | Yes       | Yes       | No        | Yes          | No        | Yes       | No          |
| **Job/Task Filtering** | No       | No        | No        | Yes          | No        | Yes       | No          |
| **Job Timeout**       | Yes       | Yes       | No        | Yes          | No        | Yes       | No          |
| **Task Retries**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Task Timeout**      | Yes       | No        | No        | Yes          | No        | No        | No          |
| **Optional Tasks**    | No        | No        | No        | Yes          | No        | Yes       | No          |
| **Always-run Tasks**  | No        | No        | No        | Yes          | No        | Yes       | No          |
| **Delay before Retry**| Yes       | No        | No        | Yes          | No        | No        | No          |
| **JWT Auth**          | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Email Notifications** | No      | Yes       | Yes       | Yes          | Yes       | Yes       | Yes         |
| **Slack Notifications** | No      | No        | No        | Yes          | No        | No        | No          |
| **Usage Reports**     | No       | No         | No        | Yes          | No        | No        | No          |
| **Jobs Stats**        | No       | No         | No        | Yes          | No        | No        | No          |
| **User Mgmt**         | Yes       | Yes       | Yes       | Yes          | Yes       | Yes       | No          |
| **Plugins**           | Yes       | Yes       | Yes       | Yes          | Yes       | No        | Yes         |
| **Multi-tenancy**     | No        | Yes       | Yes       | Yes          | Yes       | Yes       | No          |


### Migrating from Jenkins
[Jenkins](jenkins.md)

### Migrating from Gitlab
 [Gitlab](gitlab.md)
 
### Migrating from CircleCI
 [CircleCI](circleci.md)
 
