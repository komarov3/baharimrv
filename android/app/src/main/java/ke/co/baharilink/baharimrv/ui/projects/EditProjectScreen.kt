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
import ke.co.baharilink.baharimrv.data.remote.UpdateProjectRequest
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.domain.ringVertices
import ke.co.baharilink.baharimrv.domain.boundaryPolygon
import ke.co.baharilink.baharimrv.domain.closeRing
import ke.co.baharilink.baharimrv.domain.isValidBoundary
import ke.co.baharilink.baharimrv.domain.ringAreaHectares
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

data class EditProjectUiState(
    val dashboard: LoadState<DashboardDto> = LoadState.Loading,
    val saving: Boolean = false,
    val error: String? = null,
    val saved: Boolean = false,
)

@HiltViewModel
class EditProjectViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(EditProjectUiState())
    val state = _state.asStateFlow()

    fun load(projectId: String) {
        viewModelScope.launch {
            _state.update { it.copy(dashboard = LoadState.Loading) }
            try {
                _state.update { it.copy(dashboard = LoadState.Success(repository.dashboard(projectId))) }
            } catch (error: Exception) {
                _state.update { it.copy(dashboard = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    fun save(projectId: String, request: UpdateProjectRequest) {
        viewModelScope.launch {
            _state.update { it.copy(saving = true, error = null, saved = false) }
            try {
                repository.updateProject(projectId, request)
                _state.update { it.copy(saving = false, saved = true) }
            } catch (error: Exception) {
                _state.update { it.copy(saving = false, error = error.readableMessage()) }
            }
        }
    }
}

@Composable
fun EditProjectScreen(
    projectId: String,
    onBack: () -> Unit,
    onSaved: () -> Unit,
    viewModel: EditProjectViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var name by remember { mutableStateOf<String?>(null) }
    var communityGroup by remember { mutableStateOf<String?>(null) }
    var species by remember { mutableStateOf<String?>(null) }
    var conservationType by remember { mutableStateOf<String?>(null) }
    var shareBps by remember { mutableStateOf<String?>(null) }
    val vertices = remember { mutableStateListOf<Pair<Double, Double>>() }
    var message by remember { mutableStateOf("") }

    LaunchedEffect(projectId) { viewModel.load(projectId) }
    LaunchedEffect(state.saved) {
        if (state.saved) onSaved()
    }

    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("Edit project", style = MaterialTheme.typography.titleLarge)

        when (val d = state.dashboard) {
            is LoadState.Loading -> {
                LoadingState("Loading project…")
            }
            is LoadState.Error -> {
                ErrorState(d.message) { viewModel.load(projectId) }
            }
            is LoadState.Success<*> -> {
                val project = (d.data as DashboardDto).project
                if (name == null) {
                    name = project.name
                    communityGroup = project.communityGroup ?: ""
                    species = project.species ?: ""
                    conservationType = project.conservationType ?: ""
                    shareBps = (project.communityShareBasisPoints ?: 0).toString()
                    vertices.addAll(ringVertices(project.boundary))
                }
                val ring = remember(vertices.toList()) { closeRing(vertices.toList()) }
                val surveyed = vertices.size >= 3 && isValidBoundary(ring)

                if (project.status == "archived") {
                    Text(
                        "This project is archived. Restore it from the project page before editing.",
                        color = MaterialTheme.colorScheme.error,
                        style = MaterialTheme.typography.bodySmall,
                    )
                }

                BahariCard {
                    Text("Basics", style = MaterialTheme.typography.titleSmall)
                    Text(
                        "The methodology stays as created; it defines how results are measured and cannot change.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    OutlinedTextField(value = name ?: "", onValueChange = { name = it }, label = { Text("Project name") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(value = communityGroup ?: "", onValueChange = { communityGroup = it }, label = { Text("Community group") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(value = species ?: "", onValueChange = { species = it }, label = { Text("Species") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(value = conservationType ?: "", onValueChange = { conservationType = it }, label = { Text("Conservation type") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(value = shareBps ?: "", onValueChange = { shareBps = it }, label = { Text("Community share (basis points)") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
                }

                BahariCard {
                    Text("Project outline", style = MaterialTheme.typography.titleSmall)
                    Text(
                        if (project.boundary != null)
                            "Existing outline loaded. Tap the map to redraw it corner by corner; plots must stay inside."
                        else
                            "Optional. Tap the map to draw the project outline; plots' centroids must fall inside it.",
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
                            "Outline set · ${String.format(Locale.US, "%.2f", ringAreaHectares(ring))} ha",
                            style = MaterialTheme.typography.titleSmall,
                            color = MaterialTheme.colorScheme.primary,
                        )
                    } else if (vertices.isNotEmpty()) {
                        Text(
                            "Drop at least three corners for a valid outline.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    if (!surveyed && project.boundary != null && vertices.isEmpty()) {
                        Text(
                            "Clearing leaves the stored outline in place; draw a new one to replace it.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
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
                        val currentName = name?.trim().orEmpty()
                        if (currentName.isEmpty()) {
                            message = "A project name is required."
                            return@BahariButton
                        }
                        val share = shareBps?.trim()?.toIntOrNull()
                        if (shareBps?.isNotBlank() == true && share == null) {
                            message = "Community share must be a whole number of basis points."
                            return@BahariButton
                        }
                        viewModel.save(
                            projectId,
                            UpdateProjectRequest(
                                name = currentName,
                                communityGroup = communityGroup?.trim(),
                                species = species?.trim(),
                                conservationType = conservationType?.trim(),
                                communityShareBasisPoints = share,
                                boundary = if (surveyed) boundaryPolygon(ring) else null,
                            ),
                        )
                    },
                    variant = BahariButtonVariant.GREEN,
                    enabled = !state.saving && project.status != "archived",
                    modifier = Modifier.fillMaxWidth(),
                ) { Text(if (state.saving) "Saving…" else "Save changes") }
                BahariButton(onClick = onBack, variant = BahariButtonVariant.OUTLINED, modifier = Modifier.fillMaxWidth()) { Text("Back") }
            }
        }
    }
}
