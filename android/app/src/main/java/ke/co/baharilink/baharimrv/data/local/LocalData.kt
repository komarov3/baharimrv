package ke.co.baharilink.baharimrv.data.local

import androidx.room.Dao
import androidx.room.Database
import androidx.room.Embedded
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Relation
import androidx.room.RoomDatabase
import androidx.room.Transaction
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.Update
import kotlinx.coroutines.flow.Flow

enum class SyncState {
    ON_DEVICE,
    UPLOADING,
    SUBMITTED,
    AWAITING_VERIFICATION,
    ANCHORING,
    ANCHORED,
    VERIFICATION_FAILED,
    SYNC_CONFLICT,
    SYNC_FAILED,
}

enum class EvidenceKind { PHOTO, VOICE }
enum class EvidenceUploadState { PENDING, UPLOADING, UPLOADED, FAILED }

@Entity(
    tableName = "monitoring_records",
    indices = [Index("syncState"), Index("serverId", unique = true), Index("activityId")],
)
data class MonitoringRecordEntity(
    @PrimaryKey val clientId: String,
    val serverId: String? = null,
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
    val deviceCapturedAtEpochMs: Long,
    val serverReceiptAtEpochMs: Long? = null,
    val methodologyVersion: String,
    val schemaVersion: Int,
    val syncState: SyncState = SyncState.ON_DEVICE,
    val contentCommitment: String,
    val attestationStatus: String = "IDENTITY_SIGNATURE_PENDING",
    val syncAttempts: Int = 0,
    val lastError: String? = null,
    val createdAtEpochMs: Long,
    val updatedAtEpochMs: Long,
)

@Entity(
    tableName = "evidence_objects",
    foreignKeys = [
        ForeignKey(
            entity = MonitoringRecordEntity::class,
            parentColumns = ["clientId"],
            childColumns = ["recordClientId"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
    indices = [Index("recordClientId"), Index("sha256")],
)
data class EvidenceEntity(
    @PrimaryKey val clientId: String,
    val serverId: String? = null,
    val recordClientId: String,
    val kind: EvidenceKind,
    val localUri: String,
    val originalFileName: String,
    val mimeType: String,
    val sizeBytes: Long,
    val sha256: String,
    val capturedAtEpochMs: Long,
    val latitude: Double? = null,
    val longitude: Double? = null,
    val transcription: String? = null,
    val uploadState: EvidenceUploadState = EvidenceUploadState.PENDING,
)

data class MonitoringWithEvidence(
    @Embedded val record: MonitoringRecordEntity,
    @Relation(parentColumn = "clientId", entityColumn = "recordClientId")
    val evidence: List<EvidenceEntity>,
)

class RoomConverters {
    @TypeConverter fun syncState(value: SyncState): String = value.name
    @TypeConverter fun syncState(value: String): SyncState = SyncState.valueOf(value)
    @TypeConverter fun evidenceKind(value: EvidenceKind): String = value.name
    @TypeConverter fun evidenceKind(value: String): EvidenceKind = EvidenceKind.valueOf(value)
    @TypeConverter fun evidenceState(value: EvidenceUploadState): String = value.name
    @TypeConverter fun evidenceState(value: String): EvidenceUploadState = EvidenceUploadState.valueOf(value)
}

@Dao
interface MonitoringDao {
    @Transaction
    @Query("SELECT * FROM monitoring_records ORDER BY createdAtEpochMs DESC")
    fun observeAll(): Flow<List<MonitoringWithEvidence>>

    @Transaction
    @Query("SELECT * FROM monitoring_records WHERE clientId = :clientId")
    suspend fun get(clientId: String): MonitoringWithEvidence?

    @Query("SELECT * FROM monitoring_records WHERE syncState IN ('ON_DEVICE', 'SYNC_FAILED') ORDER BY createdAtEpochMs")
    suspend fun pendingRecords(): List<MonitoringRecordEntity>

    @Query("SELECT COUNT(*) FROM monitoring_records WHERE syncState IN ('ON_DEVICE', 'UPLOADING', 'SYNC_FAILED')")
    fun observePendingCount(): Flow<Int>

    @Insert(onConflict = OnConflictStrategy.ABORT)
    suspend fun insert(record: MonitoringRecordEntity)

    @Update
    suspend fun update(record: MonitoringRecordEntity)

    @Query("UPDATE monitoring_records SET syncState = 'UPLOADING', syncAttempts = syncAttempts + 1, updatedAtEpochMs = :timestamp WHERE clientId IN (:clientIds)")
    suspend fun markUploading(clientIds: List<String>, timestamp: Long)
}

@Dao
interface EvidenceDao {
    @Insert(onConflict = OnConflictStrategy.ABORT)
    suspend fun insertAll(evidence: List<EvidenceEntity>)

    @Query("SELECT * FROM evidence_objects WHERE recordClientId = :recordClientId ORDER BY capturedAtEpochMs")
    suspend fun forRecord(recordClientId: String): List<EvidenceEntity>

    /**
     * Media waits for its record's server ID because the upload endpoint is keyed by
     * the stored monitoring record, not the device's own identifier.
     */
    @Query(
        """
        SELECT evidence_objects.* FROM evidence_objects
        INNER JOIN monitoring_records ON monitoring_records.clientId = evidence_objects.recordClientId
        WHERE monitoring_records.serverId IS NOT NULL
          AND evidence_objects.uploadState IN ('PENDING', 'UPLOADING', 'FAILED')
        ORDER BY evidence_objects.capturedAtEpochMs
        LIMIT :limit
        """
    )
    suspend fun pendingUploads(limit: Int): List<EvidenceEntity>

    @Query("UPDATE evidence_objects SET uploadState = 'UPLOADED', serverId = :serverId WHERE clientId = :clientId")
    suspend fun markUploaded(clientId: String, serverId: String)

    @Query("UPDATE evidence_objects SET uploadState = 'FAILED' WHERE clientId = :clientId")
    suspend fun markFailed(clientId: String)

    @Query("SELECT COUNT(*) FROM evidence_objects WHERE uploadState NOT IN ('UPLOADED')")
    fun observePendingUploadCount(): Flow<Int>

    @Update
    suspend fun update(evidence: EvidenceEntity)
}

@Database(
    entities = [MonitoringRecordEntity::class, EvidenceEntity::class],
    version = 1,
    exportSchema = true,
)
@TypeConverters(RoomConverters::class)
abstract class BahariMrvDatabase : RoomDatabase() {
    abstract fun monitoringDao(): MonitoringDao
    abstract fun evidenceDao(): EvidenceDao
}
