-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_job_resource_uses (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      aborted BOOLEAN NOT NULL DEFAULT FALSE,
      value INTEGER NOT NULL DEFAULT 0,
      job_resource_id VARCHAR(36) NOT NULL,
      job_request_id INT(20) NOT NULL,
      task_execution_id VARCHAR(36) NOT NULL,
      user_id VARCHAR(36) NOT NULL DEFAULT "",
      expires_at TIMESTAMP,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_resource_uses_resource_fk FOREIGN KEY (job_resource_id) REFERENCES formicary_job_resources(id)
    );

    CREATE INDEX formicary_job_resource_uses_resource_ndx ON formicary_job_resource_uses(job_resource_id);
    CREATE INDEX formicary_job_resource_uses_request_ndx ON formicary_job_resource_uses(job_request_id);
    CREATE INDEX formicary_job_resource_uses_user_ndx ON formicary_job_resource_uses(user_id);
    CREATE UNIQUE INDEX formicary_job_resource_uses_request_task_ndx ON formicary_job_resource_uses(job_resource_id, job_request_id, task_execution_id);
    CREATE INDEX formicary_job_resource_uses_active_ndx ON formicary_job_resource_uses(active);
    CREATE INDEX formicary_job_resource_uses_aborted_ndx ON formicary_job_resource_uses(aborted);

-- +goose Down
    DROP TABLE IF EXISTS formicary_job_resource_uses;
