-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_email_verifications (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      email VARCHAR(100) NOT NULL,
      user_id VARCHAR(36) NOT NULL,
      organization_id VARCHAR(36) NULL,
      email_code VARCHAR(50) NOT NULL,
      verified_at TIMESTAMP NULL DEFAULT NULL,
      expires_at TIMESTAMP NULL,
      created_at TIMESTAMP DEFAULT NOW(),
      CONSTRAINT formicary_email_verifications_fk FOREIGN KEY (user_id) REFERENCES formicary_users(id)
    );

    CREATE INDEX formicary_email_verifications_email_ndx ON formicary_email_verifications(email);
    CREATE INDEX formicary_email_verifications_orgl_ndx ON formicary_email_verifications(organization_id);
    CREATE UNIQUE INDEX formicary_email_verifications_code_ndx ON formicary_email_verifications(email_code);

-- +goose Down
    DROP TABLE IF EXISTS formicary_email_verifications;
