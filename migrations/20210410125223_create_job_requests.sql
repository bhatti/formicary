-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_job_requests (
      id INT(20) NOT NULL AUTO_INCREMENT PRIMARY KEY,
      job_definition_id VARCHAR(36) NOT NULL,
      user_key VARCHAR(100),
      parent_id VARCHAR(36),
      job_execution_id VARCHAR(36),
      last_job_execution_id VARCHAR(36),
      cron_triggered BOOLEAN NOT NULL DEFAULT FALSE,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      permissions INTEGER NOT NULL DEFAULT 0,
      schedule_attempts INTEGER NOT NULL DEFAULT 0,
      description TEXT,
      platform VARCHAR(100),
      job_type VARCHAR(100) NOT NULL,
      job_version VARCHAR(50) NOT NULL DEFAULT '',
      job_state VARCHAR(100) NOT NULL DEFAULT 'PENDING',
      job_group VARCHAR(100),
      job_priority INTEGER NOT NULL DEFAULT 1,
      timeout BIGINT NOT NULL DEFAULT 0,
      retried INTEGER NOT NULL DEFAULT 0,
      quick_search LONGTEXT,
      error_code VARCHAR(100),
      error_message LONGTEXT,
      scheduled_at TIMESTAMP DEFAULT NOW(),
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP NULL,
      CONSTRAINT formicary_job_requests_job_def_fk FOREIGN KEY (job_definition_id) REFERENCES formicary_job_definitions(id)
    );

    CREATE INDEX formicary_job_parent_id_ndx ON formicary_job_requests(parent_id);
    CREATE INDEX formicary_job_requests_org_ndx ON formicary_job_requests(organization_id);
    CREATE INDEX formicary_job_requests_user_id_ndx ON formicary_job_requests(user_id);
    CREATE INDEX formicary_job_requests_job_def_ndx ON formicary_job_requests(job_definition_id);
    CREATE INDEX formicary_job_requests_pkg_job_type_ndx ON formicary_job_requests(job_type, job_version);
    CREATE INDEX formicary_job_requests_group_ndx ON formicary_job_requests(job_group);
    CREATE INDEX formicary_job_requests_platform_ndx ON formicary_job_requests(platform);
    CREATE INDEX formicary_job_requests_state_ndx ON formicary_job_requests(job_state);
    CREATE INDEX formicary_job_requests_state_pri_ndx ON formicary_job_requests(job_state, job_priority);
    CREATE UNIQUE INDEX formicary_job_user_key_ndx ON formicary_job_requests(user_key);
    CREATE INDEX formicary_job_request_updated_ndx ON formicary_job_requests(updated_at);
    CREATE INDEX formicary_job_request_context_ndx ON formicary_job_requests(quick_search(512));
    CREATE INDEX formicary_job_request_errcode_ndx ON formicary_job_requests(error_code);
    CREATE INDEX formicary_job_requests_type_state_ndx ON formicary_job_requests(job_type, job_version, job_state);

    CREATE TABLE IF NOT EXISTS formicary_job_request_params (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_request_id INT(20) NOT NULL,
      name VARCHAR(100) NOT NULL,
      `type` VARCHAR(50) NOT NULL,
      value LONGTEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_request_params_job_fk FOREIGN KEY (job_request_id) REFERENCES formicary_job_requests(id)
    );

    CREATE INDEX formicary_job_request_params_request_ndx ON formicary_job_request_params(job_request_id);
    CREATE UNIQUE INDEX formicary_job_request_params_name_ndx ON formicary_job_request_params(job_request_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_job_request_params;
    DROP TABLE IF EXISTS formicary_job_requests;
