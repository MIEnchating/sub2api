-- Historical credit balances, ledger, audit and delivery records are retained.
UPDATE auto_credit_config SET enabled = false, version = version + 1 WHERE enabled;

-- Keep the original pool relationships and archive request routing configuration.
-- Pin each account to its saved fixed egress or to one of its existing active pool members.
WITH candidates AS (
    SELECT a.id, a.extra, a.proxy_id,
           a.extra ? 'openai_proxy_mode' AS has_openai_mode,
           (a.extra ? 'openai_proxy_mode' AND a.extra ->> 'openai_proxy_mode' IS DISTINCT FROM 'single')
             OR (NOT (a.extra ? 'openai_proxy_mode') AND a.extra ->> 'openai_excel_bps_proxy_mode' = 'request'
                 AND a.extra -> 'openai_excel_bps' = 'true'::jsonb) AS request_mode
    FROM accounts a
    WHERE a.deleted_at IS NULL
      AND (a.extra ?| ARRAY['openai_proxy_mode', 'openai_proxy_ids', 'openai_excel_bps_proxy_mode', 'openai_excel_bps_proxy_ids']
           OR EXISTS (SELECT 1 FROM account_proxies ap WHERE ap.account_id = a.id))
),
routes AS (
    SELECT a.*, COALESCE(current_proxy.id, request_proxy.id, pool_proxy.id) AS fixed_proxy_id,
           a.request_mode IS TRUE OR EXISTS (SELECT 1 FROM account_proxies ap WHERE ap.account_id = a.id) AS needs_proxy
    FROM candidates a
    LEFT JOIN proxies current_proxy ON current_proxy.id = a.proxy_id AND current_proxy.deleted_at IS NULL
        AND current_proxy.status = 'active' AND (current_proxy.expires_at IS NULL OR current_proxy.expires_at > NOW())
    LEFT JOIN LATERAL (
        SELECT p.id
        FROM jsonb_array_elements(CASE
            WHEN a.has_openai_mode AND jsonb_typeof(a.extra -> 'openai_proxy_ids') = 'array' THEN a.extra -> 'openai_proxy_ids'
            WHEN NOT a.has_openai_mode AND jsonb_typeof(a.extra -> 'openai_excel_bps_proxy_ids') = 'array' THEN a.extra -> 'openai_excel_bps_proxy_ids'
            ELSE '[]'::jsonb END) WITH ORDINALITY AS member(value, position)
        JOIN proxies p ON to_jsonb(p.id) = member.value
        WHERE a.request_mode IS TRUE AND p.deleted_at IS NULL AND p.status = 'active'
          AND (p.expires_at IS NULL OR p.expires_at > NOW())
          AND p.protocol IN ('http', 'https', 'socks5', 'socks5h')
        ORDER BY member.position
        LIMIT 1
    ) request_proxy ON true
    LEFT JOIN LATERAL (
        SELECT p.id FROM account_proxies ap JOIN proxies p ON p.id = ap.proxy_id
        WHERE ap.account_id = a.id AND p.deleted_at IS NULL AND p.status = 'active'
          AND (p.expires_at IS NULL OR p.expires_at > NOW())
        ORDER BY ap.position, ap.proxy_id
        LIMIT 1
    ) pool_proxy ON true
),
changed AS (
    UPDATE accounts a
    SET proxy_id = CASE WHEN r.needs_proxy THEN COALESCE(r.fixed_proxy_id, a.proxy_id) ELSE a.proxy_id END,
        schedulable = a.schedulable AND (NOT r.needs_proxy OR r.fixed_proxy_id IS NOT NULL),
        extra = (COALESCE(a.extra, '{}'::jsonb) - ARRAY['openai_proxy_mode', 'openai_proxy_ids', 'openai_excel_bps_proxy_mode', 'openai_excel_bps_proxy_ids'])
            || CASE WHEN r.needs_proxy AND r.fixed_proxy_id IS NULL THEN '{"retired_multi_proxy_needs_review":true}'::jsonb ELSE '{}'::jsonb END
            || CASE WHEN a.extra ?| ARRAY['openai_proxy_mode', 'openai_proxy_ids', 'openai_excel_bps_proxy_mode', 'openai_excel_bps_proxy_ids']
                 THEN jsonb_build_object('retired_request_proxy', jsonb_strip_nulls(jsonb_build_object(
                    'openai_proxy_mode', a.extra -> 'openai_proxy_mode',
                    'openai_proxy_ids', a.extra -> 'openai_proxy_ids',
                    'openai_excel_bps_proxy_mode', a.extra -> 'openai_excel_bps_proxy_mode',
                    'openai_excel_bps_proxy_ids', a.extra -> 'openai_excel_bps_proxy_ids')))
                 ELSE '{}'::jsonb END,
        updated_at = GREATEST(clock_timestamp(), a.updated_at + INTERVAL '1 microsecond')
    FROM routes r
    WHERE a.id = r.id
    RETURNING a.id
)
INSERT INTO scheduler_outbox (event_type, account_id)
SELECT 'account_changed', id FROM changed;

-- Keep the original bindings as history. The live table must be empty so old
-- route checks do not treat a retired member as active or block a fixed exit.
CREATE TABLE IF NOT EXISTS retired_account_proxy_bindings (
    account_id BIGINT NOT NULL,
    proxy_id BIGINT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, proxy_id)
);

WITH archived AS (
    INSERT INTO retired_account_proxy_bindings (account_id, proxy_id, position, created_at)
    SELECT account_id, proxy_id, position, created_at FROM account_proxies
    ON CONFLICT (account_id, proxy_id) DO UPDATE
        SET position = EXCLUDED.position, created_at = EXCLUDED.created_at
    RETURNING account_id, proxy_id
)
DELETE FROM account_proxies ap
USING archived
WHERE ap.account_id = archived.account_id AND ap.proxy_id = archived.proxy_id;
