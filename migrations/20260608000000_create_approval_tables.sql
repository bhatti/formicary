-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_approval_policies (
      id                    VARCHAR(36) NOT NULL PRIMARY KEY,
      task_definition_id    VARCHAR(36) NOT NULL,
      min_approvals         INTEGER NOT NULL DEFAULT 1,
      allowed_roles         TEXT NOT NULL DEFAULT '',
      allowed_users         TEXT NOT NULL DEFAULT '',
      require_unanimous     BOOLEAN NOT NULL DEFAULT FALSE,
      sla_deadline_ns       BIGINT NOT NULL DEFAULT 0,
      timeout_action        VARCHAR(32) NOT NULL DEFAULT 'ESCALATE',
      escalation_recipients TEXT NOT NULL DEFAULT '',
      escalation_message    TEXT NOT NULL DEFAULT '',
      created_at            TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at            TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      CONSTRAINT fk_approval_policy_task_def
        FOREIGN KEY (task_definition_id) REFERENCES formicary_task_definitions(id)
        ON DELETE CASCADE
    );

    CREATE UNIQUE INDEX formicary_approval_policies_task_ndx
      ON formicary_approval_policies(task_definition_id);

    CREATE TABLE IF NOT EXISTS formicary_approval_votes (
      id                VARCHAR(36) NOT NULL PRIMARY KEY,
      task_execution_id VARCHAR(36) NOT NULL,
      job_request_id    VARCHAR(36) NOT NULL,
      voter_id          VARCHAR(128) NOT NULL,
      voter_name        VARCHAR(255) NOT NULL DEFAULT '',
      decision          VARCHAR(32) NOT NULL,
      comments          TEXT NOT NULL DEFAULT '',
      voted_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
      CONSTRAINT fk_approval_vote_task_exec
        FOREIGN KEY (task_execution_id) REFERENCES formicary_task_executions(id)
        ON DELETE CASCADE,
      CONSTRAINT uq_approval_vote_voter_task
        UNIQUE (task_execution_id, voter_id)
    );

    CREATE INDEX formicary_approval_votes_request_ndx
      ON formicary_approval_votes(job_request_id);
    CREATE INDEX formicary_approval_votes_task_exec_ndx
      ON formicary_approval_votes(task_execution_id);

    CREATE TABLE IF NOT EXISTS formicary_approval_deadlines (
      id                    VARCHAR(36) NOT NULL PRIMARY KEY,
      task_execution_id     VARCHAR(36) NOT NULL,
      job_request_id        VARCHAR(36) NOT NULL,
      deadline              TIMESTAMP NOT NULL,
      escalated             BOOLEAN NOT NULL DEFAULT FALSE,
      resolved              BOOLEAN NOT NULL DEFAULT FALSE,
      timeout_action        VARCHAR(32) NOT NULL DEFAULT 'ESCALATE',
      escalation_recipients TEXT NOT NULL DEFAULT '',
      created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
      CONSTRAINT fk_approval_deadline_task_exec
        FOREIGN KEY (task_execution_id) REFERENCES formicary_task_executions(id)
        ON DELETE CASCADE
    );

    CREATE INDEX formicary_approval_deadlines_pending_ndx
      ON formicary_approval_deadlines(resolved, deadline);

-- +goose Down
    DROP TABLE IF EXISTS formicary_approval_deadlines;
    DROP TABLE IF EXISTS formicary_approval_votes;
    DROP TABLE IF EXISTS formicary_approval_policies;
