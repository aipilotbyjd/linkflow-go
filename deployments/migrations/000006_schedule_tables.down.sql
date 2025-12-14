-- ============================================================================
-- Migration: 000006_schedule_tables (ROLLBACK)
-- Description: Drop scheduling tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_schedules_updated_at ON schedule.schedules;

DROP TABLE IF EXISTS schedule.schedule_executions;
DROP TABLE IF EXISTS schedule.schedules;

COMMIT;
