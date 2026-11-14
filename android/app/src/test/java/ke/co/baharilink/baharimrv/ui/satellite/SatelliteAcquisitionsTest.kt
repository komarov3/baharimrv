package ke.co.baharilink.baharimrv.ui.satellite

import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SatelliteAcquisitionsTest {
    private fun overlay(mode: String, scene: String?) = SatelliteOverlayDto(
        sceneId = scene, mission = "S2", acquiredAt = "2026-11-10T10:51:00Z",
        cloudCoverPct = 12.0, quality = "complete", adapterMode = mode, processingVersion = "pv-1",
    )

    @Test
    fun liveOverlayListsCurrentSceneWithServerMetadata() {
        val rows = matchedSceneAcquisitions(
            "proj", overlay("live", "CUR"), NdviChangeDto(baselineSceneId = "BASE", adapterMode = "live"),
        )
        assertEquals(2, rows.size)
        val current = rows[0]
        assertEquals("CUR", current.id)
        assertEquals("CUR", current.sceneId)
        assertEquals("acquisition", current.passType)
        assertEquals("2026-11-10T10:51:00Z", current.acquiredAt)
        assertEquals(12.0, current.cloudCoverPct!!, 0.0)
        assertEquals("live", current.adapterMode)
        assertEquals("complete", current.quality)
        val baseline = rows[1]
        assertEquals("BASE", baseline.id)
        assertEquals("baseline", baseline.passType)
        assertEquals("live", baseline.adapterMode)
        assertNull(baseline.acquiredAt)
    }

    @Test
    fun sandboxOverlayListsNothingSoLegacyRowsStayInCharge() {
        assertTrue(matchedSceneAcquisitions("proj", overlay("sandbox", "S"), null).isEmpty())
    }

    @Test
    fun liveOverlayWithoutASceneListsNothing() {
        assertTrue(matchedSceneAcquisitions("proj", overlay("live", null), null).isEmpty())
        assertTrue(matchedSceneAcquisitions("proj", overlay("live", "  "), null).isEmpty())
    }

    @Test
    fun nullOverlayListsNothing() {
        assertTrue(matchedSceneAcquisitions("proj", null, NdviChangeDto(baselineSceneId = "BASE")).isEmpty())
    }

    @Test
    fun baselineEqualToCurrentIsNotDuplicated() {
        val rows = matchedSceneAcquisitions("proj", overlay("live", "CUR"), NdviChangeDto(baselineSceneId = "CUR"))
        assertEquals(1, rows.size)
    }

    @Test
    fun noBaselineListsOnlyTheCurrentScene() {
        val rows = matchedSceneAcquisitions("proj", overlay("live", "CUR"), NdviChangeDto(baselineSceneId = null))
        assertEquals(1, rows.size)
        assertEquals("CUR", rows[0].id)
    }
}