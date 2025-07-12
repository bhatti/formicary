-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_job_executions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_request_id VARCHAR(36) NOT NULL,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      job_type VARCHAR(100) NOT NULL,
      job_version VARCHAR(50) NOT NULL DEFAULT '',
      job_state VARCHAR(100) NOT NULL DEFAULT 'PENDING',
      cpu_secs INTEGER NOT NULL DEFAULT 0,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      current_task VARCHAR(200),
      exit_code VARCHAR(100) ,
      exit_message TEXT,
      error_code TEXT,
      error_message TEXT,
      started_at TIMESTAMP,
      ended_at TIMESTAMP,
      updated_at TIMESTAMP,
      created_at TIMESTAMP,
      CONSTRAINT formicary_job_executions_request_fk FOREIGN KEY (job_request_id) REFERENCES formicary_job_requests(id)
    );

    CREATE UNIQUE INDEX formicary_job_executions_request_ndx ON formicary_job_executions(id, job_request_id);
    CREATE INDEX formicary_job_executions_active_ndx ON formicary_job_executions(active);
    CREATE INDEX formicary_job_executions_pkg_job_type_ndx ON formicary_job_executions(job_type, job_version);
    CREATE INDEX formicary_job_executions_org_ndx ON formicary_job_executions(organization_id);
    CREATE INDEX formicary_job_executions_user_id_ndx ON formicary_job_executions(user_id);

    CREATE TABLE IF NOT EXISTS formicary_job_execution_context (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_execution_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      kind VARCHAR(50) NOT NULL,
      value TEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_execution_context_job_fk FOREIGN KEY (job_execution_id) REFERENCES formicary_job_executions(id)
    );

    CREATE INDEX formicary_job_execution_context_job_exec_ndx ON formicary_job_execution_context(job_execution_id);
    CREATE UNIQUE INDEX formicary_job_execution_context_name_ndx ON formicary_job_execution_context(job_execution_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_job_execution_context;
    DROP TABLE IF EXISTS formicary_job_executions;
