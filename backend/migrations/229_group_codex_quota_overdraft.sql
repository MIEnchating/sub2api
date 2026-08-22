ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS codex_quota_overdraft_enabled BOOLEAN DEFAULT NULL;

COMMENT ON COLUMN groups.codex_quota_overdraft_enabled IS
    'Codex quota overdraft group override; NULL inherits the global setting';
