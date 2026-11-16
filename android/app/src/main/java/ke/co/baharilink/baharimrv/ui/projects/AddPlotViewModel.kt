package ke.co.baharilink.baharimrv.ui.projects

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.CreatePlotRequestDto
import ke.co.baharilink.baharimrv.data.remote.GeoPolygonDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class AddPlotUiState(
    val saving: Boolean = false,
    val error: String? = null,
    val savedPlotId: String? = null,
)

@HiltViewModel
class AddPlotViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(AddPlotUiState())
    val state = _state.asStateFlow()

    fun save(
        projectId: String,
        siteName: String,
        code: String,
        latitude: Double,
        longitude: Double,
        areaHectares: String,
        boundary: GeoPolygonDto?,
        gpsAccuracyM: Double?,
    ) {
        viewModelScope.launch {
            _state.update { it.copy(saving = true, error = null, savedPlotId = null) }
            try {
                val response = repository.createPlot(
                    projectId,
                    CreatePlotRequestDto(
                        siteName = siteName.trim(),
                        code = code.trim(),
                        latitude = latitude,
                        longitude = longitude,
                        areaHectares = areaHectares.trim(),
                        boundary = boundary,
                        gpsAccuracyM = gpsAccuracyM,
                    ),
                )
                _state.update { it.copy(saving = false, savedPlotId = response.id) }
            } catch (error: Exception) {
                _state.update { it.copy(saving = false, error = error.readableMessage()) }
            }
        }
    }

    fun clearError() = _state.update { it.copy(error = null) }
}