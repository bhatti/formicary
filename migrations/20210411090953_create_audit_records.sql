-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_audit_records (
        id CHAR(36) NOT NULL PRIMARY KEY,
        user_id CHAR(36),
        organization_id VARCHAR(36),
        kind CHAR(50) NOT NULL,
        target_id VARCHAR(36),
        job_type CHAR(100),
        remote_ip CHAR(100) NOT NULL,
        url TEXT,
        error TEXT,
        message LONGTEXT,

        created_at TIMESTAMP NOT NULL DEFAULT now()
    ) ;

    CREATE INDEX formicary_audit_records_user_id_ndx ON formicary_audit_records(user_id);
    CREATE INDEX formicary_audit_records_org_id_ndx ON formicary_audit_records(organization_id);
    CREATE INDEX formicary_audit_records_kind_ndx ON formicary_audit_records(kind);
    CREATE INDEX formicary_audit_records_created_ndx ON formicary_audit_records(created_at);

-- +goose Down
    DROP TABLE IF EXISTS formicary_audit_records;
