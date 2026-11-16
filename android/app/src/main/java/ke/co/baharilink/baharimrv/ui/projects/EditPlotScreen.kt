package ke.co.baharilink.baharimrv.ui.projects

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import java.util.Locale
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.remote.UpdatePlotRequest
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.domain.ringVertices
import ke.co.baharilink.baharimrv.domain.boundaryPolygon
import ke.co.baharilink.baharimrv.domain.closeRing
import ke.co.baharilink.baharimrv.domain.isValidBoundary
import ke.co.baharilink.baharimrv.domain.ringAreaHectares
import ke.co.baharilink.baharimrv.domain.ringCentroid
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.BoundaryCaptureMap
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class EditPlotUiState(
    val plot: LoadState<PlotDto> = LoadState.Loading,
    val saving: Boolean = false,
    val error: String? = null,
    val saved: Boolean = false,
)

@HiltViewModel
class EditPlotViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(EditPlotUiState())
    val state = _state.asStateFlow()

    fun load(projectId: String, plotId: String) {
        viewModelScope.launch {
            _state.update { it.copy(plot = LoadState.Loading) }
            try {
                val dashboard = repository.dashboard(projectId)
                val plot = (dashboard as DashboardDto).plots.firstOrNull { it.id == plotId }
                _state.update {
                    if (plot == null) {
                        it.copy(plot = LoadState.Error("This plot is no longer in the project."))
                    } else {
                        it.copy(plot = LoadState.Success(plot))
                    }
                }
            } catch (error: Exception) {
                _state.update { it.copy(plot = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    fun save(projectId: String, plotId: String, request: UpdatePlotRequest) {
        viewModelScope.launch {
            _state.update { it.copy(saving = true, error = null, saved = false) }
            try {
                repository.updatePlot(projectId, plotId, request)
                _state.update { it.copy(saving = false, saved = true) }
            } catch (error: Exception) {
                _state.update { it.copy(saving = false, error = error.readableMessage()) }
            }
        }
    }
}

@Composable
fun EditPlotScreen(
    projectId: String,
    plotId: String,
    onBack: () -> Unit,
    onSaved: () -> Unit,
    viewModel: EditPlotViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var code by remember { mutableStateOf<String?>(null) }
    var area by remember { mutableStateOf<String?>(null) }
    var latitude by remember { mutableStateOf<String?>(null) }
    var longitude by remember { mutableStateOf<String?>(null) }
    val vertices = remember { mutableStateListOf<Pair<Double, Double>>() }
    var message by remember { mutableStateOf("") }

    LaunchedEffect(plotId) { viewModel.load(projectId, plotId) }
    LaunchedEffect(state.saved) {
        if (state.saved) onSaved()
    }

    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("Edit plot", style = MaterialTheme.typography.titleLarge)

        when (val p = state.plot) {
            is LoadState.Loading -> LoadingState("Loading plot…")
            is LoadState.Error -> ErrorState(p.message) { viewModel.load(projectId, plotId) }
            is LoadState.Success<*> -> {
                val plot = p.data as PlotDto
                if (code == null) {
                    code = plot.label
                    area = plot.areaHectares ?: ""
                    latitude = plot.latitude.toString()
                    longitude = plot.longitude.toString()
                    vertices.addAll(ringVertices(plot.boundary))
                }
                val ring = remember(vertices.toList()) { closeRing(vertices.toList()) }
                val surveyed = vertices.size >= 3 && isValidBoundary(ring)

                BahariCard {
                    OutlinedTextField(value = code ?: "", onValueChange = { code = it }, label = { Text("Plot code") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                }

                BahariCard {
                    Text("Boundary", style = MaterialTheme.typography.titleSmall)
                    Text(
                        if (plot.boundary != null) "Existing outline loaded. Tap to reposition the map and undo corners, then tap new ones to redraw."
                        else "Optional. Tap the map to capture the surveyed outline; the area is then derived from it.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    BoundaryCaptureMap(
                        vertices = vertices,
                        tapEnabled = { true },
                        onVertex = { lon, lat -> vertices.add(lon to lat) },
                        modifier = Modifier.fillMaxWidth().height(260.dp),
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        BahariButton(onClick = { if (vertices.isNotEmpty()) vertices.removeAt(vertices.size - 1) }, variant = BahariButtonVariant.OUTLINED, small = true, enabled = vertices.isNotEmpty()) { Text("Undo") }
                        BahariButton(onClick = { vertices.clear() }, variant = BahariButtonVariant.GHOST, small = true, enabled = vertices.isNotEmpty()) { Text("Clear") }
                    }
                    if (surveyed) {
                        Text(
                            "Surveyed outline · derived ${String.format(Locale.US, "%.2f", ringAreaHectares(ring))} ha",
                            style = MaterialTheme.typography.titleSmall,
                            color = MaterialTheme.colorScheme.primary,
                        )
                    } else if (plot.boundary != null && vertices.isEmpty()) {
                        Text(
                            "Clearing the corners does not remove the stored outline; redraw it to replace.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }

                if (!surveyed || plot.boundary == null) {
                    BahariCard {
                        Text("Position and area", style = MaterialTheme.typography.titleSmall)
                        OutlinedTextField(value = area ?: "", onValueChange = { area = it }, label = { Text("Area (ha)") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                        OutlinedTextField(value = latitude ?: "", onValueChange = { latitude = it }, label = { Text("Latitude") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                        OutlinedTextField(value = longitude ?: "", onValueChange = { longitude = it }, label = { Text("Longitude") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                    }
                }

                if (message.isNotBlank()) {
                    Text(message, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }
                state.error?.let {
                    Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }

                BahariButton(
                    onClick = {
                        message = ""
                        val currentCode = code?.trim().orEmpty()
                        if (currentCode.isEmpty()) {
                            message = "A plot code is required."
                            return@BahariButton
                        }
                        if (surveyed) {
                            val (lat, lon) = ringCentroid(ring)
                            viewModel.save(
                                projectId, plotId,
                                UpdatePlotRequest(
                                    code = currentCode,
                                    latitude = lat,
                                    longitude = lon,
                                    areaHectares = String.format(Locale.US, "%.6f", ringAreaHectares(ring)),
                                    boundary = boundaryPolygon(ring),
                                ),
                            )
                        } else {
                            val lat = latitude?.toDoubleOrNull()
                            val lon = longitude?.toDoubleOrNull()
                            val ha = area?.toDoubleOrNull()
                            if (lat == null || lon == null) {
                                message = "Enter coordinates or capture an outline."
                                return@BahariButton
                            }
                            if (plot.boundary == null && ha == null) {
                                message = "Enter the plot area in hectares."
                                return@BahariButton
                            }
                            viewModel.save(
                                projectId, plotId,
                                UpdatePlotRequest(
                                    code = currentCode,
                                    latitude = lat,
                                    longitude = lon,
                                    areaHectares = area?.trim()?.ifBlank { null } ?: plot.areaHectares,
                                ),
                            )
                        }
                    },
                    variant = BahariButtonVariant.GREEN,
                    enabled = !state.saving,
                    modifier = Modifier.fillMaxWidth(),
                ) { Text(if (state.saving) "Saving…" else "Save plot") }
                BahariButton(onClick = onBack, variant = BahariButtonVariant.OUTLINED, modifier = Modifier.fillMaxWidth()) { Text("Back") }
            }
        }
    }
}
