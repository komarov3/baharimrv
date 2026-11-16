-- Activity naming (SQLite variant). Same column as the PostgreSQL migration. The manager
-- labels their own record, nothing is attested and no anchor is touched, so an empty
-- string simply means the activity has no name and the clients show its type.
-- No comment in this file may contain a semicolon because the runner splits on every one.

ALTER TABLE conservation_activities ADD COLUMN title TEXT NOT NULL DEFAULT '';
