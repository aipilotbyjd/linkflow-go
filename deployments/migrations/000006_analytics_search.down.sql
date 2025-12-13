-- Drop search schema
DROP TRIGGER IF EXISTS update_workflow_index_updated_at ON search.workflow_index;
DROP TRIGGER IF EXISTS update_workflow_search_vector_trigger ON search.workflow_index;
DROP FUNCTION IF EXISTS search.update_workflow_search_vector();
DROP TABLE IF EXISTS search.search_history;
DROP TABLE IF EXISTS search.workflow_index;
DROP SCHEMA IF EXISTS search;

-- Drop analytics schema
DROP TABLE IF EXISTS analytics.node_usage;
DROP TABLE IF EXISTS analytics.system_metrics;
DROP TABLE IF EXISTS analytics.user_activity;
DROP TABLE IF EXISTS analytics.workflow_stats;
DROP SCHEMA IF EXISTS analytics;
