package ke.co.baharilink.baharimrv.data.remote

import okhttp3.MultipartBody
import okhttp3.RequestBody
import okhttp3.ResponseBody
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.Part
import retrofit2.http.Path
import retrofit2.http.Query
import retrofit2.http.Streaming

/**
 * The BahariMRV HTTP contract. Field names mirror the server's camelCase JSON
 * exactly, so a rename on either side shows up as a missing value rather than a
 * silent default. Amounts are strings: the server sends decimal strings and this
 * client must never turn them into floats.
 */

// ── shared ────────────────────────────────────────────────────────────────

data class AmountDto(
    val amount: String,
    val currency: String,
)

data class ModesDto(
    val chain: String,
    val satellite: String,
    val registry: String,
    val payment: String,
)

data class HealthDto(
    val status: String,
    val serverTime: String,
    val modes: ModesDto,
)

// ── auth and session ──────────────────────────────────────────────────────

data class LoginRequest(val email: String, val password: String)

data class RegisterRequest(
    val email: String,
    val password: String,
    val displayName: String,
    val phone: String? = null,
    val role: String,
)

data class AuthUserDto(
    val id: String,
    val email: String,
    val name: String,
    val phone: String? = null,
    val role: String,
    val status: String,
)

/** Mirrors the server's identity provision record; `state` drives every custody label. */
data class ProvisionedIdentityDto(
    val id: String,
    val userId: String,
    val state: String,
    val did: String? = null,
    val didDocumentReference: String? = null,
    val address: String? = null,
    val provisionTransactionReference: String? = null,
    val network: String = "",
    val attempts: Int = 0,
    val lastError: String? = null,
    val activatedAt: String? = null,
)

/**
 * `sessionToken` is absent when the server answers 202 while IOTA provisioning is
 * still running, so it stays nullable rather than pretending to always be issued.
 */
data class LoginResponse(
    val user: AuthUserDto,
    val identity: ProvisionedIdentityDto,
    val sessionToken: String? = null,
    val expiresAt: String? = null,
    val message: String? = null,
)

data class MeResponse(val user: AuthUserDto, val identity: ProvisionedIdentityDto)

/** `retryIdentity` returns the whole profile, so the screen can refresh in one call. */
data class IdentityResponse(val identity: ProvisionedIdentityDto, val message: String? = null)

/**
 * A step-up ticket is minted from the password and bound to one action. It is
 * single-use, so every gated call carries a freshly minted code.
 */
data class StepUpRequest(val password: String, val action: String)

data class StepUpResponse(
    val stepUpCode: String,
    val action: String,
    val expiresAt: String,
)

// ── projects and dashboard ────────────────────────────────────────────────

data class ProjectSummaryDto(
    val total: Int = 0,
    val active: Int = 0,
    val pendingVerification: Int = 0,
    val achievedUnverified: Int = 0,
)

data class ProjectRowDto(
    val id: String,
    val name: String,
    val communityGroup: String? = null,
    val site: String? = null,
    val species: String? = null,
    val conservationType: String? = null,
    val activityType: String? = null,
    val status: String,
    val verificationState: String? = null,
    val proofAvailable: Boolean = false,
    val sandboxDemo: Boolean = false,
    val managerName: String? = null,
    val integrationNote: String? = null,
)

data class ProjectsResponse(
    val rows: List<ProjectRowDto> = emptyList(),
    val summary: ProjectSummaryDto? = null,
)

data class ProjectHeaderDto(
    val id: String,
    val name: String,
    val communityGroup: String? = null,
    val species: String? = null,
    val conservationType: String? = null,
    val siteCount: Int = 0,
    val methodology: String? = null,
    val methodologyPeriod: String? = null,
)

data class PlotDto(
    val id: String,
    val label: String,
    val latitude: Double,
    val longitude: Double,
    val areaHectares: String? = null,
    val activityType: String? = null,
    val status: String,
    val chainState: String? = null,
    val ndvi: Double? = null,
)

data class NdviDto(
    val status: String,
    val source: String? = null,
    val acquiredAt: String? = null,
)

/**
 * `status` carries the achievement state the metric was measured under, so a
 * survival rate computed from unverified records cannot read as verified.
 */
data class MetricDto(
    val key: String,
    val label: String,
    val value: String,
    val unit: String? = null,
    val status: String? = null,
    val methodologyPeriod: String? = null,
)

data class AlertDto(
    val id: String,
    val severity: String,
    val title: String,
    val detail: String? = null,
)

data class DashboardDto(
    val project: ProjectHeaderDto,
    val plots: List<PlotDto> = emptyList(),
    val ndvi: NdviDto? = null,
    val chainMode: String? = null,
    val metrics: List<MetricDto> = emptyList(),
    val alerts: List<AlertDto> = emptyList(),
)

// ── project management ────────────────────────────────────────────────────

data class MethodologyDto(
    val id: String,
    val code: String,
    val name: String,
    val version: String,
)

data class MethodologiesResponse(
    val items: List<MethodologyDto> = emptyList(),
)

data class CreatePlotRequestDto(
    val siteName: String,
    val code: String,
    val latitude: Double,
    val longitude: Double,
    val areaHectares: String,
    val assignedMonitorId: String? = null,
)

/** Request to create a planting activity bound to one plot and methodology. */
data class CreateActivityRequestDto(
    val plotId: String,
    val activityType: String,
    val methodologyVersion: String,
    val reportingPeriodStart: String? = null,
    val reportingPeriodEnd: String? = null,
)

data class CreateProjectRequest(
    val name: String,
    val methodologyId: String,
    val communityGroup: String? = null,
    val species: String? = null,
    val conservationType: String? = null,
    val communityShareBasisPoints: Int = 0,
    val sites: List<String> = emptyList(),
    val plots: List<CreatePlotRequestDto> = emptyList(),
)

data class CreateProjectResponse(
    val id: String,
    val name: String,
    val status: String,
)

// ── wallet, revenue and reconciliation ────────────────────────────────────

data class WalletBalancesDto(
    val tourismContributions: AmountDto? = null,
    val carbonCreditSales: AmountDto? = null,
    val communityShare: AmountDto? = null,
)

data class AllocationDto(
    val label: String,
    val percent: String,
    val amount: AmountDto,
)

data class RevenueRuleDto(
    val id: String,
    val label: String,
    val communityPercent: String,
    val effectiveFrom: String? = null,
    val allocations: List<AllocationDto> = emptyList(),
)

data class WalletEntryDto(
    val id: String,
    val kind: String,
    val description: String,
    val amount: AmountDto,
    val status: String,
    val source: String? = null,
    val occurredAt: String,
    val reference: String? = null,
)

data class WalletDto(
    val projectId: String,
    val mode: String,
    val balances: WalletBalancesDto,
    val revenueRule: RevenueRuleDto? = null,
    val entries: List<WalletEntryDto> = emptyList(),
    val lastImportedAt: String? = null,
)

data class ReconciliationIssueDto(
    val id: String,
    val detail: String,
    val severity: String,
)

data class ReconciliationDto(
    val projectId: String,
    val status: String,
    val checkedAt: String? = null,
    val issues: List<ReconciliationIssueDto> = emptyList(),
)

data class DocumentDto(
    val id: String,
    val kind: String? = null,
    val name: String,
    val status: String,
    val version: String? = null,
    val updatedAt: String? = null,
    val downloadUrl: String? = null,
)

data class DocumentsResponse(val documents: List<DocumentDto> = emptyList())

// ── identity profile ──────────────────────────────────────────────────────

data class RoleCapabilityDto(
    val role: String,
    val status: String,
    val reference: String? = null,
)

data class LinkedPlatformDto(
    val platform: String,
    val issuer: String,
    val subject: String,
    val status: String,
    val linkedAt: String? = null,
)

data class LinksResponse(val links: List<LinkedPlatformDto> = emptyList())

/**
 * `keyCustodyReference` is an envelope handle. This client never receives,
 * stores, or asks for key material, a mnemonic, or a wallet password.
 */
data class IdentityProfileDto(
    val user: AuthUserDto,
    val status: String,
    val did: String? = null,
    val address: String? = null,
    val didDocumentReference: String? = null,
    val keyCustodyReference: String? = null,
    val network: String,
    val provisioningError: String? = null,
    val roleCapabilities: List<RoleCapabilityDto> = emptyList(),
    val transactionReferences: List<String> = emptyList(),
    val linkedPlatforms: List<LinkedPlatformDto> = emptyList(),
)

// ── provenance and verification ───────────────────────────────────────────

data class ActivityRefDto(
    val id: String,
    val projectName: String,
    val achievementId: String? = null,
    val verificationRequestId: String? = null,
    val status: String,
)

data class VerificationStageDto(
    val id: String,
    val key: String,
    val label: String,
    val owner: String? = null,
    val status: String,
    val timestamp: String? = null,
    val evidenceReferences: List<String> = emptyList(),
    val exception: String? = null,
    val nextAction: String? = null,
)

/**
 * `network` tells the client which ledger a reference came from. A sandbox
 * reference is labelled sandbox and never rendered as an IOTA claim.
 */
data class ProvenanceEventDto(
    val id: String,
    val type: String,
    val actor: String,
    val timestamp: String,
    val status: String,
    val network: String? = null,
    val commitment: String? = null,
    val chainReference: String? = null,
    val explorerUrl: String? = null,
    val objectId: String? = null,
    val version: Int = 0,
    val detail: String? = null,
)

data class ProvenanceDto(
    val activity: ActivityRefDto,
    val stages: List<VerificationStageDto> = emptyList(),
    val documents: List<DocumentDto> = emptyList(),
    val events: List<ProvenanceEventDto> = emptyList(),
)

data class QueueItemDto(
    val id: String,
    val activityId: String,
    val projectName: String,
    val plotCode: String? = null,
    val status: String,
    val requestedAt: String? = null,
    val methodologyVersion: String? = null,
    val evidenceCount: Int = 0,
    val recordsCount: Int = 0,
    val unsignedRecords: Int = 0,
)

data class VerificationQueueResponse(val items: List<QueueItemDto> = emptyList())

data class DecisionRequest(
    val decision: String,
    val notes: String,
    val methodologyVersion: String,
    val stepUpCode: String,
    val idempotencyKey: String,
)

data class DecisionDto(
    val status: String,
    val attestationReference: String? = null,
    val provenanceStatus: String? = null,
    val explorerUrl: String? = null,
)

data class ActivityDto(
    val id: String,
    val status: String,
    val projectName: String? = null,
)

data class CompleteResponse(
    val activity: ActivityDto,
    val event: ProvenanceEventDto? = null,
)

// ── proofs ────────────────────────────────────────────────────────────────

data class VerifierAttestationDto(
    val organization: String,
    val decision: String,
    val timestamp: String,
    val did: String? = null,
)

data class ChainBlockDto(
    val network: String,
    val transactionReference: String? = null,
    val commitment: String? = null,
    val explorerUrl: String? = null,
)

data class EvidenceEntryDto(
    val name: String,
    val sha256: String,
    val access: String,
    val kind: String? = null,
)

data class ProofModesDto(
    val chain: String,
    val satellite: String,
    val registry: String,
)

data class ProofDto(
    val id: String,
    val achievementId: String,
    val version: Int,
    val proofType: String,
    val status: String,
    val generatedAt: String,
    val projectName: String,
    val claimSummary: String,
    val issuerDid: String? = null,
    val verifierAttestation: VerifierAttestationDto? = null,
    val chain: ChainBlockDto? = null,
    val registryReferences: List<String> = emptyList(),
    val evidenceManifest: List<EvidenceEntryDto> = emptyList(),
    val events: List<ProvenanceEventDto> = emptyList(),
    val availableFormats: List<String> = emptyList(),
    val supersededBy: String? = null,
    val modes: ProofModesDto? = null,
    val sandboxNotice: String? = null,
    val manifestHash: String? = null,
)

data class ProofsResponse(val proofs: List<ProofDto> = emptyList())

data class GenerateProofRequest(
    val format: String? = null,
    val idempotencyKey: String,
)

// ── marketplace ───────────────────────────────────────────────────────────

data class RegistryDto(
    val name: String,
    val status: String,
    val reference: String? = null,
)

data class ListingDto(
    val id: String,
    val batchId: String,
    val projectName: String,
    val vintage: String,
    val productType: String,
    val registry: RegistryDto,
    val availableTonnes: String,
    val coBenefits: List<String> = emptyList(),
    val pricePerTonne: AmountDto,
    val mode: String,
)

data class ListingsResponse(val listings: List<ListingDto> = emptyList())

data class PurchaseRequest(
    val listingId: String,
    val quantityTonnes: String,
    val stepUpCode: String,
    val idempotencyKey: String,
)

data class PurchaseResultDto(
    val id: String,
    val listingId: String,
    val batchId: String,
    val quantityTonnes: String,
    val total: AmountDto,
    val status: String,
    val mode: String,
    val custodyReference: String? = null,
    val settlementReference: String? = null,
    val detail: String? = null,
)

data class RetirementRequest(
    val quantityTonnes: String,
    val reason: String,
    val stepUpCode: String,
    val idempotencyKey: String,
)

data class RetirementResultDto(
    val id: String,
    val batchId: String,
    val quantityTonnes: String,
    val status: String,
    val mode: String,
    val registrySerialReference: String? = null,
    val registryCertificateReference: String? = null,
    val chainReference: String? = null,
    val chainNetwork: String? = null,
    val explorerUrl: String? = null,
    val retiredAt: String? = null,
    val detail: String? = null,
)

// ── field capture and sync ────────────────────────────────────────────────

data class LocationDto(
    val latitude: Double,
    val longitude: Double,
    val accuracyMetres: Double,
)

data class AttestationDto(
    val statement: String,
    val identityDid: String? = null,
    val signature: String? = null,
)

data class EvidenceManifestItemDto(
    val name: String,
    val kind: String,
    val mediaType: String,
    val size: Long,
    val sha256: String,
)

/**
 * Decimal measures are strings and the timestamp is RFC3339, matching how the
 * server canonicalises a record. `schemaVersion` is a string for the same reason.
 */
data class SyncRecordDto(
    val clientId: String,
    val activityId: String,
    val plotId: String,
    val methodologyVersion: String,
    val schemaVersion: String,
    val deviceTimestamp: String,
    val location: LocationDto,
    val aliveCount: Int,
    val plantedCount: Int,
    val averageHeightMetres: String,
    val species: String,
    val observations: String? = null,
    val attestation: AttestationDto,
    val evidenceManifest: List<EvidenceManifestItemDto> = emptyList(),
)

data class SyncActionDto(
    val type: String,
    val record: SyncRecordDto,
)

data class SyncBatchRequest(val actions: List<SyncActionDto>)

data class ConflictDto(
    val serverVersion: String,
    val detail: String,
)

data class SyncResultDto(
    val clientId: String,
    val monitoringRecordId: String? = null,
    val status: String,
    val error: String? = null,
    val conflict: ConflictDto? = null,
    val signed: Boolean = false,
    val lowAccuracy: Boolean = false,
)

data class SyncBatchResponse(val results: List<SyncResultDto> = emptyList())

data class EvidenceUploadResponse(
    val id: String,
    val sha256: String,
    val status: String,
)

data class DisputeRequest(val reason: String)

data class AssignVerifierRequest(val verifierId: String)

data class AnchorRetryResponse(
    val status: String,
    val event: ProvenanceEventDto? = null,
)

data class VerificationRequestDto(
    val id: String,
    val activityId: String,
    val verifierId: String,
    val status: String,
    val requestedAt: String? = null,
)

data class DisputeDto(
    val id: String,
    val activityId: String,
    val status: String,
    val reason: String,
    val createdAt: String? = null,
)

// ── API ───────────────────────────────────────────────────────────────────
data class SatellitePassDto(
    val id: String,
    val projectId: String,
    val mission: String,
    val passType: String,
    val windowStart: String? = null,
    val windowEnd: String? = null,
    val acquiredAt: String? = null,
    val sceneId: String? = null,
    val cloudCoverPct: Double? = null,
    val coveragePct: Double = 0.0,
    val statusDetail: String? = null,
    val adapterMode: String? = null,
)

data class SatellitePassesResponse(val items: List<SatellitePassDto> = emptyList())

data class SatelliteLatestResponse(val pass: SatellitePassDto? = null)


data class WeatherDto(
    val source: String? = null,
    val temperatureC: Double? = null,
    val humidityPct: Int? = null,
    val windKph: Double? = null,
    val note: String? = null,
)

data class SalinityDto(
    val source: String? = null,
    val salinityPpt: Double? = null,
    val note: String? = null,
)



data class ProjectActivityDto(
    val id: String,
    val projectId: String,
    val plotId: String,
    val activityType: String,
    val status: String,
    val periodStart: String? = null,
    val periodEnd: String? = null,
    val methodologyVersion: String? = null,
)

data class ActivitiesResponse(val items: List<ProjectActivityDto> = emptyList())

data class MonitorDto(
    val id: String,
    val displayName: String,
    val email: String,
    val role: String,
    val status: String,
)

data class MonitorsResponse(val items: List<MonitorDto> = emptyList())

data class UpdatePlotRequest(val assignedMonitorId: String? = null)

data class PlotResponse(
    val id: String,
    val projectId: String,
    val siteId: String,
    val code: String,
    val latitude: Double,
    val longitude: Double,
    val areaHectares: String,
    val assignedMonitorId: String? = null,
)


// ── planting activity detail ──────────────────────────────────────────────

/**
 * Bundles the monitoring record, evidence manifest, and the latest provenance
 * event for one activity in a single round trip, so the mobile detail screen
 * does not chase three endpoints. `record` and `provenance` are null when the
 * activity has no synced record or is not yet anchored on-chain.
 */
data class MonitoringRecordViewDto(
    val aliveCount: Int = 0,
    val plantedCount: Int = 0,
    val averageHeightMetres: String = "",
    val species: String = "",
    val observations: String? = null,
    val attestationStatement: String? = null,
    val evidence: List<EvidenceViewDto> = emptyList(),
)

data class EvidenceViewDto(
    val id: String,
    val kind: String,
    val sha256: String,
    val mediaType: String,
    val url: String? = null,
)

data class ProvViewDto(
    val status: String,
    val network: String? = null,
    val explorerUrl: String? = null,
    val objectId: String? = null,
)

data class ActivityDetailDto(
    val id: String,
    val projectId: String,
    val plotId: String,
    val activityType: String,
    val status: String,
    val methodologyVersion: String? = null,
    val periodStart: String? = null,
    val periodEnd: String? = null,
    val record: MonitoringRecordViewDto? = null,
    val provenance: ProvViewDto? = null,
)

interface BahariMrvApi {
    @GET("api/v1/health")
    suspend fun health(): HealthDto

    @POST("api/v1/auth/login")
    suspend fun login(@Body request: LoginRequest): LoginResponse

    @POST("api/v1/auth/register")
    suspend fun register(@Body request: RegisterRequest): LoginResponse

    @POST("api/v1/auth/logout")
    suspend fun logout()

    @GET("api/v1/auth/me")
    suspend fun me(): MeResponse

    @POST("api/v1/auth/step-up")
    suspend fun stepUp(@Body request: StepUpRequest): StepUpResponse

    @POST("api/v1/auth/federation/exchange")
    suspend fun exchangeFederation(@Body request: FederationExchangeRequest): LoginResponse

    @POST("api/v1/auth/federation/link")
    suspend fun linkFederation(@Body request: FederationExchangeRequest): FederationLinkResponse

    @GET("api/v1/projects")
    suspend fun projects(): ProjectsResponse
    @GET("api/v1/projects/{id}/activities")
    suspend fun activities(@Path("id") projectId: String): ActivitiesResponse
    @POST("api/v1/projects/{id}/activities")
    suspend fun createActivity(
        @Path("id") projectId: String,
        @Body request: CreateActivityRequestDto
    ): ProjectActivityDto
    @GET("api/v1/projects/{id}/weather")
    suspend fun weather(@Path("id") projectId: String): WeatherDto

    @GET("api/v1/projects/{id}/salinity")
    suspend fun salinity(@Path("id") projectId: String): SalinityDto
    @GET("api/v1/projects/{id}/satellite/passes")
    suspend fun satellitePasses(
        @Path("id") projectId: String,
        @Query("mission") mission: String,
        @Query("from") from: String? = null,
        @Query("to") to: String? = null,
    ): SatellitePassesResponse

    @GET("api/v1/projects/{id}/satellite/latest")
    suspend fun satelliteLatest(
        @Path("id") projectId: String,
        @Query("mission") mission: String,
    ): SatelliteLatestResponse





    @GET("api/v1/monitors")
    suspend fun monitors(): MonitorsResponse

    @PATCH("api/v1/projects/{id}/plots/{plotId}")
    suspend fun updatePlot(
        @Path("id") projectId: String,
        @Path("plotId") plotId: String,
        @Body request: UpdatePlotRequest,
    ): PlotResponse


    @GET("api/v1/methodologies")
    suspend fun methodologies(): MethodologiesResponse

    @POST("api/v1/projects")
    suspend fun createProject(@Body request: CreateProjectRequest): CreateProjectResponse


    @GET("api/v1/projects/{id}/dashboard")
    suspend fun dashboard(@Path("id") projectId: String): DashboardDto

    @GET("api/v1/projects/{id}/wallet")
    suspend fun wallet(@Path("id") projectId: String): WalletDto

    @GET("api/v1/projects/{id}/reconciliation")
    suspend fun reconciliation(@Path("id") projectId: String): ReconciliationDto

    @GET("api/v1/projects/{id}/documents")
    suspend fun documents(@Path("id") projectId: String): DocumentsResponse

    @Streaming
    @GET("api/v1/documents/{id}")
    suspend fun downloadDocument(@Path("id") documentId: String): ResponseBody

    @POST("api/v1/sync/batch")
    suspend fun syncBatch(@Body request: SyncBatchRequest): SyncBatchResponse

    @POST("api/v1/monitoring-records/{id}/evidence")
    @Multipart
    suspend fun uploadEvidence(
        @Path("id") monitoringRecordId: String,
        @Part file: MultipartBody.Part,
        @Part("kind") kind: RequestBody,
        @Part("sha256") sha256: RequestBody,
    ): EvidenceUploadResponse

    @GET("api/v1/activities/{id}/provenance")
    suspend fun provenance(@Path("id") activityId: String): ProvenanceDto

    @GET("api/v1/activities/{id}/detail")
    suspend fun activityDetail(@Path("id") activityId: String): ActivityDetailDto

    @Streaming
    @GET("api/v1/monitoring-records/evidence/{id}")
    suspend fun evidenceMedia(@Path("id") evidenceId: String): ResponseBody

    @POST("api/v1/activities/{id}/complete")
    suspend fun complete(@Path("id") activityId: String): CompleteResponse

    @GET("api/v1/anchors/{id}/status")
    suspend fun anchorStatus(@Path("id") eventId: String): ProvenanceEventDto

    @POST("api/v1/activities/{id}/anchor/retry")
    suspend fun retryAnchor(@Path("id") activityId: String): AnchorRetryResponse

    @POST("api/v1/activities/{id}/dispute")
    suspend fun dispute(@Path("id") activityId: String, @Body request: DisputeRequest): DisputeDto

    @POST("api/v1/activities/{id}/submit-verification")
    suspend fun submitVerification(
        @Path("id") activityId: String,
        @Body request: AssignVerifierRequest,
    ): VerificationRequestDto

    @GET("api/v1/verification-requests")
    suspend fun verificationQueue(): VerificationQueueResponse

    @POST("api/v1/verification-requests/{id}/decision")
    suspend fun decision(
        @Path("id") requestId: String,
        @Body request: DecisionRequest,
    ): DecisionDto

    @GET("api/v1/achievements/{id}/proofs")
    suspend fun proofs(@Path("id") achievementId: String): ProofsResponse

    @POST("api/v1/achievements/{id}/proofs")
    suspend fun generateProof(
        @Path("id") achievementId: String,
        @Body request: GenerateProofRequest,
    ): ProofDto

    @Streaming
    @GET("api/v1/proofs/{id}/download")
    suspend fun downloadProof(
        @Path("id") proofId: String,
        @Query("format") format: String,
    ): ResponseBody

    @GET("api/v1/marketplace/listings")
    suspend fun listings(): ListingsResponse

    @POST("api/v1/marketplace/purchases")
    suspend fun purchase(@Body request: PurchaseRequest): PurchaseResultDto

    @POST("api/v1/credit-batches/{id}/retire")
    suspend fun retire(
        @Path("id") creditBatchId: String,
        @Body request: RetirementRequest,
    ): RetirementResultDto

    @GET("api/v1/users/me/identity")
    suspend fun identity(): IdentityProfileDto

    @POST("api/v1/identity/provision/retry")
    suspend fun retryIdentity(): IdentityProfileDto

    @GET("api/v1/auth/links")
    suspend fun links(): LinksResponse
}

// ── Jahazi federation ─────────────────────────────────────────────────────

/**
 * The backend, not this device, redeems the authorization code. The issuer is sent
 * in the body and matched against the server's own trust list, so an unconfigured
 * URL can never be injected.
 */
data class FederationExchangeRequest(
    val issuer: String,
    val code: String,
    val codeVerifier: String,
    val nonce: String,
    val redirectUri: String,
)

data class FederationLinkResponse(
    val issuer: String,
    val subject: String,
    val status: String,
)
