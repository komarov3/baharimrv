-- Rotating refresh grants (SQLite variant).
-- Same shape as the PostgreSQL migration. No comment below this one may contain
-- a semicolon, because the runner splits this file on every one.

CREATE TABLE refresh_grants (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), token_hash TEXT NOT NULL UNIQUE, expires_at TIMESTAMP NOT NULL, revoked_at TIMESTAMP, created_at TIMESTAMP NOT NULL);
CREATE INDEX idx_refresh_grants_user_id ON refresh_grants(user_id);
CREATE INDEX idx_refresh_grants_expires_at ON refresh_grants(expires_at);