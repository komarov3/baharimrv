package ke.co.baharilink.baharimrv.data.repository

import java.math.BigDecimal
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import ke.co.baharilink.baharimrv.data.remote.BahariMrvApi
import ke.co.baharilink.baharimrv.data.remote.DecisionRequest
import ke.co.baharilink.baharimrv.data.remote.CreatePlotRequestDto
import ke.co.baharilink.baharimrv.data.remote.CreateProjectRequest
import ke.co.baharilink.baharimrv.data.remote.UpdatePlotRequest
import ke.co.baharilink.baharimrv.data.remote.DisputeRequest
import ke.co.baharilink.baharimrv.data.remote.GenerateProofRequest
import ke.co.baharilink.baharimrv.data.remote.PurchaseRequest
import ke.co.baharilink.baharimrv.data.remote.RetirementRequest
import ke.co.baharilink.baharimrv.data.remote.StepUpRequest

/**
 * Every gated write mints a fresh single-use step-up ticket from the password the
 * user just entered. The ticket is bound to one action name on the server, so the
 * action constant here must match the server's own, or the call is rejected.
 */
private const val ACTION_VERIFICATION_DECISION = "verification_decision"
private const val ACTION_PURCHASE = "marketplace_purchase"
private const val ACTION_RETIREMENT = "credit_retirement"

@Singleton
class ProjectRepository @Inject constructor(private val api: BahariMrvApi) {
    suspend fun health() = api.health()
    suspend fun projects() = api.projects()
    suspend fun activities(projectId: String) = api.activities(projectId).items
    suspend fun weather(projectId: String) = api.weather(projectId)
    suspend fun salinity(projectId: String) = api.salinity(projectId)
    suspend fun satellitePasses(projectId: String, mission: String) =
        api.satellitePasses(projectId, mission).items
    suspend fun satelliteLatest(projectId: String, mission: String) =
        api.satelliteLatest(projectId, mission).pass
    suspend fun monitors() = api.monitors().items
    suspend fun assignMonitor(projectId: String, plotId: String, monitorId: String?) =
        api.updatePlot(projectId, plotId, UpdatePlotRequest(assignedMonitorId = monitorId))

    suspend fun methodologies() = api.methodologies()
    suspend fun createProject(
        name: String,
        methodologyId: String,
        conservationType: String,
        communityGroup: String,
        species: String,
        communityShareBasisPoints: Int,
        sites: List<String>,
        plots: List<CreatePlotRequestDto>,
    ) = api.createProject(
        CreateProjectRequest(
            name = name.trim().ifBlank { error("Project name is required") },
            methodologyId = methodologyId.ifBlank { error("Methodology is required") },
            communityGroup = communityGroup.trim().ifBlank { null },
            species = species.trim().ifBlank { null },
            conservationType = conservationType.trim().ifBlank { null },
            communityShareBasisPoints = communityShareBasisPoints,
            sites = if (sites.map { it.trim() }.filter { it.isNotBlank() }.isEmpty()) {
            listOf("Site 1")
        } else {
            sites.map { it.trim() }.filter { it.isNotBlank() }
        },
            plots = plots.map { plot ->
            if (plot.siteName.isBlank()) plot.copy(siteName = "Site 1") else plot
        },
        ),
    )
    suspend fun dashboard(projectId: String) = api.dashboard(projectId)
    suspend fun wallet(projectId: String) = api.wallet(projectId)
    suspend fun reconciliation(projectId: String) = api.reconciliation(projectId)
    suspend fun documents(projectId: String) = api.documents(projectId)
    suspend fun downloadDocument(documentId: String) = api.downloadDocument(documentId)
    suspend fun provenance(activityId: String) = api.provenance(activityId)
    suspend fun anchorStatus(eventId: String) = api.anchorStatus(eventId)
    suspend fun verificationQueue() = api.verificationQueue()
    suspend fun listings() = api.listings()
    suspend fun identity() = api.identity()
    suspend fun retryIdentity() = api.retryIdentity()
    suspend fun links() = api.links()

    /**
     * Records a verifier decision. `decision` is the server's own vocabulary:
     * approve, reject, or request_corrections.
     */
    suspend fun decide(
        requestId: String,
        password: String,
        decision: String,
        notes: String,
        methodologyVersion: String,
    ) = api.decision(
        requestId,
        DecisionRequest(
            decision = requireDecision(decision),
            notes = notes.trim().ifBlank { error("A written rationale is required with every decision") },
            methodologyVersion = methodologyVersion.ifBlank { error("The methodology version is required") },
            stepUpCode = stepUpCode(password, ACTION_VERIFICATION_DECISION),
            idempotencyKey = newKey(),
        ),
    )

    suspend fun proofs(achievementId: String) = api.proofs(achievementId)

    /** The manager's own claim that the activity is achieved; it is recorded as unverified. */
    suspend fun completeActivity(activityId: String) = api.complete(
        activityId.ifBlank { error("Activity ID is required") },
    )

    suspend fun dispute(activityId: String, reason: String) = api.dispute(
        activityId.ifBlank { error("Activity ID is required") },
        DisputeRequest(reason.trim().ifBlank { error("State what is contested before disputing") }),
    )

    /** Re-submits an unconfirmed anchor. The server, not this device, decides the new status. */
    suspend fun retryAnchor(activityId: String) = api.retryAnchor(
        activityId.ifBlank { error("Activity ID is required") },
    )

    /** A proof is rebuilt from current server state on every call; the key only guards retries. */
    suspend fun generateProof(achievementId: String) = api.generateProof(
        achievementId,
        GenerateProofRequest(format = null, idempotencyKey = newKey()),
    )

    suspend fun downloadProof(proofId: String, format: String) = api.downloadProof(proofId, requireFormat(format))

    suspend fun purchase(password: String, listingId: String, quantityTonnes: String) = api.purchase(
        PurchaseRequest(
            listingId = listingId.ifBlank { error("Select a listing first") },
            quantityTonnes = requirePositiveDecimal(quantityTonnes),
            stepUpCode = stepUpCode(password, ACTION_PURCHASE),
            idempotencyKey = newKey(),
        ),
    )

    suspend fun retire(
        password: String,
        creditBatchId: String,
        quantityTonnes: String,
        reason: String,
    ) = api.retire(
        creditBatchId.ifBlank { error("Credit batch ID is required") },
        RetirementRequest(
            quantityTonnes = requirePositiveDecimal(quantityTonnes),
            reason = reason.trim().ifBlank { error("Retirement reason is required") },
            stepUpCode = stepUpCode(password, ACTION_RETIREMENT),
            idempotencyKey = newKey(),
        ),
    )

    private suspend fun stepUpCode(password: String, action: String): String {
        require(password.isNotBlank()) { "Confirm your password to authorise this action" }
        val ticket = api.stepUp(StepUpRequest(password, action))
        require(ticket.stepUpCode.isNotBlank()) { "The server did not issue a confirmation ticket" }
        require(ticket.action == action) { "The confirmation ticket is not valid for this action" }
        return ticket.stepUpCode
    }

    private fun requireDecision(value: String) = value.trim().lowercase().ifEmpty {
        error("Choose approve, reject, or request corrections")
    }.also {
        if (it !in setOf("approve", "reject", "request_corrections")) {
            error("The decision must be approve, reject or request_corrections")
        }
    }

    private fun requireFormat(value: String) = value.trim().lowercase().ifEmpty {
        error("Choose pdf, json, or zip")
    }.also {
        if (it !in setOf("pdf", "json", "zip")) error("Proofs render as pdf, json, or zip")
    }

    private fun requirePositiveDecimal(value: String): String {
        val trimmed = value.trim()
        val isPositive = runCatching { BigDecimal(trimmed).signum() > 0 }.getOrDefault(false)
        if (!isPositive) error("Enter a quantity greater than zero")
        return trimmed
    }

    private fun newKey(): String = UUID.randomUUID().toString()
}
