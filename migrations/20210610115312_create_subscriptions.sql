-- +goose Up
    CREATE TABLE IF NOT EXISTS formicary_subscriptions (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      kind VARCHAR(100) NOT NULL,
      policy VARCHAR(100) NOT NULL,
      period VARCHAR(100) NOT NULL,
      price INT(20) NOT NULL DEFAULT 0,
      cpu_quota INT(20) NOT NULL DEFAULT 0,
      disk_quota INT(20) NOT NULL DEFAULT 0,
      started_at TIMESTAMP,
      ended_at TIMESTAMP,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      updated_at TIMESTAMP,
      created_at TIMESTAMP DEFAULT NOW()
    );

    CREATE INDEX formicary_subscriptions_user_ndx ON formicary_subscriptions(id, user_id);
    CREATE INDEX formicary_subscriptions_org_ndx ON formicary_subscriptions(id, organization_id);
    CREATE INDEX formicary_subscriptions_active_ndx ON formicary_subscriptions(active);

    CREATE TABLE IF NOT EXISTS formicary_subscription_payments (
      id VARCHAR(36) NOT NULL PRIMARY KEY,
      subscription_id VARCHAR(36) NOT NULL,
      user_id VARCHAR(36),
      organization_id VARCHAR(36),
      external_id VARCHAR(200) NOT NULL,
      state VARCHAR(100) NOT NULL DEFAULT 'PENDING',
      kind VARCHAR(100) NOT NULL,
      payment_method VARCHAR(100) NOT NULL,
      notes TEXT NOT NULL,
      price INT(20) NOT NULL DEFAULT 0,
      cpu_quota INT(20) NOT NULL DEFAULT 0,
      refunded_at TIMESTAMP,
      started_at TIMESTAMP,
      ended_at TIMESTAMP,
      active BOOLEAN NOT NULL DEFAULT TRUE,
      updated_at TIMESTAMP,
      created_at TIMESTAMP DEFAULT NOW()
    );

    CREATE INDEX formicary_subscription_payments_user_ndx ON formicary_subscription_payments(id, user_id);
    CREATE INDEX formicary_subscription_payments_org_ndx ON formicary_subscription_payments(id, organization_id);
    CREATE INDEX formicary_subscription_payments_fk_ndx ON formicary_subscription_payments(id, subscription_id);
    CREATE INDEX formicary_subscription_payments_active_ndx ON formicary_subscription_payments(active);

-- +goose Down
    DROP TABLE IF EXISTS formicary_subscription_payments;
    DROP TABLE IF EXISTS formicary_subscriptions;
