-- Retire Prism and normalize the old standalone fingerprint alias.
-- This fork retires protection presets through the explicit restore endpoint:
-- retain saved snapshots, independent adaptive concurrency and health policy.
DO $$
DECLARE
    a RECORD;
    next_extra JSONB;
    next_credentials JSONB;
    seed_bytes BYTEA;
BEGIN
    FOR a IN
        SELECT id, platform, type, extra, credentials
        FROM accounts
        WHERE extra ? 'prism'
           OR credentials ?| ARRAY['prism_cookie', 'prism_cookie_configured']
           OR (extra ->> 'codex_fingerprint_mode' = 'account_device'
               AND COALESCE(extra -> 'anti_degradation', extra #> '{anti_degrade,enabled}', 'false'::jsonb) <> 'true'::jsonb)
        FOR UPDATE
    LOOP
        next_extra := COALESCE(a.extra, '{}'::jsonb) - 'prism';
        next_credentials := a.credentials - ARRAY['prism_cookie', 'prism_cookie_configured'];
        IF next_extra ->> 'codex_fingerprint_mode' = 'account_device'
           AND COALESCE(next_extra -> 'anti_degradation', next_extra #> '{anti_degrade,enabled}', 'false'::jsonb) <> 'true'::jsonb
           AND a.platform = 'openai' AND a.type IN ('oauth', 'setup-token') THEN
            -- Preserve the exact account-derived identity when materializing a seed.
            IF COALESCE(next_extra ->> 'codex_fingerprint_seed', '') !~
               '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
               OR next_extra ->> 'codex_fingerprint_seed' = '00000000-0000-0000-0000-000000000000' THEN
                seed_bytes := substring(sha256(convert_to('sub2api:openai-account-fingerprint:v1:' || a.id::text, 'UTF8')) FROM 1 FOR 16);
                seed_bytes := set_byte(seed_bytes, 6, (get_byte(seed_bytes, 6) & 15) | 64);
                seed_bytes := set_byte(seed_bytes, 8, (get_byte(seed_bytes, 8) & 63) | 128);
                next_extra := jsonb_set(next_extra, '{codex_fingerprint_seed}', to_jsonb(encode(seed_bytes, 'hex')::uuid::text));
            END IF;
            next_extra := jsonb_set(next_extra, '{codex_fingerprint_mode}', '"device"');
        END IF;
        IF next_extra IS DISTINCT FROM a.extra OR next_credentials IS DISTINCT FROM a.credentials THEN
            UPDATE accounts
            SET extra = next_extra, credentials = next_credentials, updated_at = NOW()
            WHERE id = a.id;
            INSERT INTO scheduler_outbox (event_type, account_id) VALUES ('account_changed', a.id);
        END IF;
    END LOOP;
END $$;
