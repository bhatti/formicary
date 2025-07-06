-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_task_executions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_execution_id VARCHAR(36) NOT NULL,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      ant_id VARCHAR(100) ,
      ant_host VARCHAR(100) ,
      task_type VARCHAR(100) NOT NULL,
      method VARCHAR(100) NOT NULL,
      task_state VARCHAR(100) NOT NULL DEFAULT 'PENDING',
      allow_failure BOOLEAN NOT NULL DEFAULT FALSE,
      exit_code VARCHAR(100) ,
      exit_message TEXT,
      error_code VARCHAR(100),
      error_message TEXT,
      failed_command TEXT,
      comments TEXT,
      task_order INTEGER DEFAULT 0,
      count_services INTEGER DEFAULT 0,
      retried INTEGER NOT NULL DEFAULT 0,
      cost_factor DECIMAL(10, 2) DEFAULT 0,
      manual_approved_by VARCHAR(100),
      manual_approved_at TIMESTAMP,
      started_at TIMESTAMP,
      ended_at TIMESTAMP,
      updated_at TIMESTAMP,
      created_at TIMESTAMP,
      CONSTRAINT formicary_task_executions_job_fk FOREIGN KEY (job_execution_id) REFERENCES formicary_job_executions(id)
    );

    CREATE INDEX formicary_task_executions_job_ndx ON formicary_task_executions(job_execution_id);
    CREATE INDEX formicary_task_executions_job_task_ndx ON formicary_task_executions(job_execution_id, task_type);
    CREATE INDEX formicary_task_executions_active_ndx ON formicary_task_executions(active);


    CREATE TABLE IF NOT EXISTS formicary_task_execution_context (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      task_execution_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      kind VARCHAR(50) NOT NULL,
      value TEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_task_execution_context_task_fk FOREIGN KEY (task_execution_id) REFERENCES formicary_task_executions(id)
    );

    CREATE INDEX formicary_task_execution_context_task_exec_ndx ON formicary_task_execution_context(task_execution_id);
    CREATE UNIQUE INDEX formicary_task_execution_context_name_ndx ON formicary_task_execution_context(task_execution_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_task_execution_context;
    DROP TABLE IF EXISTS formicary_task_executions;
