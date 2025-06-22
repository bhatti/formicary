-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_job_definitions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_type VARCHAR(100) NOT NULL,
      version INTEGER NOT NULL DEFAULT 0,
      sem_version VARCHAR(50),
      description TEXT,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      disabled BOOLEAN NOT NULL DEFAULT FALSE,
      platform VARCHAR(100),
      cron_trigger VARCHAR(50),
      timeout BIGINT NOT NULL DEFAULT 0,
      uses_template BOOLEAN NOT NULL DEFAULT FALSE,
      public_plugin BOOLEAN NOT NULL DEFAULT FALSE,
      hard_reset_after_retries INTEGER NOT NULL DEFAULT 3,
      retry INTEGER NOT NULL DEFAULT 0,
      delay_between_retries BIGINT NOT NULL DEFAULT 10,
      pause_time BIGINT NOT NULL DEFAULT 10,
      max_concurrency INTEGER NOT NULL DEFAULT 1,
      url VARCHAR(150),
      notify_serialized TEXT,
      tags TEXT,
      methods TEXT,
      raw_yaml TEXT, -- CHARACTER SET utf8 COLLATE utf8_unicode_ci,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
    );

    CREATE UNIQUE INDEX formicary_job_definitions_type_version_ndx ON formicary_job_definitions(user_id, job_type, version);
    CREATE INDEX formicary_job_definition_org_ndx ON formicary_job_definitions(organization_id);
    CREATE INDEX formicary_job_definition_user_id_ndx ON formicary_job_definitions(user_id);
    CREATE INDEX formicary_job_definitions_platform_ndx ON formicary_job_definitions(platform);
    CREATE INDEX formicary_job_definitions_disabled_ndx ON formicary_job_definitions(disabled);
    CREATE INDEX formicary_job_definitions_sem_version_ndx ON formicary_job_definitions(job_type, sem_version);
    CREATE INDEX formicary_job_definitions_active_ndx ON formicary_job_definitions(active);

    CREATE TABLE IF NOT EXISTS formicary_job_definition_variables (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_definition_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      kind VARCHAR(50) NOT NULL,
      value TEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_definition_variables_job_fk FOREIGN KEY (job_definition_id) REFERENCES formicary_job_definitions(id)
    );

    CREATE INDEX formicary_job_definition_variables_job_ndx ON formicary_job_definition_variables(job_definition_id);
    CREATE UNIQUE INDEX formicary_job_definition_variables_id_name_ndx ON formicary_job_definition_variables(job_definition_id, name);

    CREATE TABLE IF NOT EXISTS formicary_job_definition_configs (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_definition_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      kind VARCHAR(50) NOT NULL,
      value TEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_definition_configs_job_fk FOREIGN KEY (job_definition_id) REFERENCES formicary_job_definitions(id)
    );

    CREATE INDEX formicary_job_definition_configs_job_ndx ON formicary_job_definition_configs(job_definition_id);
    CREATE UNIQUE INDEX formicary_job_definition_configs_id_name_ndx ON formicary_job_definition_configs(job_definition_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_job_definition_variables;
    DROP TABLE IF EXISTS formicary_job_definition_configs;
    DROP TABLE IF EXISTS formicary_job_definitions;
