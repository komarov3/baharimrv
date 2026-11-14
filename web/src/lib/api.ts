import type {
  ActivityProvenance,
  ActivitySummary,
  AnchoredEvent,
  AnchorStatus,
  AuthSession,
  DecisionReceipt,
  DeltaPage,
  HealthReport,
  IdentityProfile,
  IdentityProvision,
  LinkedPlatform,
  MarketplaceListing,
  MethodologyTable,
  MonitoringRecordPayload,
  MonitoringRecordResult,
  PlotInput,
  PlotReceipt,
  ProjectDashboard,
  ProjectPlanner,
  ProjectTable,
  ProjectWallet,
  ProofDocument,
  ProofFormat,
  ProofTable,
  PublicProofEnvelope,
  ProofVerification,
  PurchaseResult,
  Reconciliation,
  RetirementResult,
  StepUpAction,
  StepUpReceipt,
  SyncBatchResponse,
  VerificationQueue,
  VerificationRequest,
} from '../types/api';

const API_ROOT = '/api/v1';

export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly requestId?: string;
  readonly details?: unknown;

  constructor(message: string, status: number, options: { code?: string; requestId?: string; details?: unknown } = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = options.code;
    this.requestId = options.requestId;
    this.details = options.details;
  }
}

interface ErrorEnvelope {
  message?: string;
  error?: string | { message?: string; code?: string; details?: unknown };
  code?: string;
  requestId?: string;
  details?: unknown;
}

function parseError(body: ErrorEnvelope | null, status: number, statusText: string): ApiError {
  const nested = typeof body?.error === 'object' ? body.error : undefined;
  const message = body?.message ?? nested?.message ?? (typeof body?.error === 'string' ? body.error : undefined);
  return new ApiError(message || `Request failed (${status} ${statusText}). Try again.`, status, {
    code: body?.code ?? nested?.code,
    requestId: body?.requestId,
    details: body?.details ?? nested?.details,
  });
}

export interface SessionHandlers {
  onRenewed: (session: AuthSession) => void;
  onExpired: () => void;
}

// A 401 on these means bad credentials or a dead grant, never an expired
// access token, so they must not trigger a refresh attempt.
function isAuthEntry(path: string): boolean {
  return path === '/auth/login' || path === '/auth/register' || path === '/auth/refresh' || path.startsWith('/auth/federation/');
}

export class ApiClient {
  private token?: string;
  private refreshToken?: string;
  private refreshInFlight?: Promise<boolean>;
  private sessionHandlers?: SessionHandlers;

  constructor(private readonly baseUrl = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '')) {}

  setToken(token?: string) {
    this.token = token;
  }

  setRefreshToken(token?: string) {
    this.refreshToken = token;
  }

  setSessionHandlers(handlers?: SessionHandlers) {
    this.sessionHandlers = handlers;
  }

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    if (this.token) headers.set('Authorization', `Bearer ${this.token}`);
    if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
    headers.set('Accept', 'application/json');

    const response = await this.fetchWithRefresh(path, init, headers);
    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as ErrorEnvelope | null;
      throw parseError(body, response.status, response.statusText);
    }
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  }

  // One retry after a successful refresh; a second 401 surfaces to the caller.
  private async fetchWithRefresh(path: string, init: RequestInit, headers: Headers, retried = false): Promise<Response> {
    let response: Response;
    try {
      response = await fetch(`${this.baseUrl}${API_ROOT}${path}`, {
        ...init,
        headers,
        credentials: 'include',
      });
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') throw error;
      throw new ApiError('The BahariMRV service could not be reached. Check your connection and try again.', 0, {
        code: 'network_unavailable',
        details: error,
      });
    }
    if (!retried && response.status === 401 && this.refreshToken && !isAuthEntry(path)) {
      if (await this.refreshOnce()) {
        if (this.token) headers.set('Authorization', `Bearer ${this.token}`);
        return this.fetchWithRefresh(path, init, headers, true);
      }
    }
    return response;
  }

  // Concurrent 401s share one rotation: the grant is single-use, so a second
  // parallel refresh would present a spent grant and revoke the family.
  private refreshOnce(): Promise<boolean> {
    this.refreshInFlight ??= this.performRefresh().finally(() => {
      this.refreshInFlight = undefined;
    });
    return this.refreshInFlight;
  }

  private async performRefresh(): Promise<boolean> {
    const grant = this.refreshToken;
    if (!grant) return false;
    let response: Response;
    try {
      response = await fetch(`${this.baseUrl}${API_ROOT}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ refreshToken: grant }),
        credentials: 'include',
      });
    } catch {
      return false; // offline: keep the session, the next call can try again
    }
    if (response.status === 401 || response.status === 403) {
      this.token = undefined;
      this.refreshToken = undefined;
      this.sessionHandlers?.onExpired();
      return false;
    }
    if (!response.ok) return false;
    const session = (await response.json().catch(() => null)) as AuthSession | null;
    if (!session?.sessionToken) {
      this.token = undefined;
      this.refreshToken = undefined;
      this.sessionHandlers?.onExpired();
      return false;
    }
    this.token = session.sessionToken;
    this.refreshToken = session.refreshToken;
    this.sessionHandlers?.onRenewed(session);
    return true;
  }

  private get<T>(path: string, signal?: AbortSignal): Promise<T> {
    return this.request<T>(path, { signal });
  }

  private post<T>(path: string, body: unknown, headers?: HeadersInit): Promise<T> {
    return this.request<T>(path, { method: 'POST', body: body instanceof FormData ? body : JSON.stringify(body), headers });
  }

  // A fresh key per call makes an accidental retry safe; a deliberate retry
  // reuses the caller's key so the server can replay its original receipt.
  private idempotent<T>(path: string, body: Record<string, unknown>, idempotencyKey?: string): Promise<T> {
    return this.post<T>(path, { ...body, idempotencyKey: idempotencyKey ?? crypto.randomUUID() });
  }

  // Health names the adapter modes the server is actually running, so the UI
  // can label sandbox behaviour instead of asserting a live network.
  getHealth(signal?: AbortSignal) {
    return this.get<HealthReport>('/health', signal);
  }

  login(email: string, password: string) {
    return this.post<AuthSession>('/auth/login', { email, password });
  }

  register(input: {
    displayName: string;
    email: string;
    password: string;
    phone?: string;
    role: 'project_manager' | 'field_monitor' | 'independent_verifier' | 'carbon_buyer';
  }) {
    return this.post<AuthSession>('/auth/register', input);
  }

  /**
   * The issuer travels in the body and the backend resolves it against its own
   * configured clients, so a deployment whose issuer host is not literally
   * "jahazi" (localhost in development) still federates.
   */
  exchangeJahazi(input: { issuer: string; code: string; codeVerifier: string; nonce: string; redirectUri: string }) {
    return this.post<AuthSession>('/auth/federation/exchange', input);
  }

  linkJahazi(input: { issuer: string; code: string; codeVerifier: string; nonce: string; redirectUri: string }) {
    return this.post<{ issuer: string; subject: string; status: 'linked' }>('/auth/federation/link', input);
  }

  logout() {
    return this.request<void>('/auth/logout', { method: 'POST', body: '{}' });
  }

  getLinks(signal?: AbortSignal) {
    return this.get<{ links: LinkedPlatform[] }>('/auth/links', signal);
  }

  unlink(linkId: string) {
    return this.request<void>(`/auth/links/${encodeURIComponent(linkId)}`, { method: 'DELETE' });
  }

  stepUp(password: string, action: StepUpAction) {
    return this.post<StepUpReceipt>('/auth/step-up', { password, action });
  }

  getProjects(signal?: AbortSignal) {
    return this.get<ProjectTable>('/projects', signal);
  }
  getMethodologies(signal?: AbortSignal) {
    return this.get<MethodologyTable>('/methodologies', signal);
  }

  createProject(input: ProjectPlanner) {
    return this.post<{ id: string; name: string }>('/projects', input);
  }

  getDashboard(projectId: string, signal?: AbortSignal) {
    return this.get<ProjectDashboard>(`/projects/${encodeURIComponent(projectId)}/dashboard`, signal);
  }

  addPlot(projectId: string, input: PlotInput) {
    return this.post<PlotReceipt>(`/projects/${encodeURIComponent(projectId)}/plots`, input);
  }

  createActivity(projectId: string, input: {
    plotId: string;
    activityType: string;
    methodologyVersion: string;
    reportingPeriodStart: string;
    reportingPeriodEnd: string;
  }) {
    return this.post<ActivitySummary>(`/projects/${encodeURIComponent(projectId)}/activities`, input);
  }

  getDocuments(projectId: string, signal?: AbortSignal) {
    return this.get<{ documents: ActivityProvenance['documents'] }>(`/projects/${encodeURIComponent(projectId)}/documents`, signal);
  }

  syncBatch(records: MonitoringRecordPayload[]) {
    return this.post<SyncBatchResponse>('/sync/batch', { actions: records.map((record) => ({ type: 'monitoring_record', record })) });
  }

  syncDelta(since: string, signal?: AbortSignal) {
    return this.get<DeltaPage>(`/sync/delta?since=${encodeURIComponent(since)}`, signal);
  }

  createMonitoringRecord(activityId: string, record: MonitoringRecordPayload) {
    return this.post<MonitoringRecordResult>(`/activities/${encodeURIComponent(activityId)}/monitoring-records`, record);
  }

  uploadEvidence(recordId: string, input: { file: Blob; name: string; kind: 'photo' | 'voice'; sha256: string }) {
    const data = new FormData();
    data.append('file', input.file, input.name);
    data.append('kind', input.kind);
    data.append('sha256', input.sha256);
    return this.post<{ id: string; sha256: string; status: string }>(`/monitoring-records/${encodeURIComponent(recordId)}/evidence`, data);
  }

  getProvenance(activityId: string, signal?: AbortSignal) {
    return this.get<ActivityProvenance>(`/activities/${encodeURIComponent(activityId)}/provenance`, signal);
  }

  // Completion is the manager's attestation only; the verified milestone
  // arrives later from an independent verifier decision. Repeating it is
  // rejected by the activity's own closed state, not silently re-anchored.
  completeActivity(activityId: string) {
    return this.post<{ activity: ActivitySummary; event: AnchoredEvent }>(`/activities/${encodeURIComponent(activityId)}/complete`, {});
  }

  retryAnchor(activityId: string) {
    return this.post<{ status: AnchorStatus['status']; event: AnchoredEvent }>(`/activities/${encodeURIComponent(activityId)}/anchor/retry`, {});
  }

  getAnchorStatus(eventId: string, signal?: AbortSignal) {
    return this.get<AnchorStatus>(`/anchors/${encodeURIComponent(eventId)}/status`, signal);
  }

  submitVerification(activityId: string, verifierId: string) {
    return this.post<VerificationRequest>(`/activities/${encodeURIComponent(activityId)}/submit-verification`, { verifierId });
  }

  getVerificationQueue(signal?: AbortSignal) {
    return this.get<VerificationQueue>('/verification-requests', signal);
  }

  submitDecision(requestId: string, input: {
    decision: 'approve' | 'reject' | 'request_corrections';
    notes: string;
    methodologyVersion: string;
    stepUpCode?: string;
    idempotencyKey: string;
  }) {
    return this.post<DecisionReceipt>(`/verification-requests/${encodeURIComponent(requestId)}/decision`, input);
  }

  getListings(signal?: AbortSignal) {
    return this.get<{ listings: MarketplaceListing[] }>('/marketplace/listings', signal);
  }

  purchase(input: { listingId: string; quantityTonnes: string; stepUpCode?: string; idempotencyKey?: string }) {
    return this.idempotent<PurchaseResult>('/marketplace/purchases', input, input.idempotencyKey);
  }

  retire(batchId: string, input: { quantityTonnes: string; reason: string; stepUpCode?: string; idempotencyKey?: string }) {
    return this.idempotent<RetirementResult>(`/credit-batches/${encodeURIComponent(batchId)}/retire`, input, input.idempotencyKey);
  }

  getWallet(projectId: string, signal?: AbortSignal) {
    return this.get<ProjectWallet>(`/projects/${encodeURIComponent(projectId)}/wallet`, signal);
  }

  getReconciliation(projectId: string, signal?: AbortSignal) {
    return this.get<Reconciliation>(`/projects/${encodeURIComponent(projectId)}/reconciliation`, signal);
  }

  getIdentity(signal?: AbortSignal) {
    return this.get<IdentityProfile>('/users/me/identity', signal);
  }

  retryIdentity() {
    return this.request<IdentityProfile>('/identity/provision/retry', { method: 'POST', body: '{}' });
  }

  recoverIdentity(password: string) {
    return this.post<{ identity: IdentityProvision; message?: string }>('/users/me/identity/recover', { password });
  }

  rotateIdentity(password: string) {
    return this.post<{ identity: IdentityProvision; message?: string }>('/users/me/identity/rotate', { password });
  }

  listProofs(achievementId: string, signal?: AbortSignal) {
    return this.get<ProofTable>(`/achievements/${encodeURIComponent(achievementId)}/proofs`, signal);
  }

  // The proof type is decided by the achievement's own state, never chosen here.
  generateProof(achievementId: string, idempotencyKey?: string) {
    return this.idempotent<ProofDocument>(`/achievements/${encodeURIComponent(achievementId)}/proofs`, {}, idempotencyKey);
  }

  getPublicProof(proofId: string, signal?: AbortSignal) {
    return this.get<PublicProofEnvelope>(`/public/proofs/${encodeURIComponent(proofId)}`, signal);
  }

  verifyPublicProof(proofId: string) {
    return this.post<ProofVerification>(`/public/proofs/${encodeURIComponent(proofId)}/verify`, {});
  }

  async downloadProof(proofId: string, format: ProofFormat): Promise<Blob> {
    const headers = new Headers();
    if (this.token) headers.set('Authorization', `Bearer ${this.token}`);
    const response = await this.fetchWithRefresh(
      `/proofs/${encodeURIComponent(proofId)}/download?format=${format}`,
      {},
      headers,
    );
    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as ErrorEnvelope | null;
      throw parseError(body, response.status, response.statusText);
    }
    return response.blob();
  }
}

export const api = new ApiClient();
