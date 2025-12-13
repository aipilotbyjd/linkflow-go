-- Drop template schema
DROP TRIGGER IF EXISTS update_templates_updated_at ON template.workflow_templates;
DROP TABLE IF EXISTS template.downloads;
DROP TABLE IF EXISTS template.ratings;
DROP TABLE IF EXISTS template.workflow_templates;
DROP TABLE IF EXISTS template.categories;
DROP SCHEMA IF EXISTS template;

-- Drop notification schema
DROP TRIGGER IF EXISTS update_preferences_updated_at ON notification.preferences;
DROP TRIGGER IF EXISTS update_channels_updated_at ON notification.channels;
DROP TABLE IF EXISTS notification.history;
DROP TABLE IF EXISTS notification.queue;
DROP TABLE IF EXISTS notification.preferences;
DROP TABLE IF EXISTS notification.channels;
DROP SCHEMA IF EXISTS notification;
