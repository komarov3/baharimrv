-- Project boundary (SQLite variant). Same column as the PostgreSQL migration, with
-- jsonb replaced by TEXT as in sqlite/000001_init.sql. No comment in this file may
-- contain a semicolon because the runner splits the file on every one.

ALTER TABLE conservation_projects ADD COLUMN boundary TEXT;
