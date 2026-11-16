CREATE TABLE activity_evidence (
  id TEXT PRIMARY KEY,
  activity_id TEXT NOT NULL REFERENCES conservation_activities(id),
  kind TEXT NOT NULL CHECK (kind = 'photo'),
  storage_uri TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  media_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
  captured_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  UNIQUE (activity_id, sha256)
);
