-- Run with psql after all users of this database have upgraded, in a maintenance window.
-- An old binary must never resume staging after the journal is archived.
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '3s';
DO $retire$
BEGIN
    IF to_regclass('public.usage_billing_events') IS NULL THEN
        RETURN;
    END IF;
    LOCK TABLE public.usage_billing_events IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT FROM public.usage_billing_events WHERE state NOT IN ('complete', 'legacy')) THEN
        RAISE EXCEPTION 'Unfinished billing events remain. Drain pending/applied and reconcile blocked events before retiring history.';
    END IF;
    ALTER TABLE public.usage_billing_events RENAME TO usage_billing_events_retired;
END
$retire$;
COMMIT;
