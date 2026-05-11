-- noinspection SqlNoDataSourceInspectionForFile
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS urls_user_id_idx ON urls (user_id);
