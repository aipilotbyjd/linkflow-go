-- Drop trigger and function
DROP TRIGGER IF EXISTS update_performance_summary_trigger ON execution.workflow_executions;
DROP FUNCTION IF EXISTS execution.update_performance_summary();

-- Drop indexes
DROP INDEX IF EXISTS execution.idx_performance_summary_date;
DROP INDEX IF EXISTS execution.idx_performance_summary_workflow_date;
DROP INDEX IF EXISTS execution.idx_metrics_archive_metric_type;
DROP INDEX IF EXISTS execution.idx_metrics_archive_date;
DROP INDEX IF EXISTS execution.idx_metrics_archive_execution_id;
DROP INDEX IF EXISTS execution.idx_execution_metrics_type_timestamp;
DROP INDEX IF EXISTS execution.idx_execution_metrics_timestamp;
DROP INDEX IF EXISTS execution.idx_execution_metrics_metric_type;
DROP INDEX IF EXISTS execution.idx_execution_metrics_node_id;
DROP INDEX IF EXISTS execution.idx_execution_metrics_execution_id;
DROP INDEX IF EXISTS execution.idx_state_transitions_to_state;
DROP INDEX IF EXISTS execution.idx_state_transitions_timestamp;
DROP INDEX IF EXISTS execution.idx_state_transitions_execution_id;

-- Drop tables
DROP TABLE IF EXISTS execution.performance_summary CASCADE;
DROP TABLE IF EXISTS execution.execution_metrics_archive CASCADE;
DROP TABLE IF EXISTS execution.execution_metrics CASCADE;
DROP TABLE IF EXISTS execution.state_transitions CASCADE;
