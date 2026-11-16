package ke.co.baharilink.baharimrv.ui.planting

import android.content.Context
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.CancellationException
import java.io.ByteArrayOutputStream
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
    val title: String = "",
    val species: String = "",
    val methodologyVersion: String = "",
    val reportingPeriodStart: String = "",
    val reportingPeriodEnd: String = "",
    val seedlingsPlanted: String = "",
    val creating: Boolean = false,
    val createdActivityId: String? = null,
    val photos: List<PendingPhoto> = emptyList(),
    val uploading: Boolean = false,
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
    @ApplicationContext private val context: Context,
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
    fun setTitle(value: String) = _state.update { it.copy(title = value) }
    fun setSpecies(value: String)              = _state.update { it.copy(species = value) }
    fun setMethodologyVersion(value: String)   = _state.update { it.copy(methodologyVersion = value) }
    fun setReportingPeriodStart(value: String) = _state.update { it.copy(reportingPeriodStart = value) }
    fun setReportingPeriodEnd(value: String)   = _state.update { it.copy(reportingPeriodEnd = value) }
    fun setSeedlingsPlanted(value: String)     = _state.update { it.copy(seedlingsPlanted = value) }

    fun addPhoto(photo: PendingPhoto) {
        val current = _state.value
        if (current.uploading || current.creating) return
        if (current.photos.size >= 10) { _state.update { it.copy(error = "You can attach up to 10 photos") }; return }
        if (photo.sizeBytes > 5 * 1024 * 1024) { _state.update { it.copy(error = "Each photo must be 5 MB or smaller") }; return }
        if (current.photos.none { it.uri == photo.uri }) _state.update { it.copy(photos = it.photos + photo) }
    }

    fun removePhoto(photo: PendingPhoto) {
        if (_state.value.uploading || _state.value.creating || photo.uploadedId != null) return
        _state.update { it.copy(photos = it.photos.filterNot { item -> item.uri == photo.uri }) }
    }

    fun retryPhotos() {
        val id = _state.value.createdActivityId ?: return
        if (_state.value.uploading) return
        viewModelScope.launch { uploadPhotos(id) }
    }

    private suspend fun uploadPhotos(activityId: String) {
        _state.update { it.copy(uploading = true) }
        try {
            for (photo in _state.value.photos.filter { it.uploadedId == null }) {
                _state.update { it.copy(photos = it.photos.map { p -> if (p.uri == photo.uri) p.copy(uploadStatus = "Uploading…", error = null) else p }) }
                try {
                    val result = withContext(Dispatchers.IO) {
                        val bytes = context.contentResolver.openInputStream(photo.uri)?.use { input ->
                            val output = ByteArrayOutputStream()
                            val buffer = ByteArray(8192)
                            while (true) {
                                val read = input.read(buffer)
                                if (read < 0) break
                                require(output.size() + read <= 5 * 1024 * 1024) { "Each photo must be 5 MB or smaller" }
                                output.write(buffer, 0, read)
                            }
                            output.toByteArray()
                        } ?: error("This photo is no longer accessible. Remove it and select it again.")
                        require(bytes.isNotEmpty()) { "This photo is empty. Select it again." }
                        val mediaType = context.contentResolver.getType(photo.uri) ?: "application/octet-stream"
                        repository.uploadActivityPhoto(activityId, bytes, mediaType)
                    }
                    _state.update { it.copy(photos = it.photos.map { p -> if (p.uri == photo.uri) p.copy(uploadStatus = "Stored · SHA-256 checked by server", uploadedId = result.id, error = null) else p }) }
                } catch (e: CancellationException) { throw e
                } catch (e: Exception) {
                    _state.update { it.copy(photos = it.photos.map { p -> if (p.uri == photo.uri) p.copy(uploadStatus = "Upload failed", error = e.readableMessage()) else p }) }
                }
            }
        } finally { _state.update { it.copy(uploading = false) } }
    }

    fun create(measurements: Map<String, String> = emptyMap()) {
        val current = _state.value
        if (current.creating || current.createdActivityId != null) return
        if (current.title.codePointCount(0, current.title.length) > 120) { _state.update { it.copy(error = "Activity names can contain up to 120 characters") }; return }
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
                        title = current.title.trim(),
                        methodologyVersion = current.methodologyVersion,
                        reportingPeriodStart = current.reportingPeriodStart.ifBlank { null },
                        reportingPeriodEnd   = current.reportingPeriodEnd.ifBlank { null },
                        measurements = (measurements +
                            ("seedlingsPlanted" to current.seedlingsPlanted) +
                            ("species" to current.species))
                            .filterValues { it.isNotBlank() }
                            .ifEmpty { null },
                    ),
                )
                _state.update {
                    it.copy(
                        creating = false,
                        createdActivityId = activity.id,
                        anchoring = false,
                        message = "Planting activity saved as a draft. Photo upload results are shown above. Activity photos are not attested or anchored.",
                    )
                }
                uploadPhotos(activity.id)
            } catch (e: CancellationException) { throw e
            } catch (e: Exception) {
                _state.update { it.copy(creating = false, anchoring = false, error = e.readableMessage()) }
            }
        }
    }
}
