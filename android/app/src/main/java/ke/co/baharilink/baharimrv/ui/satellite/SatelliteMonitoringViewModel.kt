package ke.co.baharilink.baharimrv.ui.satellite

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import javax.inject.Named
import kotlin.coroutines.coroutineContext
import kotlinx.coroutines.isActive
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

data class SatelliteMonitoringUiState(
    val projectId: String = "",
    val latestPass: LoadState<SatellitePassDto?> = LoadState.Success(null),
    val passes: LoadState<List<SatellitePassDto>> = LoadState.Success(emptyList()),
    val dashboard: LoadState<DashboardDto> = LoadState.Loading,
    val overlay: LoadState<SatelliteOverlayDto?> = LoadState.Success(null),
    val change: LoadState<NdviChangeDto?> = LoadState.Success(null),
    val refreshJob: SatelliteRefreshDto? = null,
    val refreshError: String? = null,
    val activeScene: String? = null,
    val cachedImagery: Boolean = false,
    val loading: Boolean = false,
    val error: String? = null,
)

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
    private var pollJob: Job? = null

    fun setProjectId(value: String) = _state.update { it.copy(projectId = value) }

    /**
     * Fetches overlay, change, latest pass, and passes in one coroutine. The
     * screen only fails over if overlay itself fails; change and passes are
     * best-effort so a partial backend state still draws the map.
     */
    fun load(projectId: String, mission: String) {
        _state.update { it.copy(projectId = projectId, loading = true, error = null) }
        viewModelScope.launch {
            try {
                coroutineScope {
                    val overlayDeferred = async { runCatching { repository.satelliteOverlay(projectId) } }
                    val changeDeferred = async { runCatching { repository.satelliteChange(projectId) } }
                    val latestDeferred = async { runCatching { repository.satelliteLatest(projectId, mission.ifBlank { "S2" }) } }
                    val passesDeferred = async { runCatching { repository.satellitePasses(projectId, mission.ifBlank { "S2" }) } }
                    val dashboardDeferred = async { runCatching { repository.dashboard(projectId) } }

                    val overlayResult = overlayDeferred.await()
                    val changeResult = changeDeferred.await()
                    val latest = latestDeferred.await().getOrNull()
                    val passes = passesDeferred.await().getOrDefault(emptyList())
                    val dashboard = dashboardDeferred.await().getOrNull()

                    overlayResult.onFailure { throwable ->
                        _state.update {
                            it.copy(loading = false, error = throwable.readableMessage())
                        }
                        return@coroutineScope
                    }
                    val overlay = overlayResult.getOrNull()
                    _state.update {
                        it.copy(
                            loading = false,
                            overlay = LoadState.Success(overlay),
                            change = LoadState.Success(changeResult.getOrNull()),
                            latestPass = LoadState.Success(latest),
                            passes = LoadState.Success(passes),
                            dashboard = dashboard?.let { d -> LoadState.Success(d) } ?: it.dashboard,
                            activeScene = overlay?.sceneId ?: it.activeScene,
                        )
                    }
                }
            } catch (e: Exception) {
                _state.update { it.copy(loading = false, error = e.readableMessage()) }
            }
        }
    }

    fun retry() {
        val current = _state.value
        if (current.projectId.isNotBlank()) {
            load(current.projectId, "")
        }
    }

    /**
     * Posts a refresh and polls the job status every 3 s up to a 90 s deadline.
     * A second tap while a job is already open answers with the same job id
     * (409), which we just adopt as the current one.
     */
    fun refresh() {
        val projectId = _state.value.projectId
        if (projectId.isBlank()) return
        pollJob?.cancel()
        _state.update { it.copy(refreshError = null) }
        pollJob = viewModelScope.launch {
            val submitted = runCatching { repository.satelliteRefresh(projectId) }
            if (submitted.isFailure) {
                _state.update { it.copy(refreshError = submitted.exceptionOrNull()?.readableMessage()) }
                return@launch
            }
            val initial = submitted.getOrThrow()
            _state.update { it.copy(refreshJob = initial) }
            pollUntilTerminal(projectId, initial.jobId)
        }
    }

    private suspend fun pollUntilTerminal(projectId: String, jobId: String) {
        val deadline = System.currentTimeMillis() + REFRESH_DEADLINE_MS
        while (coroutineContext.isActive && System.currentTimeMillis() < deadline) {
            delay(REFRESH_POLL_INTERVAL_MS)
            val view = runCatching { repository.satelliteRefreshStatus(projectId, jobId) }.getOrNull() ?: continue
            _state.update { it.copy(refreshJob = view) }
            if (view.status == "complete" || view.status == "failed" || view.status == "cancelled") {
                if (view.status == "complete") {
                    _state.update { it.copy(refreshError = null) }
                    load(projectId, "")
                } else if (view.status == "failed") {
                    _state.update { it.copy(refreshError = view.error ?: "The imagery refresh could not be completed") }
                }
                return
            }
        }
        _state.update {
            it.copy(refreshError = it.refreshError ?: "The imagery refresh is still running; open the screen again in a moment")
        }
    }

    /**
     * Invoked by the tile provider on its binder thread when the server answers
     * 410 — the scene this provider was built for is no longer the active
     * one. Refetch the overlay once; the screen rebuilds the provider when
     * `activeScene` changes.
     */
    fun sceneSuperseded() {
        viewModelScope.launch {
            supersedeLock.withLock {
                val projectId = _state.value.projectId
                if (projectId.isBlank()) return@launch
                val fresh = runCatching { repository.satelliteOverlay(projectId) }.getOrNull() ?: return@launch
                _state.update { it.copy(overlay = LoadState.Success(fresh), activeScene = fresh.sceneId) }
            }
        }
    }

    /**
     * Build or reuse the tile provider for the current overlay. The provider
     * is keyed on the active scene so camera moves or plot edits do not churn
     * it; it is rebuilt only when [SatelliteMonitoringUiState.activeScene]
     * changes.
     */
    fun tileProvider(): NdviTileProvider? {
        val overlay = (_state.value.overlay as? LoadState.Success)?.data ?: return null
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
