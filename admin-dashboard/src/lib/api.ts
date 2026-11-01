import type { Overview, User, IdentityProvision, Project, Activity, VerificationRequest, AnchorAttempt, ProofDocument, AuditEvent, HealthReport } from '../types/api';

const API_ROOT = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '') + '/api/v1';

function token() { return localStorage.getItem('admin_token'); }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json', ...(init?.headers as any) };
  const t = token();
  if (t) headers.Authorization = `Bearer ${t}`;
  if (init?.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
  const res = await fetch(`${API_ROOT}${path}`, { ...init, headers });
  if (!res.ok) throw new Error(`Request failed: ${res.status}`);
  return res.json() as Promise<T>;
}

export const api = {
  getHealth: () => request<HealthReport>('/health'),
  getOverview: () => request<Overview>('/admin/overview'),
  getUsers: () => request<{ items: User[] }>('/admin/users'),
  getIdentities: () => request<{ items: IdentityProvision[] }>('/admin/identities'),
  getProjects: () => request<{ items: Project[] }>('/admin/projects'),
  getActivities: () => request<{ items: Activity[] }>('/admin/activities'),
  getVerificationRequests: () => request<{ items: VerificationRequest[] }>('/admin/verification-requests'),
  getAnchors: () => request<{ items: AnchorAttempt[] }>('/admin/anchors'),
  getProofs: () => request<{ items: ProofDocument[] }>('/admin/proofs'),
  getAuditEvents: () => request<{ items: AuditEvent[] }>('/admin/audit-events'),
  getDisputes: () => request<{ items: any[] }>('/admin/disputes'),
  getCorrections: () => request<{ items: any[] }>('/admin/corrections'),
  getProvenanceEvents: () => request<{ items: any[] }>('/admin/provenance-events'),
  getCreditBatches: () => request<{ items: any[] }>('/admin/credit-batches'),
  getListings: () => request<{ items: any[] }>('/admin/marketplace/listings'),
  getPurchases: () => request<{ items: any[] }>('/admin/marketplace/purchases'),
  getRetirements: () => request<{ items: any[] }>('/admin/retirements'),
  getFinancialImports: () => request<{ items: any[] }>('/admin/finance/imports'),
  getSatellitePasses: () => request<{ items: any[] }>('/admin/satellite/passes'),
  getSyncActions: () => request<{ items: any[] }>('/admin/sync-actions'),
  getMethodologies: () => request<{ items: any[] }>('/admin/methodologies'),
  search: (q: string) => request<any>(`/admin/search?q=${encodeURIComponent(q)}`),
  disableUser: (id: string, password: string) => request<any>(`/admin/users/${id}/disable`, { method: 'POST', body: JSON.stringify({ password }) } as any),
  enableUser: (id: string, password: string) => request<any>(`/admin/users/${id}/enable`, { method: 'POST', body: JSON.stringify({ password }) } as any),
  retryAnchor: (id: string, password: string) => request<any>(`/admin/anchors/${id}/retry`, { method: 'POST', body: JSON.stringify({ password }) } as any),
  getConfig: () => request<{ items: any[] }>('/admin/config'),
  patchConfig: (key: string, value: string, password: string) => request<any>(`/admin/config/${encodeURIComponent(key)}`, { method: 'PATCH', body: JSON.stringify({ value, password }) } as any),
  getAlerts: () => request<{ items: any[] }>('/admin/alerts'),
  patchAlert: (id: string, status: string, password: string) => request<any>(`/admin/alerts/${id}`, { method: 'PATCH', body: JSON.stringify({ status, password }) } as any),
  getExports: () => request<{ items: any[] }>('/admin/exports'),
  createExport: (kind: string, password: string) => request<any>('/admin/exports', { method: 'POST', body: JSON.stringify({ kind, password }) } as any),
  getEvidence: () => request<{ items: any[] }>('/admin/evidence'),
  getDataQuality: () => request<any>('/admin/data-quality'),
  getIntegrations: () => request<{ modes: Record<string, string>; serverTime: string }>('/admin/integrations'),
};
