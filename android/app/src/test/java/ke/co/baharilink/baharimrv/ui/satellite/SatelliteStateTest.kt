package ke.co.baharilink.baharimrv.ui.satellite

import java.io.IOException
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteRefreshDto
import ke.co.baharilink.baharimrv.ui.components.LoadState
import kotlinx.coroutines.CancellationException
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

class SatelliteStateTest {
    private fun httpError(code: Int, body: String): HttpException =
        HttpException(Response.error<Any>(code, body.toResponseBody("application/json".toMediaType())))

    private fun pass(mode: String?): SatellitePassDto =
        SatellitePassDto(id = "p", projectId = "proj", mission = "S2", passType = "acquisition", adapterMode = mode)

    private fun stateWithOverlay(overlay: LoadState<SatelliteSummary>): SatelliteMonitoringUiState =
        SatelliteMonitoringUiState(projectId = "proj", overlay = overlay)

    @Test
    fun summaryMissingCodeIsTypedMissing() {
        val failure = httpError(404, """{"error":{"code":"satellite_summary_missing","message":"none"}}""").satelliteFailure()
        assertEquals(SatelliteFailure.SummaryMissing, failure)
    }

    @Test
    fun unrelatedNotFoundStaysAnError() {
        val failure = httpError(404, """{"error":{"code":"not_found","message":"project gone"}}""").satelliteFailure()
        assertTrue(failure is SatelliteFailure.Error)
    }

    @Test
    fun rateLimitUsesServerMessage() {
        val message = "This project has already requested 6 imagery refreshes in the last hour. Try again later."
        val failure = httpError(429, """{"error":{"code":"satellite_refresh_rate_limited","message":"$message"}}""").satelliteFailure()
        assertEquals(message, (failure as SatelliteFailure.Error).message)
    }

    @Test
    fun rateLimitWithForeignCodeFallsBack() {
        val failure = httpError(429, """{"error":{"code":"something_else","message":"raw worker trace"}}""").satelliteFailure()
        assertEquals(SATELLITE_RATE_LIMIT_MESSAGE, (failure as SatelliteFailure.Error).message)
    }

    @Test
    fun rateLimitWithMalformedBodyFallsBack() {
        val failure = httpError(429, "not json").satelliteFailure()
        assertEquals(SATELLITE_RATE_LIMIT_MESSAGE, (failure as SatelliteFailure.Error).message)
    }

    @Test
    fun serverErrorIsAnErrorNotSandbox() {
        val failure = httpError(500, """{"error":{"code":"internal","message":"boom"}}""").satelliteFailure()
        assertTrue((failure as SatelliteFailure.Error).message.contains("500"))
    }

    @Test
    fun networkFailureIsAnError() {
        val failure = IOException("offline").satelliteFailure()
        assertTrue(failure is SatelliteFailure.Error)
    }

    @Test(expected = CancellationException::class)
    fun cancellationPropagates() {
        CancellationException("cancelled").satelliteFailure()
    }

    @Test
    fun onlyExplicitSandboxAdapterIsSandbox() {
        assertFalse(stateWithOverlay(LoadState.Loading).isSandbox)
        assertFalse(stateWithOverlay(LoadState.Error("down")).isSandbox)
        assertFalse(stateWithOverlay(LoadState.Success(SatelliteSummary.Missing)).isSandbox)
        assertFalse(stateWithOverlay(LoadState.Success(SatelliteSummary.Available(SatelliteOverlayDto(adapterMode = "live")))).isSandbox)
        assertTrue(stateWithOverlay(LoadState.Success(SatelliteSummary.Available(SatelliteOverlayDto(adapterMode = "sandbox")))).isSandbox)
    }

    @Test
    fun missingSummaryIsDistinctFromError() {
        assertTrue(stateWithOverlay(LoadState.Success(SatelliteSummary.Missing)).summaryMissing)
        assertFalse(stateWithOverlay(LoadState.Error("down")).summaryMissing)
        assertFalse(stateWithOverlay(LoadState.Success(SatelliteSummary.Available(SatelliteOverlayDto()))).summaryMissing)
    }

    @Test
    fun syntheticPassesHiddenUnlessSummaryIsSandbox() {
        val live = stateWithOverlay(LoadState.Success(SatelliteSummary.Available(SatelliteOverlayDto(adapterMode = "live"))))
        assertTrue(live.canShowLegacyPass(pass("live")))
        assertFalse(live.canShowLegacyPass(pass("sandbox")))
        assertFalse(live.canShowLegacyPass(pass(null)))

        val sandbox = stateWithOverlay(LoadState.Success(SatelliteSummary.Available(SatelliteOverlayDto(adapterMode = "sandbox"))))
        assertTrue(sandbox.canShowLegacyPass(pass("sandbox")))
        assertTrue(sandbox.canShowLegacyPass(pass("live")))

        val unknown = stateWithOverlay(LoadState.Error("down"))
        assertFalse(unknown.canShowLegacyPass(pass("sandbox")))
        assertFalse(unknown.canShowLegacyPass(pass(null)))
        assertTrue(unknown.canShowLegacyPass(pass("live")))
    }

    @Test
    fun refreshInFlightTracksSubmissionAndOpenJob() {
        assertTrue(SatelliteMonitoringUiState(refreshing = true).refreshInFlight)
        assertTrue(SatelliteMonitoringUiState(refreshJob = SatelliteRefreshDto(jobId = "j", status = "queued")).refreshInFlight)
        assertTrue(SatelliteMonitoringUiState(refreshJob = SatelliteRefreshDto(jobId = "j", status = "running")).refreshInFlight)
        assertFalse(SatelliteMonitoringUiState(refreshJob = SatelliteRefreshDto(jobId = "j", status = "complete")).refreshInFlight)
        assertFalse(SatelliteMonitoringUiState(refreshJob = SatelliteRefreshDto(jobId = "j", status = "failed")).refreshInFlight)
        assertFalse(SatelliteMonitoringUiState().refreshInFlight)
    }
}
