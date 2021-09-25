-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_orgs (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      parent_id VARCHAR(36),
      owner_user_id VARCHAR(36),
      active BOOLEAN NOT NULL DEFAULT TRUE,
      org_unit VARCHAR(100) NOT NULL,
      bundle_id VARCHAR(100) NOT NULL,
      salt VARCHAR(64) NOT NULL DEFAULT '',
      max_concurrency INTEGER NOT NULL DEFAULT 1,
      license_policy VARCHAR(100),
      sticky_message VARCHAR(200),
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
    );

    CREATE UNIQUE INDEX formicary_orgs_org_ndx ON formicary_orgs(org_unit);
    CREATE UNIQUE INDEX formicary_orgs_bundle_ndx ON formicary_orgs(bundle_id);
    CREATE INDEX formicary_orgs_active ON formicary_orgs(active);
    INSERT INTO `formicary_orgs` (id, org_unit, bundle_id) VALUES ('00000000-0000-0000-0000-000000000000', 'formicary', 'io.formicary');
    INSERT INTO `formicary_orgs` (id, org_unit, bundle_id) VALUES ('00000000-0000-0000-0000-000000000001', 'plexobject', 'com.plexobject');

    CREATE TABLE IF NOT EXISTS formicary_org_configs (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      organization_id VARCHAR(36) NOT NULL,
      name VARCHAR(100) NOT NULL,
      `type` VARCHAR(50) NOT NULL,
      value LONGTEXT NOT NULL,
      secret BOOLEAN NOT NULL DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_org_configs_job_fk FOREIGN KEY (organization_id) REFERENCES formicary_orgs(id)
    );

    CREATE INDEX formicary_org_configs_job_ndx ON formicary_org_configs(organization_id);
    CREATE UNIQUE INDEX formicary_org_configs_id_name_ndx ON formicary_org_configs(organization_id, name);

    CREATE TABLE IF NOT EXISTS formicary_users (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      locked BOOLEAN NOT NULL DEFAULT FALSE,
      max_concurrency INTEGER NOT NULL DEFAULT 1,
      sticky_message VARCHAR(200),
      email_verified BOOLEAN NOT NULL DEFAULT FALSE,
      organization_id VARCHAR(36),
      username VARCHAR(100) NOT NULL,
      bundle_id VARCHAR(100),
      salt VARCHAR(64) NOT NULL DEFAULT '',
      name VARCHAR(100) ,
      email VARCHAR(150),
      notify_serialized LONGTEXT,
      serialized_perms TEXT NOT NULL,
      serialized_roles TEXT NOT NULL,
      picture_url VARCHAR(150),
      url VARCHAR(150),
      auth_provider VARCHAR(50),
      auth_id VARCHAR(50),
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
    );

    CREATE UNIQUE INDEX formicary_users_username_ndx ON formicary_users(username);
    CREATE UNIQUE INDEX formicary_users_email_ndx ON formicary_users(email);
    CREATE INDEX formicary_users_provider_ndx ON formicary_users(auth_provider);
    CREATE INDEX formicary_users_org_ndx ON formicary_users(organization_id);
    CREATE INDEX formicary_users_active_ndx ON formicary_users(active);
    INSERT INTO `formicary_users` (id, organization_id, username, serialized_perms, serialized_roles) VALUES ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000', 'admin', '*=-1', 'Admin[]');
    INSERT INTO `formicary_users` (id, organization_id, username, serialized_perms, serialized_roles) VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'bhatti', '*=-1', '');

    CREATE TABLE IF NOT EXISTS formicary_user_sessions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      user_id VARCHAR(36) NOT NULL,
      email VARCHAR(100) NOT NULL,
      username VARCHAR(100) NOT NULL,
      session_id VARCHAR(100) NOT NULL,
      ip_address VARCHAR(40) NOT NULL,
      picture_url TEXT,
      auth_provider VARCHAR(50),
      data LONGTEXT NOT NULL,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_user_sessions_fk FOREIGN KEY (user_id) REFERENCES formicary_users(id)
    );

    CREATE TABLE IF NOT EXISTS formicary_user_tokens (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      user_id VARCHAR(36) NOT NULL,
      organization_id VARCHAR(36) NOT NULL,
      token_name VARCHAR(100) NOT NULL,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      sha256 VARCHAR(64) NOT NULL,
      expires_at TIMESTAMP NULL DEFAULT NULL,
      created_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_user_tokens_fk FOREIGN KEY (user_id) REFERENCES formicary_users(id)
    );

    CREATE TABLE IF NOT EXISTS formicary_user_invitations (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      email VARCHAR(100) NOT NULL,
      organization_id VARCHAR(36) NOT NULL,
      org_unit VARCHAR(100) NOT NULL,
      invited_by_user_id VARCHAR(36) NOT NULL,
      invitation_code VARCHAR(50) NOT NULL,
      accepted_at TIMESTAMP NULL DEFAULT NULL,
      expires_at TIMESTAMP NULL DEFAULT NULL,
      created_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_user_invitation_fk FOREIGN KEY (invited_by_user_id) REFERENCES formicary_users(id)
    );
    CREATE UNIQUE INDEX formicary_user_invitations_code_ndx ON formicary_user_invitations(invitation_code);

-- +goose Down
    DROP TABLE IF EXISTS formicary_user_invitations;
    DROP TABLE IF EXISTS formicary_user_sessions;
    DROP TABLE IF EXISTS formicary_user_tokens;
    DROP TABLE IF EXISTS formicary_users;
    DROP TABLE IF EXISTS formicary_org_configs;
    DROP TABLE IF EXISTS formicary_orgs;
