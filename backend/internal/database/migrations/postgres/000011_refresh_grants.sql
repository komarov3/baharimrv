CREATE TABLE refresh_grants (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_refresh_grants_user_id ON refresh_grants(user_id);
CREATE INDEX idx_refresh_grants_expires_at ON refresh_grants(expires_at);