# Migrating from Apache Airflow

Apache Airflow is a popular solution for building, scheduling and monitoring workflows and following mapping shows mapping between Airflow and Formicary:

|     Airflow |   Formicary   | Description
| :----------: | :-----------: | :------------------: |
| python | yaml | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/index.html) uses Python to define DAG/Workflow whereas Formicary uses a YAML config for DAG/workflow definition (also referred as job definition).
| operator/hooks | [method](definition_options.md#method) | [Airflow](https://airflow.apache.org/docs/apache-airflow-providers/operators-and-hooks-ref/index.html) supports operators and hooks for integrating with 3rd party services and Formicary uses methods to extend protocols and integrations.
| executors | [executor](executors.md) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/executor/index.html) supports local and remote executors run tasks and Formicary uses similar executors to run various types of tasks.
| pools | [tags](definition_options.md#tags) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/_modules/airflow/models/pool.html#Pool) supports worker pools to run specific tasks and Formicary uses tags to annotate workers that can run specific tasks.
| schedule | [cron](definition_options.md#cron_trigger) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/concepts/scheduler.html?highlight=schedule_interval) uses schedule_interval to define scheduled tasks and Formicary uses `cron_trigger` syntax to define periodic or scheduled tasks.
| bash_command | [script](definition_options#script), [pre_script](definition_options.md#pre_script), [post_script](definition_options.md#post_script) | Airflow uses `bash_command` to define command to run whereas Formicary provides `pre_script`/`script`/`post_script` syntax to define list of commands to run before, during and after the task execution.
| sensor | [executing](sensor.md) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/_api/airflow/sensors/index.html) uses `sensor` such as `FileSensor` to poll external resources and Formicary uses `EXECUTING` state to define a polling task.
| params | [request params](definition_options.md#Params) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/tutorial.html#default-arguments) uses `default-arguments` and `params` to pass a dictionary of parameters and/or objects to your templates and Formicary uses request `params` and `variables` for similar purpose.
| templates | [templates](definition_options#templates) | [Airflow](https://airflow.apache.org/docs/apache-airflow/stable/tutorial.html#templating-with-jinja) uses jinja templates to define macros and templates whereas Formicary uses [GO templates](https://pkg.go.dev/text/template) to customize workflow dynamically.
| filters | [filter](definition_options#filter), [except](definition_options.md#except), [allow_failure](definition_options.md#allow_failure), [always_run](definition_options.md#always_run) and [templates](definition_options.md#templates) | Airflow uses `trigger_rule` such as all_success, all_failed, all_done to provide filtering for task execution and formicary provides a number of ways such as `filter`, `except`, `allow_failure`, `always_run` and [GO templates](https://pkg.go.dev/text/template) to filter or conditionally execute any task.
| Environment | [environment](definition_options.md#environment) | Airflow uses [environment variables](https://airflow.apache.org/docs/apache-airflow/stable/howto/variable.html#storing-variables-in-environment-variables) to use environment variables in task execution and uses [Fernet](https://github.com/fernet/spec/) to secure them whereas formicary supports environment or configuration options to set properties/variables before executing a task and supports secure storage of secret configuration..
| variables | [variables](definition_options.md#variables) | Airflow uses [variables](https://airflow.apache.org/docs/apache-airflow/stable/howto/variable.html) to pass variables to the tasks and a formicary provides similar support for `variables` at job and task level, which can be accessed by the executing task.
| control-flow | [on_exit](definition_options.md#on_exit) | Airflow uses [control-flow](https://airflow.apache.org/docs/apache-airflow/stable/concepts/overview.html#control-flow) to define dependency and control-flow between tasks whereas Formicary uses `on_exit`, `on_completed`, `on_failed` to define task dependencies in the workflow.

## Sample Airflow DAGs
Here is a sample dag of Airflow :
```
from datetime import datetime, timedelta
from textwrap import dedent
from airflow import DAG
from airflow.operators.bash import BashOperator

default_args = {
    'owner': 'airflow',
    'depends_on_past': False,
    'email': ['airflow@example.com'],
    'email_on_failure': False,
    'email_on_retry': False,
    'retries': 1,
    'retry_delay': timedelta(minutes=5),
}

with DAG(
    'tutorial',
    default_args=default_args,
    description='A simple tutorial DAG',
    schedule_interval=timedelta(days=1),
    start_date=datetime(2021, 1, 1),
    catchup=False,
    tags=['example'],
) as dag:
    t1 = BashOperator(
        task_id='print_date',
        bash_command='date',
    )

    t2 = BashOperator(
        task_id='sleep',
        depends_on_past=False,
        bash_command='sleep 5',
        retries=3,
    )
    templated_command = dedent(
        """
    {% for i in range(5) %}
        echo "{{ ds }}"
        echo "{{ macros.ds_add(ds, 7)}}"
        echo "{{ params.my_param }}"
    {% endfor %}
    """
    )

    t3 = BashOperator(
        task_id='templated',
        depends_on_past=False,
        bash_command=templated_command,
        params={'my_param': 'Parameter I passed in'},
    )
    t1 >> [t2, t3]
```

Following is equivalent workflow in formicary:
```
job_type: loop-job
tasks:
- task_type: t1
  container:
    image: alpine
  script:
    - date
  on_completed: t2
- task_type: t2
  container:
    image: alpine
  script:
    - sleep 5
  on_completed: t3
- task_type: t3
  container:
    image: alpine
  task_variables:
    my_param: Parameter I passed in
  script:
{{- range $val := Iterate 5 }}
    - echo {{$val}}
    - echo {{ Add $val 7}}
    - echo $my_param
{{ end  }}
```

## Limitations in Airflow
Following are major limitations of github actions:
 - Airflow supports limited support for caching of artifacts.
 - Airflow doesn't provide any metrics or queue size whereas formicary provides detailed reporting, metrics and insights into queue size.
 - Airflow provides limited support for partial restart and retries unlike formicary that provides a number of configuration parameters to recover from the failure.
 - Airflow provides limited support for optional and always-run tasks.
 - Airflow provides limited support for specifying cpu, memory and storage limits whereas formicary allows these limits when using Kubernetes executors. 
 - Airflow does not support priority of the jobs whereas formicary allows specifying priority of jobs for determining execution order of pending jobs.
 - Formicary provides more support for scheduling periodic or cron jobs.
 - Formicary provides rich support for metrics and reporting on usage on resources and statistics on job failure/success.
 - Formicary provides plugin APIs to share common workflows and jobs among users.
