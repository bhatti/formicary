-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_log_events (
        id CHAR(36) NOT NULL PRIMARY KEY,
        source CHAR(50),
        event_type CHAR(50),
        user_id CHAR(36),
        job_request_id INT(20),
        job_type VARCHAR(100) ,
        task_type VARCHAR(100),
        job_execution_id VARCHAR(36),
        task_execution_id VARCHAR(36),
        ant_id VARCHAR(100),
        encoded_message LONGTEXT,

        created_at TIMESTAMP NOT NULL DEFAULT now()
    ) ;

    CREATE INDEX formicary_log_events_user_id_ndx ON formicary_log_events(user_id);
    CREATE INDEX formicary_log_events_job_request_id_ndx ON formicary_log_events(job_request_id);
    CREATE INDEX formicary_log_events_job_execution_id_ndx ON formicary_log_events(job_execution_id);
    CREATE INDEX formicary_log_events_task_execution_id_ndx ON formicary_log_events(task_execution_id);
    CREATE INDEX formicary_log_events_created_ndx ON formicary_log_events(created_at);

-- +goose Down
    DROP TABLE IF EXISTS formicary_log_events;
