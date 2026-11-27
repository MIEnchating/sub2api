-- Recover routes on databases that already ran 404 before the proxy pool
-- was restored. Require the current account row to still be the version
-- written by 404, so later account edits and deliberate stops win.
CREATE TABLE IF NOT EXISTS retired_account_proxy_bindings (
    account_id BIGINT NOT NULL,
    proxy_id BIGINT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, proxy_id)
);

LOCK TABLE accounts, account_proxies IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE recover_account_proxy_routes ON COMMIT DROP AS
SELECT a.id, CASE WHEN jsonb_typeof(a.extra -> 'retired_request_proxy') = 'object'
    THEN a.extra -> 'retired_request_proxy' ELSE '{}'::jsonb END AS request_config
FROM accounts a
JOIN schema_migrations m ON m.filename = '404_retire_vip_credit_and_account_multi_proxy.sql'
    AND a.xmin = m.xmin
WHERE a.deleted_at IS NULL
    AND a.parent_account_id IS NULL
    AND a.extra ->> 'openai_vm_bound' IS DISTINCT FROM 'true'
    AND a.extra ->> 'openai_vm_retired' IS DISTINCT FROM 'true'
    AND a.extra ->> 'openai_ipv6_bound' IS DISTINCT FROM 'true'
    AND a.extra ->> 'ovh_ipv6_bound' IS DISTINCT FROM 'true'
    AND NOT EXISTS (SELECT 1 FROM openai_vm_bindings WHERE account_id = a.id OR managed_proxy_id = a.proxy_id)
    AND NOT EXISTS (SELECT 1 FROM openai_ipv6_bindings WHERE account_id = a.id OR managed_proxy_id = a.proxy_id)
    AND NOT EXISTS (SELECT 1 FROM ovh_ipv6_leases WHERE owner_account_id = a.id OR proxy_id = a.proxy_id)
    AND NOT EXISTS (SELECT 1 FROM account_proxies WHERE account_id = a.id)
    AND (jsonb_typeof(a.extra -> 'retired_request_proxy') = 'object'
        OR EXISTS (SELECT 1 FROM retired_account_proxy_bindings WHERE account_id = a.id));

INSERT INTO account_proxies (account_id, proxy_id, position, created_at)
SELECT archived.account_id, archived.proxy_id, archived.position, archived.created_at
FROM retired_account_proxy_bindings archived
JOIN recover_account_proxy_routes a ON a.id = archived.account_id
JOIN proxies p ON p.id = archived.proxy_id AND p.deleted_at IS NULL
WHERE NOT EXISTS (SELECT 1 FROM openai_vm_bindings WHERE managed_proxy_id = p.id)
    AND NOT EXISTS (SELECT 1 FROM openai_ipv6_bindings WHERE managed_proxy_id = p.id)
    AND NOT EXISTS (SELECT 1 FROM ovh_ipv6_leases WHERE proxy_id = p.id)
ON CONFLICT (account_id, proxy_id) DO NOTHING;

-- 404 did not record the old schedulable flag or fixed proxy. Preserve both
-- current values. An account stopped by 404 needs an explicit resumption.
WITH changed AS (
    UPDATE accounts a
    SET extra = COALESCE(a.extra, '{}'::jsonb) || r.request_config,
        updated_at = GREATEST(clock_timestamp(), a.updated_at + INTERVAL '1 microsecond')
    FROM recover_account_proxy_routes r
    WHERE a.id = r.id
    RETURNING a.id
)
INSERT INTO scheduler_outbox (event_type, account_id)
SELECT 'account_changed', id FROM changed;
