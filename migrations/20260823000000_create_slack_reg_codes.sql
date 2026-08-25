-- +goose Up
CREATE TABLE IF NOT EXISTS formicary_slack_reg_codes (
    code          VARCHAR(64)  NOT NULL PRIMARY KEY,
    user_id       VARCHAR(36)  NOT NULL,
    org_id        VARCHAR(36)  NOT NULL,
    expires_at    DATETIME     NOT NULL,
    used          BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_slack_reg_codes_user ON formicary_slack_reg_codes(user_id);

-- +goose Down
DROP TABLE IF EXISTS formicary_slack_reg_codes;
