package ke.co.baharilink.baharimrv.ui.satellite

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import javax.inject.Named
import com.google.gson.JsonParser
import kotlin.coroutines.coroutineContext
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.isActive
import retrofit2.HttpException
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteRefreshDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import okhttp3.OkHttpClient

sealed interface SatelliteSummary {
    data object Missing : SatelliteSummary
    data class Available(val overlay: SatelliteOverlayDto) : SatelliteSummary
}

data class SatelliteMonitoringUiState(
    val projectId: String = "",
    val latestPass: LoadState<SatellitePassDto?> = LoadState.Loading,
    val passes: LoadState<List<SatellitePassDto>> = LoadState.Loading,
    val dashboard: LoadState<DashboardDto> = LoadState.Loading,
    val overlay: LoadState<SatelliteSummary> = LoadState.Loading,
    val change: LoadState<NdviChangeDto?> = LoadState.Loading,
    val refreshJob: SatelliteRefreshDto? = null,
    val refreshError: String? = null,
    val refreshing: Boolean = false,
    val activeScene: String? = null,
    val cachedImagery: Boolean = false,
    val loading: Boolean = false,
) {
    val overlayData: SatelliteOverlayDto?
        get() = ((overlay as? LoadState.Success)?.data as? SatelliteSummary.Available)?.overlay
    val isSandbox: Boolean
        get() = overlayData?.adapterMode.equals("sandbox", ignoreCase = true)
    val summaryMissing: Boolean
        get() = (overlay as? LoadState.Success)?.data == SatelliteSummary.Missing
    val refreshInFlight: Boolean
        get() = refreshing || refreshJob?.isInFlight == true

    // Legacy endpoints use SATELLITE_MODE, independently of the summary provider.
    // Never mix their synthetic (or unlabelled) rows into live/unknown imagery.
    fun canShowLegacyPass(pass: SatellitePassDto): Boolean =
        pass.adapterMode.equals("live", ignoreCase = true) ||
            (isSandbox && pass.adapterMode.equals("sandbox", ignoreCase = true))
}

private val SatelliteRefreshDto.isInFlight: Boolean
    get() = status !in setOf("complete", "failed", "cancelled")

/**
 * The acquisitions a live project actually matched: the summary's current scene and,
 * when present, the year-earlier baseline. The legacy pass predictor only knows the
 * sandbox seam, so under a live summary its synthetic rows are hidden and these are
 * the only honest acquisitions to list. Every field comes from a server envelope.
 */
internal fun matchedSceneAcquisitions(
    projectId: String,
    overlay: SatelliteOverlayDto?,
    change: NdviChangeDto?,
): List<SatellitePassDto> {
    if (overlay == null || !overlay.adapterMode.equals("live", ignoreCase = true)) return emptyList()
    val current = overlay.sceneId?.takeIf { it.isNotBlank() } ?: return emptyList()
    val rows = mutableListOf(
        SatellitePassDto(
            id = current, projectId = projectId, mission = overlay.mission, passType = "acquisition",
            acquiredAt = overlay.acquiredAt, sceneId = current, cloudCoverPct = overlay.cloudCoverPct,
            adapterMode = overlay.adapterMode, processingVersion = overlay.processingVersion, quality = overlay.quality,
        ),
    )
    val baseline = change?.baselineSceneId?.takeIf { it.isNotBlank() && it != current }
    if (baseline != null) {
        rows.add(
            SatellitePassDto(
                id = baseline, projectId = projectId, mission = overlay.mission, passType = "baseline",
                sceneId = baseline, adapterMode = overlay.adapterMode, processingVersion = change.processingVersion,
            ),
        )
    }
    return rows
}

internal sealed interface SatelliteFailure {
    data object SummaryMissing : SatelliteFailure
    data class Error(val message: String) : SatelliteFailure
}

internal const val SATELLITE_RATE_LIMIT_MESSAGE =
    "Too many imagery requests. Wait before trying again; the project refresh limit resets over the next hour."

/** Only the documented client-safe error envelope is read, never raw response/worker text. */
internal fun Throwable.satelliteFailure(): SatelliteFailure {
    if (this is CancellationException) throw this
    if (this !is HttpException) return SatelliteFailure.Error(readableMessage())
    val envelope = runCatching {
        response()?.errorBody()?.use { body ->
            JsonParser.parseString(body.string()).asJsonObject.getAsJsonObject("error")
        }
    }.getOrNull()
    val code = runCatching { envelope?.get("code")?.asString }.getOrNull()
    if (code() == 404 && code == "satellite_summary_missing") return SatelliteFailure.SummaryMissing
    if (code() == 429) {
        val safeMessage = if (code == "satellite_refresh_rate_limited") {
            runCatching { envelope?.get("message")?.asString }.getOrNull()
                ?.trim()?.takeIf { it.isNotBlank() }
        } else null
        return SatelliteFailure.Error(safeMessage ?: SATELLITE_RATE_LIMIT_MESSAGE)
    }
    return SatelliteFailure.Error("Satellite data could not be loaded (HTTP ${code()}). Try again.")
}

private suspend fun <T> satelliteLoadState(
    onMissing: (() -> T)? = null,
    request: suspend () -> T,
): LoadState<T> = try {
    val value = request()
    coroutineContext.ensureActive()
    LoadState.Success(value)
} catch (cancelled: CancellationException) {
    throw cancelled
} catch (error: Exception) {
    coroutineContext.ensureActive()
    when (val failure = error.satelliteFailure()) {
        SatelliteFailure.SummaryMissing -> onMissing?.let { LoadState.Success(it()) }
            ?: LoadState.Error("No satellite summary is available yet. Check for new imagery.")
        is SatelliteFailure.Error -> LoadState.Error(failure.message)
    }
}

@HiltViewModel
class SatelliteMonitoringViewModel @Inject constructor(
    private val repository: ProjectRepository,
    @Named("tiles") private val tilesClient: OkHttpClient,
) : ViewModel() {
    private val _state = MutableStateFlow(SatelliteMonitoringUiState())
    val state = _state.asStateFlow()

    private val baseUrl: String = BuildConfig.API_BASE_URL.trim().let {
        if (it.isBlank()) "https://configuration.invalid/" else if (it.endsWith('/')) it else "$it/"
    }.trimEnd('/')

    @Volatile
    private var currentTileProvider: NdviTileProvider? = null
    private val supersedeLock = Mutex()
    private var loadJob: Job? = null
    private var pollJob: Job? = null

    fun setProjectId(value: String) = _state.update { it.copy(projectId = value) }

    fun load(projectId: String, mission: String) {
        loadJob?.cancel()
        val previous = _state.value
        val sameProject = previous.projectId == projectId
        if (!sameProject) currentTileProvider = null
        _state.update {
            SatelliteMonitoringUiState(
                projectId = projectId,
                loading = true,
                refreshJob = if (sameProject) previous.refreshJob else null,
                refreshError = if (sameProject) previous.refreshError else null,
                refreshing = sameProject && previous.refreshing,
            )
        }
        loadJob = viewModelScope.launch {
            val resolvedMission = mission.ifBlank { "S2" }
            coroutineScope {
                val overlayDeferred = async {
                    satelliteLoadState<SatelliteSummary>(onMissing = { SatelliteSummary.Missing }) {
                        SatelliteSummary.Available(repository.satelliteOverlay(projectId))
                    }
                }
                val changeDeferred = async {
                    satelliteLoadState<NdviChangeDto?>(onMissing = { null }) { repository.satelliteChange(projectId) }
                }
                val latestDeferred = async {
                    satelliteLoadState<SatellitePassDto?> { repository.satelliteLatest(projectId, resolvedMission) }
                }
                val passesDeferred = async {
                    satelliteLoadState { repository.satellitePasses(projectId, resolvedMission) }
                }
                val dashboardDeferred = async {
                    satelliteLoadState { repository.dashboard(projectId) }
                }

                val overlayState = overlayDeferred.await()
                val scene = ((overlayState as? LoadState.Success)?.data as? SatelliteSummary.Available)?.overlay?.sceneId
                coroutineContext.ensureActive()
                _state.update {
                    it.copy(
                        loading = false,
                        overlay = overlayState,
                        change = changeDeferred.await(),
                        latestPass = latestDeferred.await(),
                        passes = passesDeferred.await(),
                        dashboard = dashboardDeferred.await(),
                        activeScene = scene,
                    )
                }
            }
        }
    }

    fun retry() {
        val current = _state.value
        if (current.projectId.isNotBlank()) {
            load(current.projectId, "")
        }
    }

    fun refresh() {
        val current = _state.value
        val projectId = current.projectId
        if (projectId.isBlank() || current.refreshing) return
        pollJob?.cancel()
        _state.update { it.copy(refreshError = null, refreshing = true) }
        val openJob = current.refreshJob?.takeIf { it.isInFlight }
        pollJob = viewModelScope.launch {
            try {
                if (openJob != null) {
                    // Polling timed out earlier; resume the same job instead of spending another quota slot.
                    pollUntilTerminal(projectId, openJob.jobId)
                    return@launch
                }
                val initial = try {
                    repository.satelliteRefresh(projectId)
                } catch (cancelled: CancellationException) {
                    throw cancelled
                } catch (error: Exception) {
                    val message = when (val failure = error.satelliteFailure()) {
                        SatelliteFailure.SummaryMissing -> "No satellite summary is available yet. Check for new imagery."
                        is SatelliteFailure.Error -> failure.message
                    }
                    _state.update { it.copy(refreshError = message) }
                    return@launch
                }
                _state.update { it.copy(refreshJob = initial) }
                pollUntilTerminal(projectId, initial.jobId)
            } finally {
                _state.update { it.copy(refreshing = false) }
            }
        }
    }

    private suspend fun pollUntilTerminal(projectId: String, jobId: String) {
        val deadline = System.currentTimeMillis() + REFRESH_DEADLINE_MS
        var lastPollError: String? = null
        while (coroutineContext.isActive && System.currentTimeMillis() < deadline) {
            delay(REFRESH_POLL_INTERVAL_MS)
            val view = try {
                repository.satelliteRefreshStatus(projectId, jobId)
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (error: Exception) {
                lastPollError = (error.satelliteFailure() as? SatelliteFailure.Error)?.message
                continue
            }
            lastPollError = null
            _state.update { it.copy(refreshJob = view) }
            when (view.status) {
                "complete" -> {
                    _state.update { it.copy(refreshError = null) }
                    load(projectId, "")
                    return
                }
                "failed" -> {
                    _state.update {
                        it.copy(refreshError = view.error?.takeIf { text -> text.isNotBlank() }
                            ?: "The imagery refresh could not be completed. Try again.")
                    }
                    return
                }
                "cancelled" -> return
            }
        }
        _state.update {
            it.copy(
                refreshError = it.refreshError ?: lastPollError
                    ?: "The imagery refresh is still running. Tap again to keep checking its progress.",
            )
        }
    }

    fun sceneSuperseded() {
        viewModelScope.launch {
            supersedeLock.withLock {
                val projectId = _state.value.projectId
                if (projectId.isBlank()) return@launch
                val fresh = try {
                    repository.satelliteOverlay(projectId)
                } catch (cancelled: CancellationException) {
                    throw cancelled
                } catch (_: Exception) {
                    return@launch
                }
                if (_state.value.projectId != projectId) return@launch
                _state.update {
                    it.copy(overlay = LoadState.Success(SatelliteSummary.Available(fresh)), activeScene = fresh.sceneId)
                }
            }
        }
    }

    fun tileProvider(): NdviTileProvider? {
        val overlay = _state.value.overlayData ?: return null
        val scene = overlay.sceneId ?: return null
        currentTileProvider?.let { if (it.isForScene(scene)) return it }
        val provider = NdviTileProvider(
            projectId = _state.value.projectId,
            source = overlay,
            baseUrl = baseUrl,
            tilesClient = tilesClient,
            onSceneSuperseded = ::sceneSuperseded,
        )
        currentTileProvider = provider
        return provider
    }

    fun plotMarkers(): List<PlotMarker> {
        val dashboard = (_state.value.dashboard as? LoadState.Success)?.data
        return dashboard?.plots?.map { plot ->
            PlotMarker(
                id = plot.id,
                label = plot.label,
                latitude = plot.latitude,
                longitude = plot.longitude,
                ndvi = plot.ndvi,
            )
        } ?: emptyList()
    }

    fun acquisitions(): List<SatellitePassDto> {
        val current = _state.value
        val matched = matchedSceneAcquisitions(
            current.projectId, current.overlayData, (current.change as? LoadState.Success)?.data,
        )
        if (matched.isNotEmpty()) return matched
        return (current.passes as? LoadState.Success)?.data ?: emptyList()
    }

    companion object {
        private const val REFRESH_POLL_INTERVAL_MS = 3_000L
        private const val REFRESH_DEADLINE_MS = 90_000L
    }
}

data class PlotMarker(
    val id: String,
    val label: String,
    val latitude: Double,
    val longitude: Double,
    val ndvi: Double?,
)
