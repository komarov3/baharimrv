package ke.co.baharilink.baharimrv.ui.planting

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.CreateActivityRequestDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PlantingActivityUiState(
    val projectId: String = "",
    val plotId: String = "",
    val species: String = "",
    val methodologyVersion: String = "",
    val reportingPeriodStart: String = "",
    val reportingPeriodEnd: String = "",
    val seedlingsPlanted: String = "",
    val creating: Boolean = false,
    val error: String? = null,
    val message: String? = null,
)

@HiltViewModel
class PlantingActivityViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlantingActivityUiState())
    val state = _state.asStateFlow()

    fun setProjectId(value: String) = _state.update { it.copy(projectId = value) }
    fun setPlotId(value: String) = _state.update { it.copy(plotId = value) }
    fun setSpecies(value: String) = _state.update { it.copy(species = value) }
    fun setMethodologyVersion(value: String) = _state.update { it.copy(methodologyVersion = value) }
    fun setReportingPeriodStart(value: String) = _state.update { it.copy(reportingPeriodStart = value) }
    fun setReportingPeriodEnd(value: String) = _state.update { it.copy(reportingPeriodEnd = value) }
    fun setSeedlingsPlanted(value: String) = _state.update { it.copy(seedlingsPlanted = value) }

    fun create() {
        val current = _state.value
        if (current.projectId.isBlank()) {
            _state.update { it.copy(error = "Select a project first") }
            return
        }
        if (current.plotId.isBlank()) {
            _state.update { it.copy(error = "Plot is required") }
            return
        }
        if (current.species.isBlank()) {
            _state.update { it.copy(error = "Species is required") }
            return
        }
        if (current.methodologyVersion.isBlank()) {
            _state.update { it.copy(error = "Methodology version is required") }
            return
        }
        if (current.seedlingsPlanted.isBlank()) {
            _state.update { it.copy(error = "Seedlings planted is required") }
            return
        }
        _state.update { it.copy(creating = true, error = null, message = null) }
        viewModelScope.launch {
            try {
                repository.createActivity(
                    projectId = current.projectId,
                    request = CreateActivityRequestDto(
                        plotId = current.plotId,
                        activityType = "planting",
                        methodologyVersion = current.methodologyVersion,
                        reportingPeriodStart = current.reportingPeriodStart.ifBlank { null },
                        reportingPeriodEnd = current.reportingPeriodEnd.ifBlank { null },
                    ),
                )
                _state.update {
                    it.copy(
                        creating = false,
                        message = "Planting activity created. Record survival during a field visit.",
                    )
                }
            } catch (e: Exception) {
                _state.update { it.copy(creating = false, error = e.readableMessage()) }
            }
        }
    }
}
