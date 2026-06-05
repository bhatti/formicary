-- +goose Up
-- formicary_trigger_states stores per-trigger runtime state: S3 poll markers and
-- rate-limit windows. Trigger definitions live in raw_yaml (transient); only
-- state that must survive restarts needs a row here.
CREATE TABLE IF NOT EXISTS formicary_trigger_states (
    -- 26-char ULID string
    id                VARCHAR(128) NOT NULL PRIMARY KEY,
    job_definition_id VARCHAR(128) NOT NULL,
    trigger_name      VARCHAR(255) NOT NULL,
    -- S3 poll cursor: last object key successfully processed
    last_seen_key     TEXT         NOT NULL DEFAULT '',
    last_seen_time    TIMESTAMP    NULL,
    -- Rate-limit window tracking
    window_start      TIMESTAMP    NULL,
    window_count      INTEGER      NOT NULL DEFAULT 0,
    created_at        TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_trigger_states_job_def
        FOREIGN KEY (job_definition_id) REFERENCES formicary_job_definitions(id)
        ON DELETE CASCADE,
    -- One state row per (job, trigger) pair
    CONSTRAINT uq_trigger_states_job_name
        UNIQUE (job_definition_id, trigger_name)
);

-- Index on job_definition_id for FindByJobDefinitionID queries
CREATE INDEX formicary_trigger_states_job_ndx
    ON formicary_trigger_states(job_definition_id);

-- +goose Down
DROP TABLE IF EXISTS formicary_trigger_states;
