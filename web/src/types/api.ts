export type IntegrationMode = 'live' | 'sandbox' | 'test' | 'unavailable';

// ServiceModes is what the backend says its own adapters are doing. A sandbox
// adapter is never presented in the UI as a live network.
export interface ServiceModes {
  chain: string;
  satellite: string;
  registry: string;
  payment: string;
}

export interface HealthReport {
  status: 'ok';
  modes: ServiceModes;
  serverTime: string;
}

// ChainNetwork is what the backend adapter reports. Sandbox is a first-class
// value: a sandbox commitment is not a public ledger fact.
export type ChainNetwork = 'iota_testnet' | 'iota_mainnet' | 'sandbox';

export type EvidenceState =
  | 'measured'
  | 'estimated'
  | 'claimed'
  | 'verified'
  | 'pending'
  | 'rejected'
  | 'unavailable'
  | 'anchored'
  | 'failed'
  | 'disputed'
  | 'revoked'
  | 'superseded'
  | 'sandbox'
  | 'achieved_unverified';

export interface User {
  id: string;
  name: string;
  email: string;
  role: string;
}

export interface IdentityProvision {
  id: string;
  userId: string;
  state: 'pending' | 'provisioning' | 'active' | 'failed' | 'revoked';
  did?: string;
  didDocumentReference?: string;
  address?: string;
  provisionTransactionReference?: string;
  network: ChainNetwork;
  attempts: number;
  createdAt: string;
  updatedAt: string;
  activatedAt?: string;
  lastProvisionedAt?: string;
  lastError?: string;
}

export interface AuthSession {
  sessionToken: string;
  expiresAt?: string;
  user: User;
  identity?: IdentityProvision;
}

export interface DashboardMetric {
  key: string;
  label: string;
  value: string;
  unit?: string;
  status: EvidenceState;
  methodologyPeriod?: string;
}

export interface ProjectDashboard {
  project: {
    id: string;
    name: string;
    communityGroup?: string;
    species?: string;
    conservationType?: string;
    siteCount: number;
    methodology: string;
    methodologyPeriod?: string;
  };
  plots: Array<{
    id: string;
    label: string;
    latitude: number;
    longitude: number;
    areaHectares: string;
    activityType?: string;
    status: EvidenceState;
    // chainState comes from the provenance ledger, never from the activity status.
    chainState: 'not_recorded' | 'pending' | 'anchored' | 'failed' | 'superseded' | 'revoked' | 'disputed';
    ndvi?: number;
  }>;
  // chainMode qualifies every chainState above; "sandbox" means no public ledger.
  chainMode: string;
  ndvi: {
    status: 'available' | 'pending' | 'unavailable';
    source?: string;
    acquiredAt?: string;
  };
  metrics: DashboardMetric[];
  wallet?: ProjectWallet;
  alerts: Array<{
    id: string;
    severity: 'info' | 'warning' | 'critical';
    title: string;
    detail: string;
    actionLabel?: string;
    actionHref?: string;
  }>;
}

// ProjectRow is one line of the manager's conservation register table.
export interface ProjectRow {
  id: string;
  name: string;
  communityGroup?: string;
  site?: string;
  species?: string;
  conservationType?: string;
  activityType?: string;
  status: string;
  areaHectares: string;
  seedlingsPlanted: number;
  seedlingsSurvived: number;
  survivalRate: string;
  latestMrvAt?: string;
  verificationState: EvidenceState;
  proofAvailable: boolean;
  sandboxDemo: boolean;
  managerName?: string;
  integrationNote?: string;
}

export interface ProjectTable {
  rows: ProjectRow[];
  summary: { total: number; active: number; pendingVerification: number; achievedUnverified: number };
}
export interface Methodology {
  id: string;
  code: string;
  name: string;
  version: string;
}

export interface MethodologyTable {
  items: Methodology[];
}

export interface DecimalAmount {
  amount: string;
  currency: string;
}

export interface WalletLedgerEntry {
  id: string;
  kind: 'tourism_contribution' | 'carbon_credit_sale' | 'community_allocation' | 'other';
  description: string;
  amount: DecimalAmount;
  status: 'imported' | 'pending' | 'failed' | 'reconciled';
  source: string;
  occurredAt: string;
  reference?: string;
}

export interface ProjectWallet {
  projectId: string;
  mode: IntegrationMode;
  balances: {
    tourismContributions: DecimalAmount;
    carbonCreditSales: DecimalAmount;
    communityShare: DecimalAmount;
  };
  revenueRule?: {
    id: string;
    label: string;
    communityPercent: string;
    effectiveFrom: string;
    allocations: Array<{ label: string; percent: string; amount: DecimalAmount }>;
  };
  entries: WalletLedgerEntry[];
  lastImportedAt?: string;
}

export interface Reconciliation {
  projectId: string;
  status: 'reconciled' | 'pending' | 'mismatch' | 'unavailable';
  checkedAt?: string;
  issues: Array<{ id: string; detail: string; severity: 'warning' | 'critical' }>;
}

export type MonitoringStatus =
  | 'on_device'
  | 'uploading'
  | 'submitted'
  | 'awaiting_verification'
  | 'anchoring'
  | 'anchored'
  | 'verification_failed'
  | 'sync_conflict'
  | 'failed';

export interface MonitoringRecordPayload {
  clientId: string;
  activityId: string;
  plotId: string;
  methodologyVersion: string;
  schemaVersion: string;
  deviceTimestamp: string;
  location: { latitude: number; longitude: number; accuracyMetres: number };
  aliveCount: number;
  plantedCount: number;
  averageHeightMetres: string;
  species: string;
  observations: string;
  attestation: { statement: string; identityDid?: string };
  evidenceManifest: Array<{ name: string; kind: 'photo' | 'voice'; sha256: string; mediaType: string; size: number }>;
}

export interface SyncBatchResponse {
  results: MonitoringRecordResult[];
}

// MonitoringRecordResult reports one action's outcome. A conflict is data the
// client must reconcile, never a silently overwritten record.
export interface MonitoringRecordResult {
  clientId: string;
  monitoringRecordId?: string;
  status: MonitoringStatus;
  error?: string;
  conflict?: { serverVersion: string; detail: string };
  signed: boolean;
  lowAccuracy: boolean;
}

export interface DeltaPage {
  changes: Array<{
    kind: 'activity' | 'monitoring_record' | 'provenance_event' | 'proof';
    id: string;
    projectId: string;
    activityId: string;
    status: string;
    version: number;
    changedAt: string;
  }>;
  serverTime: string;
  nextCursor: string;
  truncated: boolean;
}

// PlotInput/ProjectPlanner are the write shapes the conservation API accepts.
export interface PlotInput {
  code: string;
  siteName: string;
  latitude: number;
  longitude: number;
  areaHectares: string;
  assignedMonitorId?: string;
}

export interface PlotReceipt {
  id: string;
  projectId: string;
  siteId: string;
  code: string;
  latitude: number;
  longitude: number;
  areaHectares: string;
  assignedMonitorId: string;
}

export interface ProjectPlanner {
  name: string;
  methodologyId: string;
  conservationType?: string;
  communityGroup: string;
  species: string;
  communityShareBasisPoints: number;
  sites: string[];
  plots: PlotInput[];
}

export interface ActivitySummary {
  id: string;
  projectId: string;
  plotId: string;
  managerId: string;
  activityType: string;
  status: string;
  reportingPeriodStart: string;
  reportingPeriodEnd: string;
  methodologyVersion: string;
  currentEventId?: string;
  createdAt: string;
  updatedAt: string;
}

// StepUpAction names the single gated operation a ticket may authorize.
export type StepUpAction = 'verification_decision' | 'marketplace_purchase' | 'credit_retirement' | 'proof_generation';

export interface VerificationRequest {
  id: string;
  activityId: string;
  verifierId: string;
  status: 'requested' | 'in_review' | 'approved' | 'rejected' | 'corrections_requested';
  requestedAt: string;
  decidedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface VerificationQueue {
  items: VerificationQueueItem[];
}

export interface DecisionReceipt {
  status: string;
  attestationReference?: string;
  provenanceStatus?: string;
  explorerUrl?: string;
}

export interface DisputeReceipt {
  id: string;
  activityId: string;
  status: 'open' | 'resolved';
  reason: string;
  resolvedAt?: string;
  createdAt: string;
}

export interface ProofTable {
  proofs: ProofDocument[];
}

export interface LinkedPlatform {
  issuer: string;
  platform: 'Jahazi';
  subject: string;
  linkedAt: string;
  status: 'active' | 'revoked';
}

export interface VerificationStage {
  id: string;
  key: 'field_data' | 'satellite' | 'third_party_audit' | 'registry_issuance';
  label: string;
  owner: string;
  status: 'complete' | 'in_progress' | 'pending' | 'blocked' | 'failed' | 'unavailable';
  timestamp?: string;
  evidenceReferences: string[];
  exception?: string;
  nextAction?: string;
}

export interface ProvenanceEvent {
  id: string;
  type: 'evidence_submitted' | 'achieved_unverified' | 'achievement_verified' | 'custody_changed' | 'retired' | 'correction';
  actor: string;
  timestamp: string;
  status: 'pending' | 'anchored' | 'failed' | 'superseded' | 'revoked' | 'disputed';
  network?: ChainNetwork;
  commitment?: string;
  chainReference?: string;
  explorerUrl?: string;
  objectId?: string;
  version?: number;
  detail: string;
}

// VerificationQueueItem is one request awaiting an independent decision.
export interface VerificationQueueItem {
  id: string;
  activityId: string;
  projectName: string;
  plotCode: string;
  status: 'requested' | 'in_review' | 'approved' | 'rejected' | 'corrections_requested';
  requestedAt: string;
  methodologyVersion: string;
  evidenceCount: number;
  recordsCount: number;
  unsignedRecords: number;
}

// AnchorStatus is what the chain actually says about one provenance event.
export interface AnchorStatus {
  eventId: string;
  activityId: string;
  claimType: string;
  version: number;
  status: 'pending' | 'anchored' | 'failed' | 'superseded' | 'revoked' | 'disputed';
  network: string;
  mode: string;
  commitment: string;
  objectId?: string;
  transactionReference?: string;
  explorerUrl?: string;
  confirmedAt?: string;
  attempts: number;
  lastError?: string;
  readback: 'confirmed' | 'failed' | 'not_anchored' | 'unavailable_in_sandbox';
  canRetry: boolean;
}

// AnchoredEvent is the raw provenance row completion and anchor retry return.
// A reference here is only a submission receipt until AnchorStatus confirms it.
export interface AnchoredEvent {
  id: string;
  activityId: string;
  claimType: 'achieved_unverified' | 'achievement_verified' | string;
  version: number;
  commitment: string;
  previousEventId?: string;
  issuerDid: string;
  status: 'pending' | 'anchored' | 'failed' | 'superseded' | 'revoked' | 'disputed';
  network: string;
  transactionReference?: string;
  objectId?: string;
  createdAt: string;
  confirmedAt?: string;
}

export interface ActivityProvenance {
  activity: { id: string; projectName: string; achievementId?: string; verificationRequestId?: string; status: string };
  stages: VerificationStage[];
  documents: Array<{
    id: string;
    kind: 'project_design' | 'monitoring_report' | 'co_benefit_attestation' | 'other';
    name: string;
    status: 'complete' | 'missing' | 'expired' | 'pending' | 'rejected';
    version?: string;
    updatedAt?: string;
    downloadUrl?: string;
  }>;
  events: ProvenanceEvent[];
}

export interface MarketplaceListing {
  id: string;
  batchId: string;
  projectName: string;
  vintage: string;
  productType: 'registry_issued' | 'forward';
  registry: { name: string; status: 'reconciled' | 'pending' | 'unavailable'; reference?: string };
  availableTonnes: string;
  coBenefits: string[];
  pricePerTonne: DecimalAmount;
  mode: IntegrationMode;
}

export interface PurchaseResult {
  id: string;
  listingId: string;
  batchId: string;
  quantityTonnes: string;
  total: DecimalAmount;
  status: 'confirmed' | 'pending' | 'failed' | 'reconciliation_pending';
  mode: IntegrationMode;
  custodyReference?: string;
  settlementReference?: string;
}

export interface RetirementResult {
  id: string;
  batchId: string;
  quantityTonnes: string;
  status: 'completed' | 'pending' | 'failed' | 'reconciliation_pending';
  mode: IntegrationMode;
  registrySerialReference?: string;
  registryCertificateReference?: string;
  chainReference?: string;
  chainNetwork?: ChainNetwork;
  explorerUrl?: string;
  retiredAt?: string;
  detail?: string;
}

export interface IdentityProfile {
  user: User;
  status: 'active' | 'identity_pending' | 'identity_failed';
  did?: string;
  address?: string;
  didDocumentReference?: string;
  keyCustodyReference?: string;
  network: ChainNetwork;
  roleCapabilities: Array<{ role: string; status: 'active' | 'pending' | 'revoked'; reference?: string }>;
  transactionReferences: string[];
  linkedPlatforms: LinkedPlatform[];
  provisioningError?: string;
}

export type ProofFormat = 'pdf' | 'json' | 'zip';

export type ProofType = 'completion_claim' | 'verified_achievement';

export interface StepUpReceipt {
  stepUpCode: string;
  action: string;
  expiresAt: string;
}

export interface ProofDocument {
  id: string;
  achievementId: string;
  version: number;
  proofType: ProofType;
  status: 'pending' | 'anchored' | 'superseded' | 'revoked' | 'disputed' | 'failed';
  generatedAt: string;
  projectName: string;
  claimSummary: string;
  issuerDid?: string;
  verifierAttestation?: { organization: string; decision: string; timestamp: string; did?: string };
  chain?: { network: ChainNetwork; transactionReference?: string; commitment?: string; objectId?: string; explorerUrl?: string };
  registryReferences: string[];
  evidenceManifest: Array<{ name: string; sha256: string; access: 'public' | 'restricted' }>;
  events: ProvenanceEvent[];
  availableFormats: ProofFormat[];
  supersededBy?: string;
  modes: { chain: string; satellite: string; registry: string };
  sandboxNotice?: string;
  manifestHash: string;
}

// PublicProof is what /public/proofs/:id exposes: the document plus its own
// permanent URL and the checks that were run when it was published.
export interface PublicProofEnvelope {
  proof: PublicProof;
  url: string;
  verification?: ProofVerification;
}

export interface PublicProof extends Omit<ProofDocument, 'evidenceManifest'> {
  evidenceManifest: Array<{ name: string; sha256: string; access: 'public' }>;
}

export interface ProofVerification {
  overallStatus: 'valid' | 'invalid' | 'pending' | 'unavailable';
  checkedAt: string;
  checks: Array<{
    key: 'schema_signature' | 'evidence_integrity' | 'iota_commitment' | 'issuer_did' | 'verifier_attestation' | 'registry_reference' | 'supersession_dispute';
    label: string;
    status: 'passed' | 'failed' | 'pending' | 'unavailable' | 'not_applicable';
    detail: string;
  }>;
}
