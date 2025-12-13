-- Drop storage schema
DROP TRIGGER IF EXISTS update_files_updated_at ON storage.files;
DROP TABLE IF EXISTS storage.file_access_logs;
DROP TABLE IF EXISTS storage.files;
DROP SCHEMA IF EXISTS storage;

-- Drop variables schema
DROP TRIGGER IF EXISTS update_variables_updated_at ON variable.variables;
DROP TABLE IF EXISTS variable.variables;
DROP SCHEMA IF EXISTS variable;
