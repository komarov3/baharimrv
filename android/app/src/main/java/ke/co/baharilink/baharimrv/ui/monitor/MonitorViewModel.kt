package ke.co.baharilink.baharimrv.ui.monitor

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.MonitoringWithEvidence
import ke.co.baharilink.baharimrv.data.local.ProjectSelectionStore
import ke.co.baharilink.baharimrv.data.remote.MethodologyDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import ke.co.baharilink.baharimrv.data.remote.ProjectRowDto
import ke.co.baharilink.baharimrv.data.repository.CaptureInput
import ke.co.baharilink.baharimrv.data.repository.EvidenceSelection
import ke.co.baharilink.baharimrv.data.repository.MonitoringRepository
import ke.co.baharilink.baharimrv.location.DeviceLocationRepository
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Tells a device GPS fix apart from typed coordinates so the capture UI can never claim a lock it did not get. */
enum class CoordinateSource { NONE, DEVICE_FIX, MANUAL }

data class MonitorUiState(
    val projectId: String = "",
    val plotId: String = "",
    val activityId: String = "",
    val methodologyVersion: String = "",
    val latitude: String = "",
    val longitude: String = "",
    val gpsAccuracy: String = "",
    val coordinateSource: CoordinateSource = CoordinateSource.NONE,
    val plantedCount: String = "",
    val aliveCount: String = "",
    val averageHeight: String = "",
    val species: String = "",
    val attested: Boolean = false,
    val evidence: List<EvidenceSelection> = emptyList(),
    val records: List<MonitoringWithEvidence> = emptyList(),
    val pendingCount: Int = 0,
    val locating: Boolean = false,
    val saving: Boolean = false,
    val recording: Boolean = false,
    val message: String? = null,
    val error: String? = null,
    // dropdowns
    val availableProjects: List<ProjectRowDto> = emptyList(),
    val availablePlots: List<PlotDto> = emptyList(),
    val availableActivities: List<ProjectActivityDto> = emptyList(),
    val availableMethodologies: List<MethodologyDto> = emptyList(),
    val projectsLoading: Boolean = false,
    val plotsLoading: Boolean = false,
    val activitiesLoading: Boolean = false,
    val methodologiesLoading: Boolean = false,
)

@HiltViewModel
class MonitorViewModel @Inject constructor(
    private val repository: MonitoringRepository,
    private val locationRepository: DeviceLocationRepository,
    private val selection: ProjectSelectionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(MonitorUiState())
    val state = _state.asStateFlow()

    init {
        viewModelScope.launch { repository.records.collect { records -> _state.update { it.copy(records = records) } } }
        viewModelScope.launch { repository.pendingCount.collect { count -> _state.update { it.copy(pendingCount = count) } } }
        viewModelScope.launch {
            selection.selectedProjectId.collect { id ->
                if (_state.value.projectId.isBlank() && id.isNotBlank()) {
                    _state.update { it.copy(projectId = id) }
                    loadPlotsAndActivities(id)
                }
            }
        }
        loadProjects()
        loadMethodologies()
    }

    private fun loadProjects() {
        _state.update { it.copy(projectsLoading = true) }
        viewModelScope.launch {
            try {
                val rows = repository.projects()
                _state.update { it.copy(availableProjects = rows, projectsLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(projectsLoading = false) }
            }
        }
    }

    private fun loadMethodologies() {
        _state.update { it.copy(methodologiesLoading = true) }
        viewModelScope.launch {
            try {
                val methods = repository.methodologies()
                _state.update { it.copy(availableMethodologies = methods, methodologiesLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(methodologiesLoading = false) }
            }
        }
    }

    fun selectProject(projectId: String) {
        _state.update { it.copy(projectId = projectId, plotId = "", activityId = "", availablePlots = emptyList(), availableActivities = emptyList()) }
        if (projectId.isNotBlank()) loadPlotsAndActivities(projectId)
    }

    private fun loadPlotsAndActivities(projectId: String) {
        _state.update { it.copy(plotsLoading = true, activitiesLoading = true) }
        viewModelScope.launch {
            try {
                val dashboard = repository.dashboard(projectId)
                _state.update { it.copy(availablePlots = dashboard.plots, plotsLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(plotsLoading = false) }
            }
        }
        viewModelScope.launch {
            try {
                val activities = repository.activities(projectId)
                _state.update { it.copy(availableActivities = activities, activitiesLoading = false) }
            } catch (_: Exception) {
                _state.update { it.copy(activitiesLoading = false) }
            }
        }
    }

    fun update(transform: (MonitorUiState) -> MonitorUiState) = _state.update(transform)

    fun locate() {
        viewModelScope.launch {
            _state.update { it.copy(locating = true, error = null) }
            try {
                val fix = locationRepository.currentLocation()
                _state.update {
                    it.copy(
                        locating = false,
                        latitude = fix.latitude.toString(),
                        longitude = fix.longitude.toString(),
                        gpsAccuracy = fix.accuracyMeters.toString(),
                        coordinateSource = CoordinateSource.DEVICE_FIX,
                    )
                }
            } catch (error: Exception) {
                _state.update {
                    it.copy(
                        locating = false,
                        error = "${error.readableMessage()} Coordinates in Plot & GPS are marked unverified until a device fix is read.",
                    )
                }
            }
        }
    }

    fun setLatitude(value: String)   = setCoordinate { it.copy(latitude = value) }
    fun setLongitude(value: String)  = setCoordinate { it.copy(longitude = value) }
    fun setGpsAccuracy(value: String) = setCoordinate { it.copy(gpsAccuracy = value) }

    private fun setCoordinate(transform: (MonitorUiState) -> MonitorUiState) =
        _state.update { transform(it).copy(coordinateSource = CoordinateSource.MANUAL) }

    fun removeEvidence(index: Int) = _state.update { current ->
        current.copy(evidence = current.evidence.filterIndexed { i, _ -> i != index })
    }

    fun addEvidence(selection: EvidenceSelection) = _state.update { current ->
        val fix = current.copy(evidence = current.evidence + selection)
        fix.copy(evidence = fix.evidence.map {
            if (it === selection) it.copy(
                latitude = current.latitude.toDoubleOrNull(),
                longitude = current.longitude.toDoubleOrNull(),
            ) else it
        })
    }

    fun syncNow() = repository.scheduleSync()

    fun save() {
        val current = _state.value
        viewModelScope.launch {
            _state.update { it.copy(saving = true, error = null, message = null) }
            try {
                val input = CaptureInput(
                    projectId = current.projectId,
                    plotId = current.plotId,
                    activityId = current.activityId,
                    latitude = requireNotNull(current.latitude.toDoubleOrNull()) { "Latitude is required" },
                    longitude = requireNotNull(current.longitude.toDoubleOrNull()) { "Longitude is required" },
                    gpsAccuracyMeters = requireNotNull(current.gpsAccuracy.toDoubleOrNull()) { "GPS accuracy is required" },
                    plantedCount = requireNotNull(current.plantedCount.toIntOrNull()) { "Planted count is required" },
                    aliveCount = requireNotNull(current.aliveCount.toIntOrNull()) { "Alive count is required" },
                    averageHeightMeters = requireNotNull(current.averageHeight.toDoubleOrNull()) { "Average height is required" },
                    species = current.species,
                    methodologyVersion = current.methodologyVersion,
                    attested = current.attested,
                )
                val clientId = repository.saveCapture(input, current.evidence)
                selection.select(current.projectId.trim())
                _state.update {
                    it.copy(
                        saving = false,
                        plotId = "",
                        latitude = "",
                        longitude = "",
                        gpsAccuracy = "",
                        coordinateSource = CoordinateSource.NONE,
                        plantedCount = "",
                        aliveCount = "",
                        averageHeight = "",
                        species = "",
                        attested = false,
                        evidence = emptyList(),
                        message = "Saved on device as $clientId. Sync will run when the network is available.",
                    )
                }
            } catch (error: Exception) {
                _state.update { it.copy(saving = false, error = error.readableMessage()) }
            }
        }
    }
}
