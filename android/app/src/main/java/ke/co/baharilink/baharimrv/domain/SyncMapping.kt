package ke.co.baharilink.baharimrv.domain

import ke.co.baharilink.baharimrv.data.local.EvidenceEntity
import ke.co.baharilink.baharimrv.data.local.MonitoringRecordEntity
import ke.co.baharilink.baharimrv.data.local.MonitoringWithEvidence
import ke.co.baharilink.baharimrv.data.local.SyncState
import ke.co.baharilink.baharimrv.data.remote.AttestationDto
import ke.co.baharilink.baharimrv.data.remote.EvidenceManifestItemDto
import ke.co.baharilink.baharimrv.data.remote.LocationDto
import ke.co.baharilink.baharimrv.data.remote.SyncActionDto
import ke.co.baharilink.baharimrv.data.remote.SyncBatchRequest
import ke.co.baharilink.baharimrv.data.remote.SyncRecordDto
import java.math.BigDecimal
import java.math.RoundingMode
import java.security.MessageDigest
import java.time.Instant
import java.time.temporal.ChronoUnit

/** The action type the server's sync dispatcher recognises for a captured record. */
private const val SYNC_ACTION_TYPE = "monitoring_record"

/**
 * Shown to the monitor at capture and stored with the record. The server hashes this
 * text as part of the record's commitment, so it must stay byte-stable.
 */
const val ATTESTATION_STATEMENT =
    "I recorded these observations at the plot and attest they describe what I measured."

/** The server canonicalises quantities at six decimal places. */
private const val QUANTITY_SCALE = 6

object CanonicalMonitoring {
    /**
     * A local integrity fingerprint over the field set the server canonicalises. It is
     * never sent: the server computes and stores its own commitment for the record.
     */
    fun commitment(record: MonitoringRecordEntity, evidence: List<EvidenceEntity>): String {
        val manifest = evidence.sortedWith(ManifestOrder).joinToString(
            separator = ",",
            prefix = "[",
            postfix = "]",
        ) { item ->
            objectJson(
                "name" to item.originalFileName.jsonString(),
                "kind" to item.kind.name.lowercase().jsonString(),
                "mediaType" to item.mimeType.jsonString(),
                "size" to item.sizeBytes.toString(),
                "sha256" to item.sha256.jsonString(),
            )
        }
        val canonical = objectJson(
            "clientId" to record.clientId.jsonString(),
            "activityId" to record.activityId.jsonString(),
            "plotId" to record.plotId.jsonString(),
            "projectId" to record.projectId.jsonString(),
            "methodologyVersion" to record.methodologyVersion.jsonString(),
            "schemaVersion" to record.schemaVersion.toString().jsonString(),
            "deviceTimestamp" to deviceTimestamp(record.deviceCapturedAtEpochMs).jsonString(),
            "latitude" to record.latitude.jsonNumber(),
            "longitude" to record.longitude.jsonNumber(),
            "accuracyMetres" to record.gpsAccuracyMeters.jsonNumber(),
            "aliveCount" to record.aliveCount.toString(),
            "plantedCount" to record.plantedCount.toString(),
            "averageHeightMetres" to quantity(record.averageHeightMeters).jsonString(),
            "species" to record.species.jsonString(),
            "observations" to "".jsonString(),
            "attestation" to ATTESTATION_STATEMENT.jsonString(),
            "evidenceManifest" to manifest,
        )
        return sha256(canonical)
    }

    private fun objectJson(vararg fields: Pair<String, String>): String = fields.joinToString(
        separator = ",",
        prefix = "{",
        postfix = "}",
    ) { (name, value) -> "${name.jsonString()}:$value" }

    private fun String?.jsonString(): String = if (this == null) "null" else buildString {
        append('"')
        this@jsonString.forEach { character ->
            when (character) {
                '"' -> append("\\\"")
                '\\' -> append("\\\\")
                '\b' -> append("\\b")
                '\u000C' -> append("\\f")
                '\n' -> append("\\n")
                '\r' -> append("\\r")
                '\t' -> append("\\t")
                else -> if (character < ' ') append("\\u%04x".format(character.code)) else append(character)
            }
        }
        append('"')
    }

    private fun Double?.jsonNumber(): String =
        this?.let { BigDecimal.valueOf(it).stripTrailingZeros().toPlainString() } ?: "null"

    private fun sha256(value: String): String = MessageDigest.getInstance("SHA-256")
        .digest(value.toByteArray(Charsets.UTF_8))
        .joinToString("") { "%02x".format(it) }
}

/** The server orders manifest entries by hash, then name, before hashing them. */
private val ManifestOrder = compareBy<EvidenceEntity>({ it.sha256 }, { it.originalFileName })

fun quantity(value: Double): String =
    BigDecimal.valueOf(value).setScale(QUANTITY_SCALE, RoundingMode.HALF_EVEN).toPlainString()

/** RFC3339 in UTC; the server rejects a timestamp it cannot parse. */
fun deviceTimestamp(epochMs: Long): String =
    Instant.ofEpochMilli(epochMs).truncatedTo(ChronoUnit.MILLIS).toString()

fun EvidenceEntity.toManifestItem(): EvidenceManifestItemDto = EvidenceManifestItemDto(
    name = originalFileName,
    kind = kind.name.lowercase(),
    mediaType = mimeType,
    size = sizeBytes,
    sha256 = sha256,
)

/**
 * The device holds no key material, so it sends no DID or signature. The server
 * accepts the record and reports it as unsigned rather than inventing authorship.
 */
fun MonitoringWithEvidence.toSyncRecord(): SyncRecordDto = SyncRecordDto(
    clientId = record.clientId,
    activityId = record.activityId,
    plotId = record.plotId,
    methodologyVersion = record.methodologyVersion,
    schemaVersion = record.schemaVersion.toString(),
    deviceTimestamp = deviceTimestamp(record.deviceCapturedAtEpochMs),
    location = LocationDto(
        latitude = record.latitude,
        longitude = record.longitude,
        accuracyMetres = record.gpsAccuracyMeters,
    ),
    aliveCount = record.aliveCount,
    plantedCount = record.plantedCount,
    averageHeightMetres = quantity(record.averageHeightMeters),
    species = record.species,
    observations = "",
    attestation = AttestationDto(statement = ATTESTATION_STATEMENT),
    evidenceManifest = evidence.sortedWith(ManifestOrder).map { it.toManifestItem() },
)

object SyncPayloadFactory {
    fun create(records: List<MonitoringWithEvidence>): SyncBatchRequest {
        require(records.isNotEmpty()) { "A sync batch requires at least one record" }
        require(records.size <= MAX_BATCH) { "A sync batch holds at most $MAX_BATCH records" }
        return SyncBatchRequest(
            records.sortedBy { it.record.clientId }.map {
                SyncActionDto(type = SYNC_ACTION_TYPE, record = it.toSyncRecord())
            },
        )
    }

    private const val MAX_BATCH = 50
}

/**
 * Maps the server's sync status onto the device state machine. Unknown values return
 * null so the caller treats the record as unresolved instead of guessing a state.
 */
fun syncStateFromServer(status: String): SyncState? = when (status.lowercase()) {
    "submitted" -> SyncState.SUBMITTED
    "awaiting_verification" -> SyncState.AWAITING_VERIFICATION
    "anchoring" -> SyncState.ANCHORING
    "anchored" -> SyncState.ANCHORED
    "verification_failed" -> SyncState.VERIFICATION_FAILED
    "sync_conflict" -> SyncState.SYNC_CONFLICT
    "failed" -> SyncState.SYNC_FAILED
    else -> null
}
