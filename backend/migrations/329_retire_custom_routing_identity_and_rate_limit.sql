-- Upstream retired its custom identity and billing ceilings in this slot.
-- This fork still serves these explicit account policies. Preserve their mode,
-- seed, billing ceiling, probe state, account data and scheduler state unchanged.
-- Keep the migration filename so the upstream sequence remains compatible.
SELECT 1;
