package ke.co.baharilink.baharimrv.ui.planting

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.CreateActivityRequestDto
import ke.co.baharilink.baharimrv.data.remote.MethodologyDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.remote.ProjectRowDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PlantingActivityUiState(
    val projectId: String = "",
    val plotId: String = "",
    val availableProjects: List<ProjectRowDto> = emptyList(),
    val availablePlots: List<PlotDto> = emptyList(),
    val plotsLoading: Boolean = false,
    val availableMethodologies: List<MethodologyDto> = emptyList(),
    val methodologiesLoading: Boolean = false,
    val species: String = "",
    val methodologyVersion: String = "",
    val reportingPeriodStart: String = "",
    val reportingPeriodEnd: String = "",
    val seedlingsPlanted: String = "",
    val creating: Boolean = false,
    // anchoring step (after create)
    val anchoring: Boolean = false,
    val anchorStatus: String? = null,   // e.g. "anchored", "proof_pending"
    val explorerUrl: String? = null,
    val error: String? = null,
    val message: String? = null,
)

@HiltViewModel
class PlantingActivityViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlantingActivityUiState())
    val state = _state.asStateFlow()

    init {
        loadMethodologies()
        loadProjects()
    }

    private fun loadMethodologies() {
        _state.update { it.copy(methodologiesLoading = true) }
        viewModelScope.launch {
            try {
                val methods = repository.methodologies().items
                _state.update { it.copy(availableMethodologies = methods, methodologiesLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(methodologiesLoading = false) }
            }
        }
    }

    fun setProjectId(value: String) {
        _state.update { it.copy(projectId = value, plotId = "", availablePlots = emptyList()) }
        if (value.isNotBlank()) loadPlots(value)
    }

    private fun loadProjects() {
        viewModelScope.launch {
            try {
                val rows = repository.projects().rows
                _state.update { it.copy(availableProjects = rows) }
            } catch (_: Exception) {}
        }
    }

    private fun loadPlots(projectId: String) {
        _state.update { it.copy(plotsLoading = true) }
        viewModelScope.launch {
            try {
                val dashboard = repository.dashboard(projectId)
                _state.update { it.copy(availablePlots = dashboard.plots, plotsLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(plotsLoading = false) }
            }
        }
    }

    fun setPlotId(value: String)               = _state.update { it.copy(plotId = value) }
    fun setSpecies(value: String)              = _state.update { it.copy(species = value) }
    fun setMethodologyVersion(value: String)   = _state.update { it.copy(methodologyVersion = value) }
    fun setReportingPeriodStart(value: String) = _state.update { it.copy(reportingPeriodStart = value) }
    fun setReportingPeriodEnd(value: String)   = _state.update { it.copy(reportingPeriodEnd = value) }
    fun setSeedlingsPlanted(value: String)     = _state.update { it.copy(seedlingsPlanted = value) }

    fun create() {
        val current = _state.value
        if (current.projectId.isBlank())          { _state.update { it.copy(error = "Select a project first") }; return }
        if (current.plotId.isBlank())             { _state.update { it.copy(error = "Select a plot") }; return }
        if (current.species.isBlank())            { _state.update { it.copy(error = "Species is required") }; return }
        if (current.methodologyVersion.isBlank()) { _state.update { it.copy(error = "Select a methodology") }; return }
        if (current.seedlingsPlanted.isBlank())   { _state.update { it.copy(error = "Seedlings planted is required") }; return }

        _state.update { it.copy(creating = true, anchoring = false, anchorStatus = null, explorerUrl = null, error = null, message = null) }
        viewModelScope.launch {
            try {
                // Step 1: create the activity
                val activity = repository.createActivity(
                    projectId = current.projectId,
                    request = CreateActivityRequestDto(
                        plotId = current.plotId,
                        activityType = "planting",
                        methodologyVersion = current.methodologyVersion,
                        reportingPeriodStart = current.reportingPeriodStart.ifBlank { null },
                        reportingPeriodEnd   = current.reportingPeriodEnd.ifBlank { null },
                    ),
                )
                _state.update { it.copy(creating = false, anchoring = true, message = "Activity created — anchoring on IOTA testnet…") }

                // Step 2: submit a completion claim to trigger on-chain anchoring
                val completed = repository.completeActivity(activity.id)
                val event = completed.event
                val anchored = event?.status == "anchored"
                _state.update {
                    it.copy(
                        anchoring = false,
                        anchorStatus = event?.status ?: "proof_pending",
                        explorerUrl = event?.explorerUrl,
                        message = if (anchored)
                            "✔ Planting activity anchored on IOTA testnet — tap 'View on IOTA ↗' to verify"
                        else
                            "Activity created. On-chain anchor is pending — it will confirm shortly.",
                    )
                }
            } catch (e: Exception) {
                _state.update { it.copy(creating = false, anchoring = false, error = e.readableMessage()) }
            }
        }
    }
}
