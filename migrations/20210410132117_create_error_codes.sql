-- +goose Up
CREATE TABLE IF NOT EXISTS formicary_error_codes
(
    id              VARCHAR(36)  NOT NULL PRIMARY KEY,
    regex           VARCHAR(255) NOT NULL DEFAULT '',
    exit_code       INT                   DEFAULT 0,
    error_code      VARCHAR(100) NOT NULL,
    description     TEXT,
    display_message TEXT,
    display_code    TEXT,
    platform_scope  VARCHAR(100),
    job_type        VARCHAR(100) NOT NULL,
    task_type_scope VARCHAR(50),
    action          VARCHAR(50),
    hard_failure    BOOLEAN      NOT NULL DEFAULT FALSE,
    retry           INT                   DEFAULT 0,
    created_at      TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX formicary_error_codes_regex_exit_ndx ON formicary_error_codes (job_type, regex, exit_code);
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000000', '*', 'job timed out', 'ERR_JOB_TIMEOUT');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000001', '*', 'task timed out', 'ERR_TASK_TIMEOUT');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000002', '*', 'failed to schedule job', 'ERR_JOB_SCHEDULE');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000003', '*', 'failed to launch job', 'ERR_JOB_LAUNCH');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000004', '*', 'failed to execute job', 'ERR_JOB_EXECUTE');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000005', '*', 'failed to cancel job', 'ERR_JOB_CANCELLED');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000006', '*', 'ant workers unavailable', 'ERR_ANTS_UNAVAILABLE');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000007', '*', 'failed to execute task', 'ERR_TASK_EXECUTE');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000008', '*', 'failed to find next task', 'ERR_INVALID_NEXT_TASK');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000009', '*', 'failed to find container', 'ERR_CONTAINER_NOT_FOUND');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000010', '*', 'failed to stop container', 'ERR_CONTAINER_STOPPED_FAILED');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000011', '*', 'failed to execute task by ant worker',
        'ERR_ANT_EXECUTION_FAILED');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000012', '*', 'failed to marshal object', 'ERR_MARSHALING_FAILED');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000013', '*', 'restart job', 'ERR_RESTART_JOB');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000014', '*', 'restart task', 'ERR_RESTART_TASK');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000015', '*', 'filtered scheduled job', 'ERR_FILTERED_JOB');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000016', '*', 'validation error', 'ERR_VALIDATION');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000017', '*', 'ant resources not avaialble', 'ERR_ANT_RESOURCES');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000018', '*', 'fatal error', 'ERR_FATAL');
INSERT INTO `formicary_error_codes` (id, job_type, regex, error_code)
VALUES ('00000000-0000-0000-0000-000000000019', '*', 'resource quota exceeded', 'ERR_QUOTA_EXCEEDED');
-- +goose Down
DROP TABLE IF EXISTS formicary_error_codes;
