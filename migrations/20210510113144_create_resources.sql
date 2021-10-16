-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_job_resources (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      external_id VARCHAR(100) NOT NULL,
      valid_status BOOLEAN NOT NULL DEFAULT TRUE,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      disabled BOOLEAN NOT NULL DEFAULT FALSE,
      quota INTEGER NOT NULL DEFAULT 0,
      lease_timeout BIGINT NOT NULL,
      resource_type VARCHAR(100) NOT NULL,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      description TEXT,
      platform VARCHAR(100),
      category VARCHAR(100),
      tags_serialized TEXT,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
    );

    CREATE INDEX formicary_job_resources_type_ndx ON formicary_job_resources(resource_type);
    CREATE UNIQUE INDEX formicary_job_resources_external_platform_ndx ON formicary_job_resources(platform, external_id);
    CREATE INDEX formicary_job_resources_org_ndx ON formicary_job_resources(organization_id);
    CREATE INDEX formicary_job_resources_user_id_ndx ON formicary_job_resources(user_id);
    CREATE INDEX formicary_job_resources_platform_ndx ON formicary_job_resources(platform);
    CREATE INDEX formicary_job_resources_disabled_ndx ON formicary_job_resources(disabled);
    CREATE INDEX formicary_job_resources_active_ndx ON formicary_job_resources(active);

    CREATE TABLE IF NOT EXISTS formicary_job_resource_config (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      job_resource_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      `type` VARCHAR(50) NOT NULL,
      value LONGTEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_job_resource_config_resource_fk FOREIGN KEY (job_resource_id) REFERENCES formicary_job_resources(id)
    );

    CREATE INDEX formicary_job_resource_config_resource_ndx ON formicary_job_resource_config(job_resource_id);
    CREATE UNIQUE INDEX formicary_job_resource_config_id_name_ndx ON formicary_job_resource_config(job_resource_id, name);
-- +goose Down
    DROP TABLE IF EXISTS formicary_job_resource_config;
    DROP TABLE IF EXISTS formicary_job_resources;
