-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_system_config (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      scope VARCHAR(100) NOT NULL,
      kind VARCHAR(100) NOT NULL,
      name VARCHAR(100) NOT NULL,
      value LONGTEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
    );

    CREATE UNIQUE INDEX formicary_system_config_scope_kind_name_ndx ON formicary_system_config(scope, kind, name);
    CREATE INDEX formicary_system_config_scope_ndx ON formicary_system_config(scope);
-- +goose Down
    DROP TABLE IF EXISTS formicary_system_config;
