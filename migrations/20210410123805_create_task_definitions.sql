-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_task_definitions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_definition_id VARCHAR(36) NOT NULL,
      task_type VARCHAR(100) NOT NULL,
      method VARCHAR(100) NOT NULL,
      description TEXT,
      endpoint TEXT,
      always_run BOOLEAN NOT NULL DEFAULT FALSE,
      allow_failure BOOLEAN NOT NULL DEFAULT FALSE,
      allow_start_if_completed BOOLEAN NOT NULL DEFAULT FALSE,
      timeout INT(15) NOT NULL DEFAULT 0,
      retry_timeout INT(15) NOT NULL DEFAULT 0,
      retry INTEGER NOT NULL DEFAULT 1,
      delay_between_retries INT(15) NOT NULL DEFAULT 10,
      on_exit_code_serialized TEXT,
      on_completed VARCHAR(100),
      on_failed VARCHAR(100),
      task_order INTEGER NOT NULL DEFAULT 0,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_task_definitions_job_fk FOREIGN KEY (job_definition_id) REFERENCES formicary_job_definitions(id)
    );

    CREATE INDEX formicary_task_definitions_job_ndx ON formicary_task_definitions(job_definition_id);
    CREATE UNIQUE INDEX formicary_task_definitions_type_ndx ON formicary_task_definitions(job_definition_id, task_type);

    CREATE TABLE IF NOT EXISTS formicary_task_definition_variables (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      task_definition_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      `type` VARCHAR(50) NOT NULL,
      value LONGTEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_task_definition_variables_task_fk FOREIGN KEY (task_definition_id) REFERENCES formicary_task_definitions(id)
    );

    CREATE INDEX formicary_task_definition_variables_task_ndx ON formicary_task_definition_variables(task_definition_id);
    CREATE UNIQUE INDEX formicary_task_definition_variables_id_name_ndx ON formicary_task_definition_variables(task_definition_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_task_definition_variables;
    DROP TABLE IF EXISTS formicary_task_definitions;
