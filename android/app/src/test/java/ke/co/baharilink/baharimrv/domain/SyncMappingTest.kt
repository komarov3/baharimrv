package ke.co.baharilink.baharimrv.domain

import ke.co.baharilink.baharimrv.data.local.EvidenceEntity
import ke.co.baharilink.baharimrv.data.local.EvidenceKind
import ke.co.baharilink.baharimrv.data.local.MonitoringRecordEntity
import ke.co.baharilink.baharimrv.data.local.MonitoringWithEvidence
import ke.co.baharilink.baharimrv.data.local.SyncState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SyncMappingTest {
    private fun record(
        clientId: String = "11111111-1111-4111-8111-111111111111",
        averageHeightMeters: Double = 1.5,
    ) = MonitoringRecordEntity(
        clientId = clientId,
        projectId = "project-1",
        plotId = "plot-1",
        activityId = "activity-1",
        latitude = -6.7924,
        longitude = 39.2083,
        gpsAccuracyMeters = 4.0,
        plantedCount = 40,
        aliveCount = 36,
        averageHeightMeters = averageHeightMeters,
        species = "Rhizophora mucronata",
        deviceCapturedAtEpochMs = 1_730_000_000_123L,
        methodologyVersion = "VM001 v1.2",
        schemaVersion = 1,
        contentCommitment = "not-used-in-mapping",
        createdAtEpochMs = 1_730_000_000_000L,
        updatedAtEpochMs = 1_730_000_000_000L,
    )

    private fun evidence(clientId: String, sha256: String, name: String) = EvidenceEntity(
        clientId = clientId,
        recordClientId = "11111111-1111-4111-8111-111111111111",
        kind = EvidenceKind.PHOTO,
        localUri = "file:///evidence/$clientId",
        originalFileName = name,
        mimeType = "image/jpeg",
        sizeBytes = 1024L,
        sha256 = sha256,
        capturedAtEpochMs = 1_730_000_000_000L,
    )

    @Test
    fun quantitiesAreCanonicalisedAtSixDecimalsWithoutExponents() {
        assertEquals("1.500000", quantity(1.5))
        assertEquals("0.000000", quantity(0.0))
        assertEquals("123456789.123457", quantity(1.2345678912345678E8))
    }

    @Test
    fun deviceTimestampIsUtcWithMilliseconds() {
        assertEquals("2024-10-27T03:33:20.123Z", deviceTimestamp(1_730_000_000_123L))
    }

    @Test
    fun syncRecordCarriesUnsignedAttestationAndNestedLocation() {
        val syncRecord = MonitoringWithEvidence(record(), emptyList()).toSyncRecord()
        assertEquals("activity-1", syncRecord.activityId)
        assertEquals("1", syncRecord.schemaVersion)
        assertEquals("1.500000", syncRecord.averageHeightMetres)
        assertEquals(4.0, syncRecord.location.accuracyMetres, 0.0)
        assertEquals(ATTESTATION_STATEMENT, syncRecord.attestation.statement)
        assertNull(syncRecord.attestation.identityDid)
        assertNull(syncRecord.attestation.signature)
    }

    @Test
    fun manifestIsOrderedByHashThenName() {
        val later = evidence("b", "bbb", "second.jpg")
        val earlier = evidence("a", "aaa", "first.jpg")
        val sameHashOtherName = evidence("c", "aaa", "aardvark.jpg")
        val manifest = MonitoringWithEvidence(record(), listOf(later, sameHashOtherName, earlier))
            .toSyncRecord()
            .evidenceManifest
        assertEquals(listOf("aardvark.jpg", "first.jpg", "second.jpg"), manifest.map { it.name })
        assertEquals(listOf("photo", "photo", "photo"), manifest.map { it.kind })
    }

    @Test
    fun batchUsesTheServerActionTypeAndSortsByClientId() {
        val first = record(clientId = "aaa")
        val second = record(clientId = "bbb")
        val batch = SyncPayloadFactory.create(
            listOf(MonitoringWithEvidence(second, emptyList()), MonitoringWithEvidence(first, emptyList())),
        )
        assertEquals(2, batch.actions.size)
        assertEquals("monitoring_record", batch.actions.first().type)
        assertEquals(listOf("aaa", "bbb"), batch.actions.map { it.record.clientId })
    }

    @Test
    fun batchRejectsEmptyAndOversizedPayloads() {
        assertNotNull(failureMessage { SyncPayloadFactory.create(emptyList()) })
        val tooMany = (1..51).map { MonitoringWithEvidence(record(clientId = "id-%02d".format(it)), emptyList()) }
        assertTrue(failureMessage { SyncPayloadFactory.create(tooMany) }!!.contains("at most 50"))
    }

    @Test
    fun serverStatusesMapToDeviceStates() {
        assertEquals(SyncState.SUBMITTED, syncStateFromServer("submitted"))
        assertEquals(SyncState.SYNC_CONFLICT, syncStateFromServer("sync_conflict"))
        assertEquals(SyncState.SYNC_FAILED, syncStateFromServer("failed"))
        assertNull(syncStateFromServer("something_new"))
    }

    @Test
    fun commitmentIsStableAcrossEvidenceOrderAndSensitiveToMeasurements() {
        val items = listOf(evidence("a", "aaa", "first.jpg"), evidence("b", "bbb", "second.jpg"))
        val reversed = listOf(items[1], items[0])
        val base = CanonicalMonitoring.commitment(record(), items)
        assertEquals(base, CanonicalMonitoring.commitment(record(), reversed))
        assertTrue(base != CanonicalMonitoring.commitment(record(averageHeightMeters = 1.6), items))
    }

    private fun failureMessage(block: () -> Unit): String? =
        try {
            block()
            null
        } catch (error: Throwable) {
            error.message
        }
}
