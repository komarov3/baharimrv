-- BahariMRV initial schema (PostgreSQL).
-- Optional identifier and text columns are NOT NULL with an empty-string default
-- because the Go models hold them as plain strings: GORM omits zero-valued fields
-- on insert, so a nullable column would round-trip as NULL and a uuid column would
-- reject ''. Referential integrity for those columns is enforced by the service
-- layer inside transactions.
CREATE TABLE users (
  id uuid PRIMARY KEY,
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  display_name text NOT NULL,
  phone text NOT NULL DEFAULT '',
  role text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_status ON users(status);

CREATE TABLE key_references (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  provider text NOT NULL,
  external_ref text NOT NULL,
  wrapped_key bytea NOT NULL,
  nonce bytea NOT NULL,
  public_key bytea NOT NULL,
  created_at timestamptz NOT NULL,
  rotated_at timestamptz,
  revoked_at timestamptz
);

CREATE TABLE identity_provisions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL UNIQUE REFERENCES users(id),
  state text NOT NULL,
  did text NOT NULL DEFAULT '',
  did_document_ref text NOT NULL DEFAULT '',
  address text NOT NULL DEFAULT '',
  key_reference_id text NOT NULL DEFAULT '',
  provision_tx_ref text NOT NULL DEFAULT '',
  network text NOT NULL,
  attempts integer NOT NULL DEFAULT 0,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  activated_at timestamptz,
  last_provisioned_at timestamptz
);
CREATE INDEX idx_identity_provisions_state ON identity_provisions(state);

CREATE TABLE local_sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_local_sessions_user_id ON local_sessions(user_id);
CREATE INDEX idx_local_sessions_expires_at ON local_sessions(expires_at);

CREATE TABLE external_identities (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  issuer text NOT NULL,
  subject text NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT idx_external_subject UNIQUE(issuer, subject),
  CONSTRAINT idx_external_user_issuer UNIQUE(user_id, issuer)
);

CREATE TABLE federated_grants (
  id uuid PRIMARY KEY,
  external_identity_id uuid NOT NULL REFERENCES external_identities(id),
  token_hash text NOT NULL UNIQUE,
  scope text NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_federated_grants_external_identity_id ON federated_grants(external_identity_id);

CREATE TABLE oidc_clients (
  id text PRIMARY KEY,
  secret_hash text NOT NULL DEFAULT '',
  redirect_uris text NOT NULL,
  allowed_scopes text NOT NULL,
  disabled boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE oauth_grants (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  client_id text NOT NULL REFERENCES oidc_clients(id),
  scope text NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_oauth_grants_user_id ON oauth_grants(user_id);
CREATE INDEX idx_oauth_grants_client_id ON oauth_grants(client_id);

CREATE TABLE authorization_codes (
  id uuid PRIMARY KEY,
  code_hash text NOT NULL UNIQUE,
  user_id uuid NOT NULL REFERENCES users(id),
  client_id text NOT NULL REFERENCES oidc_clients(id),
  grant_id uuid NOT NULL REFERENCES oauth_grants(id),
  redirect_uri text NOT NULL,
  scope text NOT NULL,
  nonce text NOT NULL,
  code_challenge text NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_authorization_codes_user_id ON authorization_codes(user_id);
CREATE INDEX idx_authorization_codes_client_id ON authorization_codes(client_id);
CREATE INDEX idx_authorization_codes_expires_at ON authorization_codes(expires_at);

CREATE TABLE access_tokens (
  id uuid PRIMARY KEY,
  token_hash text NOT NULL UNIQUE,
  grant_id uuid NOT NULL REFERENCES oauth_grants(id),
  user_id uuid NOT NULL REFERENCES users(id),
  client_id text NOT NULL REFERENCES oidc_clients(id),
  scope text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_access_tokens_grant_id ON access_tokens(grant_id);
CREATE INDEX idx_access_tokens_user_id ON access_tokens(user_id);
CREATE INDEX idx_access_tokens_expires_at ON access_tokens(expires_at);

CREATE TABLE methodologies (
  id uuid PRIMARY KEY,
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  version text NOT NULL,
  required_evidence text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE conservation_projects (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  manager_id uuid NOT NULL REFERENCES users(id),
  methodology_id uuid NOT NULL REFERENCES methodologies(id),
  status text NOT NULL,
  community_share_bps integer NOT NULL CHECK(community_share_bps >= 0 AND community_share_bps <= 10000),
  sandbox_demo boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_conservation_projects_manager_id ON conservation_projects(manager_id);

CREATE TABLE sites (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_sites_project_id ON sites(project_id);

CREATE TABLE plots (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  site_id uuid NOT NULL REFERENCES sites(id),
  code text NOT NULL,
  latitude double precision NOT NULL,
  longitude double precision NOT NULL,
  area_hectares text NOT NULL,
  assigned_monitor_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT idx_site_plot UNIQUE(site_id, code)
);
CREATE INDEX idx_plots_project_id ON plots(project_id);
CREATE INDEX idx_plots_assigned_monitor_id ON plots(assigned_monitor_id);

CREATE TABLE conservation_activities (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  plot_id uuid NOT NULL REFERENCES plots(id),
  manager_id uuid NOT NULL REFERENCES users(id),
  activity_type text NOT NULL,
  status text NOT NULL,
  reporting_period_start timestamptz NOT NULL,
  reporting_period_end timestamptz NOT NULL,
  methodology_version text NOT NULL,
  measurements jsonb NOT NULL DEFAULT '{}',
  current_event_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_conservation_activities_project_id ON conservation_activities(project_id);
CREATE INDEX idx_conservation_activities_plot_id ON conservation_activities(plot_id);
CREATE INDEX idx_conservation_activities_status ON conservation_activities(status);

CREATE TABLE monitoring_records (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  monitor_id uuid NOT NULL REFERENCES users(id),
  client_id text NOT NULL,
  device_timestamp timestamptz NOT NULL,
  server_receipt_time timestamptz NOT NULL,
  methodology_version text NOT NULL,
  schema_version text NOT NULL,
  latitude double precision NOT NULL,
  longitude double precision NOT NULL,
  gps_accuracy_meters double precision NOT NULL,
  alive_count integer NOT NULL,
  planted_count integer NOT NULL,
  average_height_metres text NOT NULL DEFAULT '',
  species jsonb NOT NULL DEFAULT '[]',
  attestation text NOT NULL,
  attestation_hash text NOT NULL,
  evidence_manifest jsonb NOT NULL DEFAULT '[]',
  content_commitment text NOT NULL,
  signed boolean NOT NULL DEFAULT FALSE,
  version integer NOT NULL DEFAULT 1,
  supersedes_id text NOT NULL DEFAULT '',
  sync_status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT idx_monitor_client UNIQUE(monitor_id, client_id)
);
CREATE INDEX idx_monitoring_records_activity_id ON monitoring_records(activity_id);
CREATE INDEX idx_monitoring_records_monitor_id ON monitoring_records(monitor_id);

CREATE TABLE evidence_objects (
  id uuid PRIMARY KEY,
  monitoring_record_id uuid NOT NULL REFERENCES monitoring_records(id),
  kind text NOT NULL,
  storage_uri text NOT NULL,
  sha256 text NOT NULL,
  media_type text NOT NULL,
  access_class text NOT NULL,
  captured_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_evidence_objects_monitoring_record_id ON evidence_objects(monitoring_record_id);

CREATE TABLE satellite_observations (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  adapter_mode text NOT NULL,
  status text NOT NULL,
  scene_id text NOT NULL DEFAULT '',
  acquired_at timestamptz,
  processing_version text NOT NULL DEFAULT '',
  result jsonb NOT NULL DEFAULT '{}',
  result_hash text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_satellite_observations_activity_id ON satellite_observations(activity_id);

CREATE TABLE verification_requests (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  verifier_id uuid NOT NULL REFERENCES users(id),
  status text NOT NULL,
  requested_at timestamptz NOT NULL,
  decided_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_verification_requests_activity_id ON verification_requests(activity_id);
CREATE INDEX idx_verification_requests_verifier_id ON verification_requests(verifier_id);

CREATE TABLE verifier_attestations (
  id uuid PRIMARY KEY,
  request_id uuid NOT NULL REFERENCES verification_requests(id),
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  verifier_id uuid NOT NULL REFERENCES users(id),
  issuer_did text NOT NULL,
  decision text NOT NULL,
  statement text NOT NULL DEFAULT '',
  attestation_hash text NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_verifier_attestations_activity_id ON verifier_attestations(activity_id);

CREATE TABLE provenance_events (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  claim_type text NOT NULL,
  version integer NOT NULL DEFAULT 1,
  commitment text NOT NULL,
  canonical_json jsonb NOT NULL DEFAULT '{}',
  previous_event_id text NOT NULL DEFAULT '',
  issuer_did text NOT NULL,
  status text NOT NULL,
  network text NOT NULL,
  transaction_ref text NOT NULL DEFAULT '',
  object_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  confirmed_at timestamptz,
  CONSTRAINT idx_activity_claim_commitment UNIQUE(activity_id, claim_type, commitment)
);
CREATE INDEX idx_provenance_events_activity_id ON provenance_events(activity_id);
CREATE INDEX idx_provenance_events_status ON provenance_events(status);

CREATE TABLE anchor_attempts (
  id uuid PRIMARY KEY,
  provenance_event_id uuid NOT NULL REFERENCES provenance_events(id),
  attempt integer NOT NULL,
  status text NOT NULL,
  transaction_ref text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_anchor_attempts_provenance_event_id ON anchor_attempts(provenance_event_id);

CREATE TABLE corrections (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  previous_event_id uuid NOT NULL REFERENCES provenance_events(id),
  superseding_event_id text NOT NULL DEFAULT '',
  reason text NOT NULL,
  requested_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_corrections_activity_id ON corrections(activity_id);

CREATE TABLE disputes (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  status text NOT NULL,
  reason text NOT NULL,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_disputes_activity_id ON disputes(activity_id);

CREATE TABLE sync_actions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  client_id text NOT NULL,
  action_type text NOT NULL,
  request_hash text NOT NULL,
  status text NOT NULL,
  response jsonb NOT NULL DEFAULT '{}',
  error text NOT NULL DEFAULT '',
  resource_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT idx_sync_user_client UNIQUE(user_id, client_id)
);
CREATE INDEX idx_sync_actions_status ON sync_actions(status);

CREATE TABLE audit_events (
  id uuid PRIMARY KEY,
  user_id text NOT NULL DEFAULT '',
  action text NOT NULL,
  resource text NOT NULL,
  request_id text NOT NULL DEFAULT '',
  status integer NOT NULL,
  ip_address text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_audit_events_user_id ON audit_events(user_id);
CREATE INDEX idx_audit_events_action ON audit_events(action);

CREATE TABLE proof_documents (
  id uuid PRIMARY KEY,
  activity_id uuid NOT NULL REFERENCES conservation_activities(id),
  version integer NOT NULL,
  proof_type text NOT NULL,
  status text NOT NULL,
  snapshot jsonb NOT NULL DEFAULT '{}',
  manifest_hash text NOT NULL,
  pdf bytea NOT NULL,
  qr_code bytea NOT NULL,
  superseded_by_id text NOT NULL DEFAULT '',
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_proof_documents_activity_id ON proof_documents(activity_id);
CREATE INDEX idx_proof_documents_status ON proof_documents(status);

CREATE TABLE credit_batches (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  registry_issuance_id text NOT NULL UNIQUE,
  registry_name text NOT NULL,
  vintage integer NOT NULL,
  serial_start text NOT NULL,
  serial_end text NOT NULL,
  quantity text NOT NULL,
  available_quantity text NOT NULL,
  status text NOT NULL,
  adapter_mode text NOT NULL,
  reconciled_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT idx_registry_serial UNIQUE(serial_start, serial_end)
);
CREATE INDEX idx_credit_batches_project_id ON credit_batches(project_id);
CREATE INDEX idx_credit_batches_status ON credit_batches(status);

CREATE TABLE marketplace_listings (
  id uuid PRIMARY KEY,
  batch_id uuid NOT NULL UNIQUE REFERENCES credit_batches(id),
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  product_type text NOT NULL,
  price_per_tonne text NOT NULL,
  currency text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_marketplace_listings_status ON marketplace_listings(status);

CREATE TABLE purchases (
  id uuid PRIMARY KEY,
  listing_id uuid NOT NULL REFERENCES marketplace_listings(id),
  batch_id uuid NOT NULL REFERENCES credit_batches(id),
  buyer_id uuid NOT NULL REFERENCES users(id),
  quantity text NOT NULL,
  gross_amount text NOT NULL,
  currency text NOT NULL,
  payment_mode text NOT NULL,
  payment_reference text NOT NULL UNIQUE,
  status text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_purchases_buyer_id ON purchases(buyer_id);

CREATE TABLE custody_events (
  id uuid PRIMARY KEY,
  batch_id uuid NOT NULL REFERENCES credit_batches(id),
  purchase_id text NOT NULL DEFAULT '',
  owner_id uuid NOT NULL REFERENCES users(id),
  event_type text NOT NULL,
  quantity text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_custody_events_batch_id ON custody_events(batch_id);
CREATE INDEX idx_custody_events_owner_id ON custody_events(owner_id);

CREATE TABLE revenue_allocations (
  id uuid PRIMARY KEY,
  purchase_id uuid NOT NULL REFERENCES purchases(id),
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  recipient text NOT NULL,
  basis_points integer NOT NULL,
  amount text NOT NULL,
  currency text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT idx_purchase_recipient UNIQUE(purchase_id, recipient)
);
CREATE INDEX idx_revenue_allocations_project_id ON revenue_allocations(project_id);

CREATE TABLE retirements (
  id uuid PRIMARY KEY,
  batch_id uuid NOT NULL REFERENCES credit_batches(id),
  owner_id uuid NOT NULL REFERENCES users(id),
  quantity text NOT NULL,
  status text NOT NULL,
  registry_certificate text NOT NULL DEFAULT '',
  transaction_ref text NOT NULL DEFAULT '',
  object_id text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  completed_at timestamptz
);
CREATE INDEX idx_retirements_batch_id ON retirements(batch_id);
CREATE INDEX idx_retirements_status ON retirements(status);

CREATE TABLE reconciliation_cases (
  id uuid PRIMARY KEY,
  batch_id uuid NOT NULL REFERENCES credit_batches(id),
  reason text NOT NULL,
  status text NOT NULL,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_reconciliation_cases_batch_id ON reconciliation_cases(batch_id);

CREATE TABLE financial_imports (
  id uuid PRIMARY KEY,
  source text NOT NULL,
  external_id text NOT NULL,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  amount text NOT NULL,
  currency text NOT NULL,
  occurred_at timestamptz NOT NULL,
  payload_hash text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT idx_financial_source UNIQUE(source, external_id)
);
CREATE INDEX idx_financial_imports_project_id ON financial_imports(project_id);

CREATE TABLE project_documents (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  kind text NOT NULL,
  name text NOT NULL,
  status text NOT NULL,
  version text NOT NULL DEFAULT '',
  sha256 text NOT NULL DEFAULT '',
  stored_uri text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX idx_project_documents_project_id ON project_documents(project_id);

CREATE TABLE step_up_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash text NOT NULL UNIQUE,
  action text NOT NULL,
  used_at timestamptz,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_step_up_tokens_user_id ON step_up_tokens(user_id);
CREATE INDEX idx_step_up_tokens_expires_at ON step_up_tokens(expires_at);
CREATE TABLE satellite_passes (id UUID PRIMARY KEY, project_id UUID NOT NULL REFERENCES conservation_projects(id), activity_id UUID REFERENCES conservation_activities(id), mission TEXT NOT NULL, pass_type TEXT NOT NULL, window_start TIMESTAMPTZ NOT NULL, window_end TIMESTAMPTZ NOT NULL, acquired_at TIMESTAMPTZ, scene_id TEXT, orbit TEXT, tile TEXT, cloud_cover_pct NUMERIC(6,2), coverage_pct NUMERIC(6,2) NOT NULL, status_detail TEXT, adapter_mode TEXT NOT NULL, result_hash TEXT, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL); CREATE INDEX idx_satellite_passes_project_id ON satellite_passes(project_id); CREATE INDEX idx_satellite_passes_mission ON satellite_passes(mission);
