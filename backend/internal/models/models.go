package models

import (
	"time"

	"gorm.io/datatypes"
)

const (
	RoleManager  = "project_manager"
	RoleMonitor  = "field_monitor"
	RoleVerifier = "independent_verifier"
	RoleBuyer    = "carbon_buyer"
	RoleFinance  = "finance_operator"
	RoleAdmin    = "platform_administrator"
)

const (
	IdentityPending = "identity_pending"
	IdentityActive  = "active"
	IdentityFailed  = "identity_failed"
)

const (
	ClaimCompletion = "achieved_unverified"
	ClaimVerified   = "achievement_verified"
)

const (
	ActivityDraft                = "draft"
	ActivityMonitoring           = "monitoring"
	ActivityCompletionClaimed    = "completion_claimed"
	ActivityAwaitingVerification = "awaiting_verification"
	ActivityVerified             = "verified"
	ActivityRejected             = "rejected"
	ActivityDisputed             = "disputed"
)

const (
	SyncSubmitted = "submitted"
	SyncConflict  = "sync_conflict"
	SyncFailed    = "failed"
)

const (
	ProofPending    = "pending"
	ProofAnchored   = "anchored"
	ProofSuperseded = "superseded"
	ProofRevoked    = "revoked"
	ProofDisputed   = "disputed"
	ProofFailed     = "failed"
)

const (
	DecisionApprove            = "approve"
	DecisionReject             = "reject"
	DecisionRequestCorrections = "request_corrections"
)

type User struct {
	ID           string    `gorm:"type:uuid;primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	DisplayName  string    `gorm:"not null" json:"name"`
	Phone        string    `json:"phone,omitempty"`
	Role         string    `gorm:"not null;index" json:"role"`
	Status       string    `gorm:"not null;index" json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// IdentityProvision is returned verbatim by the session endpoints, so its wire
// names are part of the client contract: every product field is camelCase, while
// the DID, address and reference names are the ones the clients display.
type IdentityProvision struct {
	ID                string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            string     `gorm:"type:uuid;uniqueIndex;not null" json:"userId"`
	State             string     `gorm:"not null;index" json:"state"`
	DID               string     `gorm:"column:did;index" json:"did,omitempty"`
	DIDDocumentRef    string     `gorm:"column:did_document_ref" json:"didDocumentReference,omitempty"`
	Address           string     `json:"address,omitempty"`
	KeyReferenceID    string     `gorm:"type:uuid" json:"-"`
	ProvisionTxRef    string     `json:"provisionTransactionReference,omitempty"`
	Network           string     `gorm:"not null" json:"network"`
	Attempts          int        `gorm:"not null;default:0" json:"attempts"`
	LastError         string     `json:"lastError,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	ActivatedAt       *time.Time `json:"activatedAt,omitempty"`
	LastProvisionedAt *time.Time `json:"lastProvisionedAt,omitempty"`
	// RoleCapObjectID stores the on-chain object ID of the role capability
	// issued after identity provisioning (ManagerCap, MonitorCap, VerifierCap).
	// Empty for DID-only roles (buyer, finance, admin).
	RoleCapObjectID string `gorm:"column:role_cap_object_id" json:"roleCapObjectId,omitempty"`
}

type KeyReference struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"-"`
	UserID      string     `gorm:"type:uuid;index;not null" json:"-"`
	Provider    string     `gorm:"not null" json:"-"`
	ExternalRef string     `gorm:"not null" json:"-"`
	WrappedKey  []byte     `gorm:"not null" json:"-"`
	Nonce       []byte     `gorm:"not null" json:"-"`
	PublicKey   []byte     `gorm:"not null" json:"-"`
	CreatedAt   time.Time  `json:"-"`
	RotatedAt   *time.Time `json:"-"`
	RevokedAt   *time.Time `json:"-"`
}

type LocalSession struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"-"`
	UserID    string     `gorm:"type:uuid;index;not null" json:"-"`
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"-"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
}

type ExternalIdentity struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string     `gorm:"type:uuid;uniqueIndex:idx_external_user_issuer;not null" json:"userId"`
	Issuer    string     `gorm:"uniqueIndex:idx_external_subject;uniqueIndex:idx_external_user_issuer;not null" json:"issuer"`
	Subject   string     `gorm:"uniqueIndex:idx_external_subject;not null" json:"subject"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type FederatedGrant struct {
	ID                 string     `gorm:"type:uuid;primaryKey" json:"id"`
	ExternalIdentityID string     `gorm:"type:uuid;index;not null" json:"externalIdentityId"`
	TokenHash          string     `gorm:"uniqueIndex;not null" json:"-"`
	Scope              string     `gorm:"type:text;not null" json:"scope"`
	ExpiresAt          time.Time  `gorm:"index;not null" json:"expiresAt"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type OIDCClient struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	SecretHash    string    `json:"-"`
	RedirectURIs  string    `gorm:"type:text;not null" json:"redirectUris"`
	AllowedScopes string    `gorm:"type:text;not null" json:"allowedScopes"`
	Disabled      bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TableName keeps the OIDC acronym from becoming GORM's split default table name.
func (OIDCClient) TableName() string { return "oidc_clients" }

// TableName keeps the OAuth acronym from becoming GORM's split default table name.
func (OAuthGrant) TableName() string { return "oauth_grants" }

type OAuthGrant struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string     `gorm:"type:uuid;index;not null" json:"userId"`
	ClientID  string     `gorm:"index;not null" json:"clientId"`
	Scope     string     `gorm:"type:text;not null" json:"scope"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type AuthorizationCode struct {
	ID            string     `gorm:"type:uuid;primaryKey" json:"-"`
	CodeHash      string     `gorm:"uniqueIndex;not null" json:"-"`
	UserID        string     `gorm:"type:uuid;index;not null" json:"-"`
	ClientID      string     `gorm:"index;not null" json:"-"`
	GrantID       string     `gorm:"type:uuid;index;not null" json:"-"`
	RedirectURI   string     `gorm:"type:text;not null" json:"-"`
	Scope         string     `gorm:"type:text;not null" json:"-"`
	Nonce         string     `gorm:"not null" json:"-"`
	CodeChallenge string     `gorm:"not null" json:"-"`
	ExpiresAt     time.Time  `gorm:"index;not null" json:"-"`
	UsedAt        *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"-"`
}

type AccessToken struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"-"`
	TokenHash string    `gorm:"uniqueIndex;not null" json:"-"`
	GrantID   string    `gorm:"type:uuid;index;not null" json:"-"`
	UserID    string    `gorm:"type:uuid;index;not null" json:"-"`
	ClientID  string    `gorm:"index;not null" json:"-"`
	Scope     string    `gorm:"type:text;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
	CreatedAt time.Time `json:"-"`
}

type Methodology struct {
	ID               string         `gorm:"type:uuid;primaryKey" json:"id"`
	Code             string         `gorm:"uniqueIndex;not null" json:"code"`
	Name             string         `gorm:"not null" json:"name"`
	Version          string         `gorm:"not null" json:"version"`
	RequiredEvidence string         `gorm:"type:text;not null" json:"requiredEvidence"`
	Estimation       datatypes.JSON `gorm:"type:jsonb" json:"estimation"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

type ConservationProject struct {
	ID                string    `gorm:"type:uuid;primaryKey" json:"id"`
	Name              string    `gorm:"not null" json:"name"`
	ManagerID         string    `gorm:"type:uuid;index;not null" json:"managerId"`
	MethodologyID     string    `gorm:"type:uuid;index;not null" json:"methodologyId"`
	Status            string    `gorm:"not null" json:"status"`
	CommunityGroup    string    `gorm:"column:community_group;not null;default:''" json:"communityGroup"`
	Species           string    `gorm:"not null;default:''" json:"species"`
	ConservationType  string    `gorm:"column:conservation_type;not null;default:'mangrove'" json:"conservationType"`
	CommunityShareBPS int       `gorm:"column:community_share_bps;not null" json:"communityShareBasisPoints"`
	SandboxDemo       bool      `gorm:"not null;default:false" json:"sandboxDemo"`
	JahaziEnabled     bool      `gorm:"column:jahazi_enabled;not null;default:false" json:"jahaziEnabled"`
	JahaziProjectID   string    `gorm:"column:jahazi_project_id;index" json:"jahaziProjectId,omitempty"`
	LedgerObjectID    string    `gorm:"column:ledger_object_id" json:"ledgerObjectId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Site struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID string    `gorm:"type:uuid;index;not null" json:"projectId"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Plot struct {
	ID                string  `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID         string  `gorm:"type:uuid;index;not null" json:"projectId"`
	SiteID            string  `gorm:"type:uuid;index;not null" json:"siteId"`
	Code              string  `gorm:"uniqueIndex:idx_site_plot;not null" json:"code"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	AreaHectares      string  `gorm:"type:numeric(20,6);not null" json:"areaHectares"`
	AssignedMonitorID string  `gorm:"type:uuid;index" json:"assignedMonitorId,omitempty"`
	// Boundary is the surveyed plot outline as a GeoJSON Polygon in EPSG:4326. It is
	// stored for the phase 6 precision upgrade and not read yet: the analysis cell is
	// still the square buffered around the centroid, so the sample is only as good as
	// that position, which is what GpsAccuracyM records.
	Boundary     datatypes.JSON `gorm:"type:jsonb" json:"boundary,omitempty"`
	GpsAccuracyM *float64       `json:"gpsAccuracyM,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

type ConservationActivity struct {
	ID                   string         `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID            string         `gorm:"type:uuid;index;not null" json:"projectId"`
	PlotID               string         `gorm:"type:uuid;index;not null" json:"plotId"`
	ManagerID            string         `gorm:"type:uuid;index;not null" json:"managerId"`
	ActivityType         string         `gorm:"not null" json:"activityType"`
	Status               string         `gorm:"not null;index" json:"status"`
	ReportingPeriodStart time.Time      `json:"reportingPeriodStart"`
	ReportingPeriodEnd   time.Time      `json:"reportingPeriodEnd"`
	MethodologyVersion   string         `gorm:"not null" json:"methodologyVersion"`
	Measurements         datatypes.JSON `gorm:"type:jsonb" json:"measurements"`
	CurrentEventID       string         `gorm:"type:text;default:''" json:"currentEventId,omitempty"`
	JahaziFunded         bool           `gorm:"column:jahazi_funded;not null;default:false" json:"jahaziFunded"`
	RestorationRequestID *string        `gorm:"column:restoration_request_id;type:uuid;index" json:"restorationRequestId,omitempty"`
	MrvLinkGenerated     bool           `gorm:"column:mrv_link_generated;not null;default:false" json:"mrvLinkGenerated"`
	MrvLinkPushedAt      *time.Time     `gorm:"column:mrv_link_pushed_at" json:"mrvLinkPushedAt,omitempty"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

type MonitoringRecord struct {
	ID                  string         `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID          string         `gorm:"type:uuid;index;not null" json:"activityId"`
	MonitorID           string         `gorm:"type:uuid;uniqueIndex:idx_monitor_client;index;not null" json:"monitorId"`
	ClientID            string         `gorm:"uniqueIndex:idx_monitor_client;not null" json:"clientId"`
	DeviceTimestamp     time.Time      `json:"deviceTimestamp"`
	ServerReceiptTime   time.Time      `json:"serverReceiptTime"`
	MethodologyVersion  string         `gorm:"not null" json:"methodologyVersion"`
	SchemaVersion       string         `gorm:"not null" json:"schemaVersion"`
	Latitude            float64        `json:"latitude"`
	Longitude           float64        `json:"longitude"`
	GPSAccuracyMeters   float64        `gorm:"column:gps_accuracy_meters" json:"gpsAccuracyMeters"`
	AliveCount          int            `json:"aliveCount"`
	PlantedCount        int            `json:"plantedCount"`
	AverageHeightMetres string         `gorm:"column:average_height_metres;type:numeric(20,4)" json:"averageHeightMetres"`
	Species             datatypes.JSON `gorm:"type:jsonb" json:"species"`
	Attestation         string         `gorm:"type:text;not null" json:"-"`
	AttestationHash     string         `gorm:"not null" json:"attestationHash"`
	EvidenceManifest    datatypes.JSON `gorm:"type:jsonb;not null" json:"evidenceManifest"`
	ContentCommitment   string         `gorm:"not null" json:"contentCommitment"`
	Signed              bool           `gorm:"not null;default:false" json:"signed"`
	Version             int            `gorm:"not null;default:1" json:"version"`
	SupersedesID        string         `gorm:"type:uuid" json:"supersedesId,omitempty"`
	SyncStatus          string         `gorm:"not null" json:"syncStatus"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

type EvidenceObject struct {
	ID                 string    `gorm:"type:uuid;primaryKey" json:"id"`
	MonitoringRecordID string    `gorm:"type:uuid;index;not null" json:"monitoringRecordId"`
	Kind               string    `gorm:"not null" json:"kind"`
	StorageURI         string    `gorm:"type:text;not null" json:"-"`
	SHA256             string    `gorm:"column:sha256;not null" json:"sha256"`
	MediaType          string    `gorm:"not null" json:"mediaType"`
	AccessClass        string    `gorm:"not null" json:"accessClass"`
	CapturedAt         time.Time `json:"capturedAt"`
	CreatedAt          time.Time `json:"createdAt"`
}

type SatelliteObservation struct {
	ID                string         `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID        string         `gorm:"type:uuid;index;not null" json:"activityId"`
	AdapterMode       string         `gorm:"not null" json:"adapterMode"`
	Status            string         `gorm:"not null" json:"status"`
	SceneID           string         `json:"sceneId,omitempty"`
	AcquiredAt        *time.Time     `json:"acquiredAt,omitempty"`
	ProcessingVersion string         `json:"processingVersion,omitempty"`
	Result            datatypes.JSON `gorm:"type:jsonb" json:"result"`
	ResultHash        string         `gorm:"not null" json:"resultHash"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type VerificationRequest struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID  string     `gorm:"type:uuid;index;not null" json:"activityId"`
	VerifierID  string     `gorm:"type:uuid;index;not null" json:"verifierId"`
	Status      string     `gorm:"not null;index" json:"status"`
	RequestedAt time.Time  `json:"requestedAt"`
	DecidedAt   *time.Time `json:"decidedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type VerifierAttestation struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	RequestID       string     `gorm:"type:uuid;index;not null" json:"verificationRequestId"`
	ActivityID      string     `gorm:"type:uuid;index;not null" json:"activityId"`
	VerifierID      string     `gorm:"type:uuid;index;not null" json:"verifierId"`
	IssuerDID       string     `gorm:"column:issuer_did;not null" json:"issuerDid"`
	Decision        string     `gorm:"not null" json:"decision"`
	Statement       string     `gorm:"type:text" json:"statement"`
	AttestationHash string     `gorm:"not null" json:"attestationHash"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type ProvenanceEvent struct {
	ID              string         `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID      string         `gorm:"type:uuid;index;not null" json:"activityId"`
	ClaimType       string         `gorm:"uniqueIndex:idx_activity_claim_commitment;index;not null" json:"claimType"`
	Version         int            `gorm:"not null;default:1" json:"version"`
	Commitment      string         `gorm:"uniqueIndex:idx_activity_claim_commitment;not null" json:"commitment"`
	CanonicalJSON   datatypes.JSON `gorm:"type:jsonb;not null" json:"canonicalJson"`
	PreviousEventID string         `gorm:"type:uuid;index" json:"previousEventId,omitempty"`
	IssuerDID       string         `gorm:"column:issuer_did;not null" json:"issuerDid"`
	Status          string         `gorm:"not null;index" json:"status"`
	Network         string         `gorm:"not null" json:"network"`
	TransactionRef  string         `json:"transactionReference,omitempty"`
	ObjectID        string         `json:"objectId,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	ConfirmedAt     *time.Time     `json:"confirmedAt,omitempty"`
}

type AnchorAttempt struct {
	ID                string    `gorm:"type:uuid;primaryKey" json:"id"`
	ProvenanceEventID string    `gorm:"type:uuid;index;not null" json:"provenanceEventId"`
	Attempt           int       `gorm:"not null" json:"attempt"`
	Status            string    `gorm:"not null" json:"status"`
	TransactionRef    string    `json:"transactionReference,omitempty"`
	Error             string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

type Correction struct {
	ID                 string    `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID         string    `gorm:"type:uuid;index;not null" json:"activityId"`
	PreviousEventID    string    `gorm:"type:uuid;index;not null" json:"previousEventId"`
	SupersedingEventID string    `gorm:"type:uuid;index" json:"supersedingEventId,omitempty"`
	Reason             string    `gorm:"type:text;not null" json:"reason"`
	RequestedBy        string    `gorm:"type:uuid;not null" json:"requestedBy"`
	CreatedAt          time.Time `json:"createdAt"`
}

type Dispute struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID string     `gorm:"type:uuid;index;not null" json:"activityId"`
	Status     string     `gorm:"not null" json:"status"`
	Reason     string     `gorm:"type:text;not null" json:"reason"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// StepUpToken is a single-use confirmation ticket for sensitive operations such as
// verification decisions, purchases and retirements.
type StepUpToken struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"-"`
	UserID    string     `gorm:"type:uuid;index;not null" json:"-"`
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	Action    string     `gorm:"not null" json:"action"`
	UsedAt    *time.Time `json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expiresAt"`
	CreatedAt time.Time  `json:"-"`
}

type ProjectDocument struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID string    `gorm:"type:uuid;index;not null" json:"projectId"`
	Kind      string    `gorm:"not null" json:"kind"`
	Name      string    `gorm:"not null" json:"name"`
	Status    string    `gorm:"not null" json:"status"`
	Version   string    `gorm:"not null" json:"version"`
	SHA256    string    `gorm:"column:sha256;not null" json:"sha256"`
	StoredURI string    `gorm:"type:text;not null" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SyncAction struct {
	ID          string         `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      string         `gorm:"type:uuid;uniqueIndex:idx_sync_user_client;index;not null" json:"userId"`
	ClientID    string         `gorm:"uniqueIndex:idx_sync_user_client;not null" json:"clientId"`
	ActionType  string         `gorm:"not null" json:"actionType"`
	RequestHash string         `gorm:"not null" json:"requestHash"`
	Status      string         `gorm:"not null;index" json:"status"`
	Response    datatypes.JSON `gorm:"type:jsonb" json:"response"`
	Error       string         `gorm:"type:text" json:"error,omitempty"`
	ResourceID  string         `json:"resourceId,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type AuditEvent struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:uuid;index" json:"userId"`
	Action    string    `gorm:"not null;index" json:"action"`
	Resource  string    `gorm:"not null" json:"resource"`
	RequestID string    `gorm:"index" json:"requestId"`
	Status    int       `json:"status"`
	IPAddress string    `json:"ipAddress"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProofDocument struct {
	ID             string         `gorm:"type:uuid;primaryKey" json:"id"`
	ActivityID     string         `gorm:"type:uuid;index;not null" json:"activityId"`
	Version        int            `gorm:"not null" json:"version"`
	ProofType      string         `gorm:"not null" json:"proofType"`
	Status         string         `gorm:"not null;index" json:"status"`
	Snapshot       datatypes.JSON `gorm:"type:jsonb;not null" json:"snapshot"`
	ManifestHash   string         `gorm:"not null" json:"manifestHash"`
	PDF            []byte         `gorm:"not null" json:"-"`
	QRCode         []byte         `gorm:"not null" json:"-"`
	SupersededByID string         `gorm:"type:uuid" json:"supersededById,omitempty"`
	CreatedBy      string         `gorm:"type:uuid;index;not null" json:"createdBy"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type CreditBatch struct {
	ID                 string     `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID          string     `gorm:"type:uuid;index;not null" json:"projectId"`
	RegistryIssuanceID string     `gorm:"uniqueIndex;not null" json:"registryIssuanceId"`
	RegistryName       string     `gorm:"not null" json:"registryName"`
	Vintage            int        `gorm:"not null" json:"vintage"`
	SerialStart        string     `gorm:"uniqueIndex:idx_registry_serial;not null" json:"serialStart"`
	SerialEnd          string     `gorm:"uniqueIndex:idx_registry_serial;not null" json:"serialEnd"`
	Quantity           string     `gorm:"type:numeric(30,6);not null" json:"quantity"`
	AvailableQuantity  string     `gorm:"type:numeric(30,6);not null" json:"availableQuantity"`
	Status             string     `gorm:"not null;index" json:"status"`
	AdapterMode        string     `gorm:"not null" json:"adapterMode"`
	ReconciledAt       *time.Time `json:"reconciledAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type MarketplaceListing struct {
	ID            string    `gorm:"type:uuid;primaryKey" json:"id"`
	BatchID       string    `gorm:"type:uuid;uniqueIndex;not null" json:"batchId"`
	ProjectID     string    `gorm:"type:uuid;index;not null" json:"projectId"`
	ProductType   string    `gorm:"not null" json:"productType"`
	PricePerTonne string    `gorm:"type:numeric(30,2);not null" json:"pricePerTonne"`
	Currency      string    `gorm:"size:3;not null" json:"currency"`
	Status        string    `gorm:"not null;index" json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Purchase struct {
	ID               string    `gorm:"type:uuid;primaryKey" json:"id"`
	ListingID        string    `gorm:"type:uuid;index;not null" json:"listingId"`
	BatchID          string    `gorm:"type:uuid;index;not null" json:"batchId"`
	BuyerID          string    `gorm:"type:uuid;index;not null" json:"buyerId"`
	Quantity         string    `gorm:"type:numeric(30,6);not null" json:"quantity"`
	GrossAmount      string    `gorm:"type:numeric(30,2);not null" json:"grossAmount"`
	Currency         string    `gorm:"size:3;not null" json:"currency"`
	PaymentMode      string    `gorm:"not null" json:"paymentMode"`
	PaymentReference string    `gorm:"uniqueIndex;not null" json:"paymentReference"`
	Status           string    `gorm:"not null" json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
}

type CustodyEvent struct {
	ID         string    `gorm:"type:uuid;primaryKey" json:"id"`
	BatchID    string    `gorm:"type:uuid;index;not null" json:"batchId"`
	PurchaseID string    `gorm:"type:uuid;index" json:"purchaseId,omitempty"`
	OwnerID    string    `gorm:"type:uuid;index;not null" json:"ownerId"`
	EventType  string    `gorm:"not null;index" json:"eventType"`
	Quantity   string    `gorm:"type:numeric(30,6);not null" json:"quantity"`
	CreatedAt  time.Time `json:"createdAt"`
}

type RevenueAllocation struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	PurchaseID  string    `gorm:"type:uuid;uniqueIndex:idx_purchase_recipient;index;not null" json:"purchaseId"`
	ProjectID   string    `gorm:"type:uuid;index;not null" json:"projectId"`
	Recipient   string    `gorm:"uniqueIndex:idx_purchase_recipient;not null" json:"recipient"`
	BasisPoints int       `gorm:"not null" json:"basisPoints"`
	Amount      string    `gorm:"type:numeric(30,2);not null" json:"amount"`
	Currency    string    `gorm:"size:3;not null" json:"currency"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Retirement struct {
	ID                  string     `gorm:"type:uuid;primaryKey" json:"id"`
	BatchID             string     `gorm:"type:uuid;index;not null" json:"batchId"`
	OwnerID             string     `gorm:"type:uuid;index;not null" json:"ownerId"`
	Quantity            string     `gorm:"type:numeric(30,6);not null" json:"quantity"`
	Status              string     `gorm:"not null;index" json:"status"`
	RegistryCertificate string     `json:"registryCertificate,omitempty"`
	TransactionRef      string     `json:"transactionReference,omitempty"`
	ObjectID            string     `json:"objectId,omitempty"`
	LastError           string     `gorm:"type:text" json:"lastError,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	CompletedAt         *time.Time `json:"completedAt,omitempty"`
}

type ReconciliationCase struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	BatchID    string     `gorm:"type:uuid;index;not null" json:"batchId"`
	Reason     string     `gorm:"type:text;not null" json:"reason"`
	Status     string     `gorm:"not null;index" json:"status"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type SatellitePass struct {
	ID            string     `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID     string     `gorm:"type:uuid;index;not null" json:"project_id"`
	ActivityID    string     `gorm:"type:uuid;index" json:"activity_id,omitempty"`
	Mission       string     `gorm:"not null;index" json:"mission"`
	PassType      string     `gorm:"not null;index" json:"pass_type"`
	WindowStart   time.Time  `gorm:"not null" json:"window_start"`
	WindowEnd     time.Time  `gorm:"not null" json:"window_end"`
	AcquiredAt    *time.Time `json:"acquired_at,omitempty"`
	SceneID       string     `json:"scene_id,omitempty"`
	Orbit         string     `json:"orbit,omitempty"`
	Tile          string     `json:"tile,omitempty"`
	CloudCoverPct *float64   `json:"cloud_cover_pct,omitempty"`
	CoveragePct   float64    `json:"coverage_pct"`
	StatusDetail  string     `gorm:"type:text" json:"status_detail"`
	AdapterMode   string     `gorm:"not null" json:"adapter_mode"`
	ResultHash    string     `json:"result_hash,omitempty"`
	// Provider names the catalog that supplied the item, because the two differ in how
	// much provenance they expose: a CDSE granule name encodes orbit and processing
	// baseline, an earth-search item does not.
	Provider string `gorm:"type:text;not null;default:''" json:"provider,omitempty"`
	// NdviMean is this pass's own figure for the project, so a reader can see the
	// scatter between passes rather than only the window mean.
	NdviMean          *float64       `json:"ndvi_mean,omitempty"`
	Footprint         datatypes.JSON `gorm:"type:jsonb" json:"footprint_geojson,omitempty"`
	ProcessingVersion string         `gorm:"type:text;not null;default:''" json:"processing_version,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// ProjectNdviSummary is one computation of a project's area-weighted NDVI mean and the
// change against the same window a year earlier. Rows are appended, never updated: a
// figure that was anchored keeps the value it was anchored with, and the hash is over
// the result payload, so a re-run that changes the number changes the commitment.
type ProjectNdviSummary struct {
	ID             string    `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID      string    `gorm:"type:uuid;not null" json:"projectId"`
	PeriodStart    time.Time `gorm:"not null" json:"periodStart"`
	PeriodEnd      time.Time `gorm:"not null" json:"periodEnd"`
	CurrentSceneID string    `gorm:"type:text;not null" json:"currentSceneId"`
	// CurrentAcquiredAt is the granule's own timestamp, not the moment it was read. It
	// and MeanNDVI are nil when the computation found nothing usable: NDVI 0.0 is a real
	// reading for bare substrate, so a zero cannot stand in for an absent figure.
	CurrentAcquiredAt  *time.Time     `json:"currentAcquiredAt,omitempty"`
	BaselineSceneID    string         `gorm:"type:text;not null;default:''" json:"baselineSceneId,omitempty"`
	BaselineAcquiredAt *time.Time     `json:"baselineAcquiredAt,omitempty"`
	MeanNDVI           *float64       `gorm:"type:numeric(6,4)" json:"meanNdvi,omitempty"`
	BaselineMeanNDVI   *float64       `gorm:"type:numeric(6,4)" json:"baselineMeanNdvi,omitempty"`
	ChangeAbs          *float64       `gorm:"type:numeric(6,4)" json:"changeAbs,omitempty"`
	ChangePct          *float64       `gorm:"type:numeric(8,2)" json:"changePct,omitempty"`
	CoveragePct        float64        `gorm:"type:numeric(6,2);not null" json:"coveragePct"`
	CurrentCloudPct    *float64       `gorm:"type:numeric(6,2)" json:"currentCloudPct,omitempty"`
	PlotCount          int            `gorm:"not null" json:"plotCount"`
	Quality            string         `gorm:"type:text;not null" json:"quality"`
	AdapterMode        string         `gorm:"type:text;not null" json:"adapterMode"`
	Provider           string         `gorm:"type:text;not null;default:''" json:"provider,omitempty"`
	MethodologyVersion string         `gorm:"type:text;not null;default:''" json:"methodologyVersion,omitempty"`
	Result             datatypes.JSON `gorm:"type:jsonb;not null" json:"result"`
	ResultHash         string         `gorm:"type:text;not null" json:"resultHash"`
	ProcessingVersion  string         `gorm:"type:text;not null" json:"processingVersion"`
	// TileMinZoom and TileMaxZoom are the range the worker painted for this row. They
	// are nil when no render ran (sandbox, no plots, or the worker refused), so a tile
	// proxy that reads nil answers 410 instead of guessing a range.
	TileMinZoom *int `json:"tileMinZoom,omitempty"`
	TileMaxZoom *int `json:"tileMaxZoom,omitempty"`
	ComputedAt  time.Time `gorm:"not null" json:"computedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SatelliteRefreshJob is one queued compute. The database, not a map in memory, holds
// the one-open-job-per-project rule, so a restart cannot strand a project with a
// running job nothing will ever finish.
type SatelliteRefreshJob struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID   string     `gorm:"type:uuid;not null" json:"projectId"`
	Status      string     `gorm:"type:text;not null;index" json:"status"`
	RequestedBy string     `gorm:"type:text;not null;default:''" json:"requestedBy,omitempty"`
	Reason      string     `gorm:"type:text;not null;default:''" json:"reason,omitempty"`
	Error       string     `gorm:"type:text;not null;default:''" json:"error,omitempty"`
	SummaryID   string     `gorm:"type:text;not null;default:''" json:"summaryId,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type FinancialImport struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	Source      string    `gorm:"uniqueIndex:idx_financial_source;not null" json:"source"`
	ExternalID  string    `gorm:"uniqueIndex:idx_financial_source;not null" json:"externalId"`
	ProjectID   string    `gorm:"type:uuid;index;not null" json:"projectId"`
	Amount      string    `gorm:"type:numeric(30,2);not null" json:"amount"`
	Currency    string    `gorm:"size:3;not null" json:"currency"`
	OccurredAt  time.Time `json:"occurredAt"`
	PayloadHash string    `gorm:"not null" json:"payloadHash"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SystemConfig struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	Key         string    `gorm:"uniqueIndex;not null" json:"key"`
	Value       string    `gorm:"type:text;not null" json:"value"`
	Type        string    `gorm:"not null" json:"type"`
	Description string    `json:"description,omitempty"`
	UpdatedBy   string    `gorm:"type:uuid" json:"updatedBy,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Alert struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	Scope      string     `gorm:"not null;index" json:"scope"`
	Severity   string     `gorm:"not null;index" json:"severity"`
	Type       string     `gorm:"not null;index" json:"type"`
	Title      string     `gorm:"not null" json:"title"`
	Detail     string     `gorm:"type:text" json:"detail"`
	Action     string     `json:"action,omitempty"`
	Status     string     `gorm:"not null;index" json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy string     `gorm:"type:uuid" json:"resolvedBy,omitempty"`
}

type Incident struct {
	ID           string         `gorm:"type:uuid;primaryKey" json:"id"`
	Type         string         `gorm:"not null;index" json:"type"`
	Status       string         `gorm:"not null;index" json:"status"`
	StartedAt    time.Time      `json:"startedAt"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
	DurationMs   int64          `json:"durationMs"`
	Error        string         `gorm:"type:text" json:"error,omitempty"`
	RetryCount   int            `json:"retryCount"`
	ResourceType string         `json:"resourceType,omitempty"`
	ResourceID   string         `json:"resourceId,omitempty"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
}

type AdminNote struct {
	ID         string    `gorm:"type:uuid;primaryKey" json:"id"`
	TargetType string    `gorm:"not null;index" json:"targetType"`
	TargetID   string    `gorm:"not null;index" json:"targetId"`
	AuthorID   string    `gorm:"type:uuid;not null" json:"authorId"`
	Body       string    `gorm:"type:text;not null" json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ExportJob struct {
	ID          string         `gorm:"type:uuid;primaryKey" json:"id"`
	Requester   string         `gorm:"type:uuid;not null" json:"requester"`
	Kind        string         `gorm:"not null" json:"kind"`
	Filters     datatypes.JSON `gorm:"type:jsonb" json:"filters,omitempty"`
	Status      string         `gorm:"not null;index" json:"status"`
	FileRef     string         `json:"fileRef,omitempty"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

type SavedView struct {
	ID        string         `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"userId"`
	Page      string         `gorm:"not null" json:"page"`
	Name      string         `gorm:"not null" json:"name"`
	Filters   datatypes.JSON `gorm:"type:jsonb" json:"filters,omitempty"`
	Shared    bool           `gorm:"not null;default:false" json:"shared"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type VerificationOverride struct {
	ID            string    `gorm:"type:uuid;primaryKey" json:"id"`
	RequestID     string    `gorm:"type:uuid;not null;index" json:"requestId"`
	AdminID       string    `gorm:"type:uuid;not null" json:"adminId"`
	Reason        string    `gorm:"type:text;not null" json:"reason"`
	StepUpReceipt string    `json:"stepUpReceipt,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type PublicProofVerificationLog struct {
	ID           string    `gorm:"type:uuid;primaryKey" json:"id"`
	ProofID      string    `gorm:"type:uuid;not null;index" json:"proofId"`
	CheckedAt    time.Time `json:"checkedAt"`
	Overall      string    `gorm:"not null" json:"overall"`
	FailedChecks string    `gorm:"type:text" json:"failedChecks,omitempty"`
	IPHash       string    `json:"ipHash,omitempty"`
}

// ── Jahazi cross-platform integration ─────────────────────────────────────────

// RestorationClaimStatus constants for the RestorationRequest lifecycle.
const (
	RestorationClaimOpen               = "open"
	RestorationClaimMatched            = "matched"
	RestorationClaimFulfilled          = "fulfilled"
	RestorationClaimPartiallyFulfilled = "partially_fulfilled"
	RestorationClaimExpired            = "expired"
	RestorationClaimCancelled          = "cancelled"
)

// RestorationRequest is an explicit claim from Jahazi that tourist-funded
// restoration is expected for a project. The PM's planting activity is matched
// against open requests — only matched activities generate MRV links and
// trigger the certificate loop. Normal project activities never touch Jahazi.
type RestorationRequest struct {
	ID                  string         `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID           string         `gorm:"type:uuid;index;not null" json:"projectId"`
	JahaziClaimID       string         `gorm:"uniqueIndex;not null" json:"jahaziClaimId"`
	JahaziProjectID     string         `gorm:"not null" json:"jahaziProjectId"`
	PeriodStart         time.Time      `json:"periodStart"`
	PeriodEnd           time.Time      `json:"periodEnd"`
	TotalCostUsd        string         `gorm:"type:numeric(30,2);not null" json:"totalCostUsd"`
	TotalUnits          int            `gorm:"not null" json:"totalUnits"`
	Species             string         `gorm:"not null" json:"species"`
	ContributorCount    int            `gorm:"not null" json:"contributorCount"`
	ContributorJSON     datatypes.JSON `gorm:"column:contributor_breakdown;type:jsonb;not null" json:"contributorBreakdown"`
	Nonce               string         `gorm:"uniqueIndex;not null" json:"-"`
	Signature           string         `gorm:"not null" json:"-"`
	Status              string         `gorm:"not null;index" json:"status"`
	MatchedActivityID   string         `gorm:"type:uuid;index" json:"matchedActivityId,omitempty"`
	FulfilledUnits      int            `gorm:"default:0" json:"fulfilledUnits"`
	ExpiresAt           time.Time      `gorm:"index" json:"expiresAt"`
	ChainAnchorObjectID string         `gorm:"column:chain_anchor_object_id" json:"chainAnchorObjectId,omitempty"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

// JahaziContribution mirrors a tourist contribution from Jahazi into the
// BahariMRV wallet ledger, recorded as a FinancialImport for auditability.
type JahaziContribution struct {
	JahaziContributionID string `json:"jahaziContributionId"`
	JahaziProjectID      string `json:"jahaziProjectId"`
	TouristDID           string `json:"touristId"`
	AmountUsd            string `json:"amountUsd"`
	SeedlingsEquivalent  int    `json:"seedlingsEquivalent"`
	ContributedAt        string `json:"contributedAt"`
	Nonce                string `json:"nonce"`
	Signature            string `json:"signature"`
}
