-- Allow explicit plans to target one account. Group strategies
-- retain their existing scope and disabled-draft rules.
ALTER TABLE scheduled_test_plans DROP CONSTRAINT scheduled_test_plans_target_mode_check;
ALTER TABLE scheduled_test_plans ADD CONSTRAINT scheduled_test_plans_target_mode_check CHECK (
    (target_mode='all_accounts' AND account_id IS NULL AND (
        (cardinality(group_ids)>0 AND group_id IS NOT NULL AND group_id=group_ids[1])
        OR (NOT enabled AND cardinality(group_ids)=0 AND group_id IS NULL)
    ))
    OR (target_mode='account' AND account_id IS NOT NULL AND account_id>0
        AND group_id IS NULL AND cardinality(group_ids)=0
        AND (pelican_config IS NULL OR jsonb_typeof(pelican_config)='object'))
);
