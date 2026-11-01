export interface User { id: string; name: string; email: string; role: string; status: string; createdAt?: string; }
export interface IdentityProvision { id: string; userId: string; state: string; did?: string; address?: string; network: string; attempts: number; lastError?: string; createdAt: string; }
export interface Project { id: string; name: string; communityGroup?: string; conservationType?: string; createdAt: string; }
export interface Activity { id: string; projectId: string; status: string; createdAt: string; }
export interface VerificationRequest { id: string; activityId: string; status: string; verifierId: string; createdAt: string; }
export interface AnchorAttempt { id: string; status: string; commitmentHash?: string; transactionReference?: string; createdAt: string; }
export interface ProofDocument { id: string; activityId: string; proofType: string; status: string; version: number; createdAt: string; }
export interface AuditEvent { id: string; userId: string; action: string; resource: string; status: number; createdAt: string; }
export interface Overview { users: number; activeUsers: number; pendingIdentities: number; failedIdentities: number; projects: number; activities: number; verifiedActivities: number; verificationQueue: number; anchorsPending: number; anchorsFailed: number; proofs: number; auditEvents: number; }
export interface ServiceModes { chain: string; satellite: string; registry: string; payment: string; }
export interface HealthReport { status: 'ok'; modes: ServiceModes; serverTime: string; }
