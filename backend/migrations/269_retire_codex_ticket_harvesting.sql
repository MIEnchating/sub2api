-- Fork compatibility: retain active account-scoped ticket harvesting, its
-- settings, short-lived tickets and cookies. Only obsolete account-owned proxy
-- secrets are retired; the dedicated global harvest proxy remains unchanged.
DELETE FROM settings WHERE key = 'openai_codex_ticket_harvest_proxy_ids';

WITH retired AS (
    SELECT a.id, array_agg(e.key) AS keys
    FROM accounts a
    CROSS JOIN LATERAL jsonb_object_keys(
        CASE WHEN jsonb_typeof(a.extra) = 'object' THEN a.extra ELSE '{}'::jsonb END
    ) AS e(key)
    WHERE e.key IN ('codex_harvest_proxy_url', 'codex_ticket_harvest_proxy_ids')
    GROUP BY a.id
), changed AS (
    UPDATE accounts a
    SET extra = a.extra - retired.keys, updated_at = NOW()
    FROM retired
    WHERE a.id = retired.id
    RETURNING a.id
)
INSERT INTO scheduler_outbox (event_type, account_id)
SELECT 'account_changed', id FROM changed;
