package ke.co.baharilink.baharimrv.ui.satellite

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SatelliteMonitoringUiState(
    val projectId: String = "",
    val latestPass: LoadState<SatellitePassDto?> = LoadState.Success(null),
    val passes: LoadState<List<SatellitePassDto>> = LoadState.Success(emptyList()),
    val dashboard: LoadState<DashboardDto> = LoadState.Loading,
    val loading: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class SatelliteMonitoringViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(SatelliteMonitoringUiState())
    val state = _state.asStateFlow()

    fun setProjectId(value: String) = _state.update { it.copy(projectId = value) }

    fun load(projectId: String, mission: String) {
        _state.update { it.copy(projectId = projectId, loading = true, error = null) }
        viewModelScope.launch {
            try {
                val latest = repository.satelliteLatest(projectId, mission)
                val passes = repository.satellitePasses(projectId, mission)
                val dashboard = repository.dashboard(projectId)
                _state.update {
                    it.copy(
                        loading = false,
                        latestPass = LoadState.Success(latest),
                        passes = LoadState.Success(passes),
                        dashboard = LoadState.Success(dashboard),
                    )
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
}

data class PlotMarker(
    val id: String,
    val label: String,
    val latitude: Double,
    val longitude: Double,
    val ndvi: Double?,
)
