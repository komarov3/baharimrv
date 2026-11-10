package ke.co.baharilink.baharimrv.data.repository

import android.content.Context
import android.net.Uri
import androidx.room.withTransaction
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.io.IOException
import java.security.MessageDigest
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import ke.co.baharilink.baharimrv.data.local.BahariMrvDatabase
import ke.co.baharilink.baharimrv.data.local.EvidenceDao
import ke.co.baharilink.baharimrv.data.local.EvidenceEntity
import ke.co.baharilink.baharimrv.data.local.EvidenceKind
import ke.co.baharilink.baharimrv.data.local.MonitoringDao
import ke.co.baharilink.baharimrv.data.local.MonitoringRecordEntity
import ke.co.baharilink.baharimrv.data.local.MonitoringWithEvidence
import ke.co.baharilink.baharimrv.data.local.SyncState
import ke.co.baharilink.baharimrv.data.remote.BahariMrvApi
import ke.co.baharilink.baharimrv.domain.CanonicalMonitoring
import ke.co.baharilink.baharimrv.domain.SyncPayloadFactory
import ke.co.baharilink.baharimrv.domain.syncStateFromServer
import ke.co.baharilink.baharimrv.sync.SyncScheduler
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import retrofit2.HttpException

data class CaptureInput(
    val projectId: String,
    val plotId: String,
    val activityId: String,
    val latitude: Double,
    val longitude: Double,
    val gpsAccuracyMeters: Double,
    val plantedCount: Int,
    val aliveCount: Int,
    val averageHeightMeters: Double,
    val species: String,
    val methodologyVersion: String,
    val attested: Boolean,
)

data class EvidenceSelection(
    val uri: Uri,
    val kind: EvidenceKind,
    val originalFileName: String,
    val mimeType: String,
    val capturedAtEpochMs: Long,
    val latitude: Double? = null,
    val longitude: Double? = null,
    val transcription: String? = null,
)

enum class SyncRunResult { EMPTY, SUCCESS, RETRY, FAILURE }

@Singleton
class MonitoringRepository @Inject constructor(
    @ApplicationContext private val context: Context,
    private val database: BahariMrvDatabase,
    private val monitoringDao: MonitoringDao,
    private val evidenceDao: EvidenceDao,
    private val api: BahariMrvApi,
    private val scheduler: SyncScheduler,
) {
    val records: Flow<List<MonitoringWithEvidence>> = monitoringDao.observeAll()
    val pendingCount: Flow<Int> = monitoringDao.observePendingCount()

    // ── Remote lookups for dropdowns ──────────────────────────────────────────
    suspend fun projects()                        = api.projects().rows
    suspend fun methodologies()                   = api.methodologies().items
    suspend fun dashboard(projectId: String)      = api.dashboard(projectId)
    suspend fun activities(projectId: String)     = api.activities(projectId).items

    suspend fun saveCapture(input: CaptureInput, selections: List<EvidenceSelection>): String = withContext(Dispatchers.IO) {
        validate(input)
        val now = System.currentTimeMillis()
        val clientId = UUID.randomUUID().toString()
        val evidence = selections.map { selection ->
            importEvidence(clientId, selection, now)
        }
        val unsignedRecord = MonitoringRecordEntity(
            clientId = clientId,
            projectId = input.projectId.trim(),
            plotId = input.plotId.trim(),
            activityId = input.activityId.trim(),
            latitude = input.latitude,
            longitude = input.longitude,
            gpsAccuracyMeters = input.gpsAccuracyMeters,
            plantedCount = input.plantedCount,
            aliveCount = input.aliveCount,
            averageHeightMeters = input.averageHeightMeters,
            species = input.species.trim(),
            deviceCapturedAtEpochMs = now,
            methodologyVersion = input.methodologyVersion.trim(),
            schemaVersion = SCHEMA_VERSION,
            contentCommitment = "",
            createdAtEpochMs = now,
            updatedAtEpochMs = now,
        )
        val record = unsignedRecord.copy(
            contentCommitment = CanonicalMonitoring.commitment(unsignedRecord, evidence),
        )
        database.withTransaction {
            monitoringDao.insert(record)
            evidenceDao.insertAll(evidence)
        }
        scheduler.schedule()
        clientId
    }

    suspend fun syncPending(): SyncRunResult = withContext(Dispatchers.IO) {
        val pending = monitoringDao.pendingRecords()
        if (pending.isEmpty()) return@withContext SyncRunResult.EMPTY
        val ids = pending.map { it.clientId }
        monitoringDao.markUploading(ids, System.currentTimeMillis())
        val records = ids.mapNotNull { monitoringDao.get(it) }
        if (records.isEmpty()) return@withContext SyncRunResult.EMPTY

        try {
            val response = api.syncBatch(SyncPayloadFactory.create(records))
            val results = response.results.associateBy { it.clientId }
            var outcome = SyncRunResult.SUCCESS
            records.forEach { item ->
                val current = item.record
                val result = results[current.clientId]
                when {
                    result == null -> {
                        markFailed(current, "Server omitted this record from the batch response")
                        outcome = SyncRunResult.RETRY
                    }

                    result.status.equals("sync_conflict", ignoreCase = true) -> {
                        monitoringDao.update(
                            current.copy(
                                serverId = result.monitoringRecordId ?: current.serverId,
                                syncState = SyncState.SYNC_CONFLICT,
                                lastError = result.conflict?.detail ?: result.error ?: "A different version of this record is already stored",
                                updatedAtEpochMs = System.currentTimeMillis(),
                            ),
                        )
                        if (outcome == SyncRunResult.SUCCESS) outcome = SyncRunResult.FAILURE
                    }

                    result.status.equals("failed", ignoreCase = true) -> {
                        markFailed(current, result.error ?: "The server rejected this record")
                        if (outcome == SyncRunResult.SUCCESS) outcome = SyncRunResult.FAILURE
                    }

                    else -> {
                        val serverState = syncStateFromServer(result.status)
                        if (serverState == null) {
                            markFailed(current, "Server returned an unsupported sync state '${result.status}'")
                            if (outcome == SyncRunResult.SUCCESS) outcome = SyncRunResult.FAILURE
                        } else {
                            monitoringDao.update(
                                current.copy(
                                    serverId = result.monitoringRecordId ?: current.serverId,
                                    serverReceiptAtEpochMs = System.currentTimeMillis(),
                                    syncState = serverState,
                                    attestationStatus = if (result.signed) "IDENTITY_SIGNATURE_VERIFIED" else "IDENTITY_SIGNATURE_PENDING",
                                    lastError = null,
                                    updatedAtEpochMs = System.currentTimeMillis(),
                                ),
                            )
                        }
                    }
                }
            }
            uploadPendingEvidence()
            outcome
        } catch (error: HttpException) {
            val retryable = error.code() == 408 || error.code() == 429 || error.code() >= 500
            val state = if (error.code() == 409) SyncState.SYNC_CONFLICT else SyncState.SYNC_FAILED
            records.forEach { item ->
                monitoringDao.update(
                    item.record.copy(
                        syncState = state,
                        lastError = "Sync failed with HTTP ${error.code()}",
                        updatedAtEpochMs = System.currentTimeMillis(),
                    ),
                )
            }
            when {
                error.code() == 409 -> SyncRunResult.FAILURE
                retryable -> SyncRunResult.RETRY
                else -> SyncRunResult.FAILURE
            }
        } catch (error: IOException) {
            records.forEach { item -> markFailed(item.record, "No server connection; the local record is preserved") }
            SyncRunResult.RETRY
        } catch (error: Exception) {
            records.forEach { item -> markFailed(item.record, error.message ?: "Unexpected sync error") }
            SyncRunResult.FAILURE
        }
    }

    /**
     * Media streams only after the server has accepted the record, because the
     * upload endpoint is keyed by the stored record ID. A hash mismatch is the
     * server's decision, recorded here rather than smoothed over.
     */
    suspend fun uploadPendingEvidence(limit: Int = 10): Int = withContext(Dispatchers.IO) {
        val queued = evidenceDao.pendingUploads(limit)
        var stored = 0
        queued.forEach { item ->
            val record = monitoringDao.get(item.recordClientId)?.record
            val serverId = record?.serverId
            if (serverId.isNullOrBlank() || record.syncState == SyncState.SYNC_CONFLICT) return@forEach
            val file = File(Uri.parse(item.localUri).path.orEmpty())
            if (!file.isFile || file.length() != item.sizeBytes) {
                evidenceDao.markFailed(item.clientId)
                return@forEach
            }
            val body = MultipartBody.Part.createFormData(
                "file",
                item.originalFileName,
                file.asRequestBody(item.mimeType.toMediaTypeOrNull()),
            )
            try {
                val response = api.uploadEvidence(
                    monitoringRecordId = serverId,
                    file = body,
                    kind = item.kind.name.lowercase().toRequestBody(),
                    sha256 = item.sha256.toRequestBody(),
                )
                if (response.sha256.equals(item.sha256, ignoreCase = true)) {
                    evidenceDao.markUploaded(item.clientId, response.id)
                    stored++
                } else {
                    evidenceDao.markFailed(item.clientId)
                }
            } catch (error: Exception) {
                evidenceDao.markFailed(item.clientId)
            }
        }
        stored
    }

    fun scheduleSync() = scheduler.schedule()

    private suspend fun markFailed(record: MonitoringRecordEntity, message: String) {
        monitoringDao.update(
            record.copy(
                syncState = SyncState.SYNC_FAILED,
                lastError = message,
                updatedAtEpochMs = System.currentTimeMillis(),
            ),
        )
    }

    private fun importEvidence(recordId: String, selection: EvidenceSelection, timestamp: Long): EvidenceEntity {
        val evidenceId = UUID.randomUUID().toString()
        val directory = File(context.filesDir, "evidence/$recordId").apply { mkdirs() }
        val destination = File(directory, evidenceId)
        val digest = MessageDigest.getInstance("SHA-256")
        var size = 0L
        context.contentResolver.openInputStream(selection.uri)?.use { input ->
            destination.outputStream().buffered().use { output ->
                val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                while (true) {
                    val count = input.read(buffer)
                    if (count < 0) break
                    digest.update(buffer, 0, count)
                    output.write(buffer, 0, count)
                    size += count
                }
            }
        } ?: throw IOException("Evidence could not be opened")
        require(size > 0) { "Evidence files cannot be empty" }
        return EvidenceEntity(
            clientId = evidenceId,
            recordClientId = recordId,
            kind = selection.kind,
            localUri = Uri.fromFile(destination).toString(),
            originalFileName = selection.originalFileName,
            mimeType = selection.mimeType,
            sizeBytes = size,
            sha256 = digest.digest().joinToString("") { "%02x".format(it) },
            capturedAtEpochMs = selection.capturedAtEpochMs,
            latitude = selection.latitude,
            longitude = selection.longitude,
            transcription = selection.transcription,
        )
    }

    private fun validate(input: CaptureInput) {
        require(input.attested) { "Confirm the monitor attestation before saving" }
        require(input.projectId.isNotBlank() && input.plotId.isNotBlank() && input.activityId.isNotBlank()) {
            "Project, plot, and activity IDs are required"
        }
        require(input.species.isNotBlank() && input.methodologyVersion.isNotBlank()) {
            "Species and methodology version are required"
        }
        require(input.latitude in -90.0..90.0 && input.longitude in -180.0..180.0) { "GPS coordinates are out of range" }
        require(input.gpsAccuracyMeters.isFinite() && input.gpsAccuracyMeters > 0) { "GPS accuracy must be greater than zero" }
        require(input.plantedCount > 0 && input.aliveCount >= 0 && input.aliveCount <= input.plantedCount) {
            "Alive count must be between zero and the planted count"
        }
        require(input.averageHeightMeters.isFinite() && input.averageHeightMeters > 0) { "Average height must be greater than zero" }
    }

    companion object {
        const val SCHEMA_VERSION = 1
    }
}
