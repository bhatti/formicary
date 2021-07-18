-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_artifacts (
        id CHAR(255) NOT NULL PRIMARY KEY,
        job_request_id INT(20) NOT NULL,
        job_execution_id CHAR(36) NOT NULL,
        task_execution_id CHAR(36) NOT NULL,
        task_type VARCHAR(100) DEFAULT '',
        sha256 CHAR(64) NOT NULL,
        bucket CHAR(64) NOT NULL,
        content_type CHAR(100) NOT NULL,
        content_length BIGINT NOT NULL,
        name CHAR(100) NOT NULL,
        kind CHAR(50) NOT NULL,
        e_tag CHAR(50) DEFAULT '' NOT NULL,
        `group` CHAR(50) NOT NULL,
        user_id CHAR(36),
        organization_id VARCHAR(36),
        permissions INTEGER DEFAULT 0,
        artifact_order INTEGER DEFAULT 0,
        metadata_serialized LONGTEXT,
        tags_serialized LONGTEXT,
        expires_at TIMESTAMP,
        active BOOLEAN NOT NULL DEFAULT TRUE,
        created_at TIMESTAMP NOT NULL DEFAULT now(),
        updated_at TIMESTAMP DEFAULT NOW()
    ) ;

    CREATE INDEX formicary_artifacts_user_id_ndx ON formicary_artifacts(user_id);
    CREATE INDEX formicary_artifacts_org_ndx ON formicary_artifacts(organization_id);
    CREATE INDEX formicary_artifacts_kind_ndx ON formicary_artifacts(kind);
    CREATE INDEX formicary_artifacts_job_request_id_ndx ON formicary_artifacts(job_request_id);
    CREATE INDEX formicary_artifacts_job_execution_id_ndx ON formicary_artifacts(job_execution_id);
    CREATE INDEX formicary_artifacts_task_execution_id_ndx ON formicary_artifacts(task_execution_id);
    CREATE INDEX formicary_artifacts_task_type_ndx ON formicary_artifacts(task_type);
    CREATE INDEX formicary_artifacts_created_ndx ON formicary_artifacts(created_at);
    CREATE INDEX formicary_artifacts_active_ndx ON formicary_artifacts(active);
    CREATE INDEX formicary_artifacts_sha256_ndx ON formicary_artifacts(sha256);

-- +goose Down
    DROP TABLE IF EXISTS formicary_artifacts;
