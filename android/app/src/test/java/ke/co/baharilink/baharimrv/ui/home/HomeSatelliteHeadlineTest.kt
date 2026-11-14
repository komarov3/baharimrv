package ke.co.baharilink.baharimrv.ui.home

import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.NdviDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class HomeSatelliteHeadlineTest {
    private fun change(mode: String, quality: String): NdviChangeDto =
        NdviChangeDto(adapterMode = mode, quality = quality, meanNdvi = 0.1, coveragePct = 80.0, plotCount = 1)

    @Test
    fun noSummaryReadsNoImageryNotSandbox() {
        val state = satelliteHeadlineState(null, NdviDto(status = "unavailable", source = "No corroboration attached"))
        assertEquals("no imagery", state.chipText)
        assertFalse(state.isSandbox)
        assertNull(state.liveChange)
    }

    @Test
    fun noSummaryPendingIsNotSandbox() {
        val state = satelliteHeadlineState(null, NdviDto(status = "pending"))
        assertEquals("no imagery", state.chipText)
        assertFalse(state.isSandbox)
    }

    @Test
    fun nothingReturnedIsNotSandbox() {
        val state = satelliteHeadlineState(null, null)
        assertEquals("no imagery", state.chipText)
        assertFalse(state.isSandbox)
    }

    @Test
    fun liveChangeShowsLive() {
        val state = satelliteHeadlineState(change("live", "complete"), NdviDto(status = "available", adapterMode = "live"))
        assertEquals("live", state.chipText)
        assertFalse(state.isSandbox)
        assertNotNull(state.liveChange)
    }

    @Test
    fun sandboxChangeShowsSandbox() {
        val state = satelliteHeadlineState(change("sandbox", "complete"), null)
        assertEquals("sandbox", state.chipText)
        assertTrue(state.isSandbox)
    }

    @Test
    fun sandboxUnavailableStillSandbox() {
        val state = satelliteHeadlineState(change("sandbox", "unavailable"), null)
        assertEquals("sandbox", state.chipText)
        assertTrue(state.isSandbox)
        assertNull(state.liveChange)
    }

    @Test
    fun legacySandboxModeIsHonored() {
        val state = satelliteHeadlineState(null, NdviDto(status = "available", adapterMode = "sandbox"))
        assertEquals("sandbox", state.chipText)
        assertTrue(state.isSandbox)
    }

    @Test
    fun liveUnavailableIsNotSandbox() {
        val state = satelliteHeadlineState(change("live", "unavailable"), null)
        assertEquals("no imagery", state.chipText)
        assertFalse(state.isSandbox)
        assertNull(state.liveChange)
    }
}