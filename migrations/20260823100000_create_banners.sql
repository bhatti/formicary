-- +goose Up
CREATE TABLE IF NOT EXISTS formicary_banners (
    id          VARCHAR(36)   NOT NULL PRIMARY KEY,
    key         VARCHAR(255)  NOT NULL DEFAULT '',
    level       VARCHAR(20)   NOT NULL DEFAULT 'warning',
    scope       VARCHAR(10)   NOT NULL DEFAULT 'global',
    org_id      VARCHAR(36)   NOT NULL DEFAULT '',
    source      VARCHAR(20)   NOT NULL DEFAULT 'admin',
    message     TEXT          NOT NULL,
    active      BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_formicary_banners_scope_org ON formicary_banners(scope, org_id, active);
CREATE UNIQUE INDEX IF NOT EXISTS idx_formicary_banners_key ON formicary_banners(key) WHERE key != '';

-- +goose Down
DROP TABLE IF EXISTS formicary_banners;
