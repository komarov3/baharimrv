CREATE TABLE activity_evidence (
  id UUID PRIMARY KEY,
  activity_id UUID NOT NULL REFERENCES conservation_activities(id),
  kind TEXT NOT NULL CHECK (kind = 'photo'),
  storage_uri TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  media_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
  captured_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (activity_id, sha256)
);
