-- noinspection SqlNoDataSourceInspectionForFile
DROP INDEX IF EXISTS urls_user_id_idx;
ALTER TABLE urls DROP COLUMN IF EXISTS user_id;
