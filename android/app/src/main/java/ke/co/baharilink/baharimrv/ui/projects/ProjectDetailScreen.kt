package ke.co.baharilink.baharimrv.ui.projects

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.KeyValue
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.SectionHeader
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import ke.co.baharilink.baharimrv.ui.components.stateTone
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ProjectDetailUiState(
    val dashboard: LoadState<DashboardDto> = LoadState.Loading,
    val activities: LoadState<List<ProjectActivityDto>> = LoadState.Loading,
    val weather: String = "Loading...",
    val salinity: String = "Loading...",
    val satelliteS1: String = "Loading...",
    val satelliteS2: String = "Loading...",
    val message: String? = null,
    val error: String? = null,
)

@HiltViewModel
class ProjectDetailViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(ProjectDetailUiState())
    val state = _state.asStateFlow()

    fun load(projectId: String) {
        viewModelScope.launch {
            _state.update { it.copy(dashboard = LoadState.Loading, activities = LoadState.Loading) }
            try {
                _state.update { it.copy(dashboard = LoadState.Success(repository.dashboard(projectId))) }
            } catch (e: Exception) {
                _state.update { it.copy(dashboard = LoadState.Error(e.message ?: "Failed to load dashboard")) }
            }
            try {
                _state.update { it.copy(activities = LoadState.Success(repository.activities(projectId))) }
            } catch (e: Exception) {
                _state.update { it.copy(activities = LoadState.Error(e.message ?: "Failed to load activities")) }
            }
            try {
                val w = repository.weather(projectId)
                _state.update { it.copy(weather = "${w.temperatureC ?: "--"} C, ${w.humidityPct ?: "--"}% humidity, ${w.windKph ?: "--"} kph wind (${w.source ?: "sandbox"})") }
            } catch (e: Exception) {
                _state.update { it.copy(weather = "Weather unavailable") }
            }
            try {
                val sal = repository.salinity(projectId)
                _state.update { it.copy(salinity = "${sal.salinityPpt ?: "--"} ppt (${sal.source ?: "sandbox"})") }
            } catch (e: Exception) {
                _state.update { it.copy(salinity = "Salinity unavailable") }
            }
            try {
                repository.satelliteLatest(projectId, "S1")
                _state.update { it.copy(satelliteS1 = "S1 latest fetched") }
            } catch (e: Exception) {
                _state.update { it.copy(satelliteS1 = "Satellite unavailable") }
            }
            try {
                repository.satelliteLatest(projectId, "S2")
                _state.update { it.copy(satelliteS2 = "S2 latest fetched") }
            } catch (e: Exception) {
                _state.update { it.copy(satelliteS2 = "Satellite unavailable") }
            }
        }
    }

    fun setArchived(projectId: String, archived: Boolean) {
        viewModelScope.launch {
            _state.update { it.copy(error = null, message = null) }
            try {
                if (archived) repository.archiveProject(projectId) else repository.unarchiveProject(projectId)
                _state.update { it.copy(message = if (archived) "Project archived." else "Project restored.") }
                load(projectId)
            } catch (e: Exception) {
                _state.update { it.copy(error = e.readableMessage()) }
            }
        }
    }

    fun deletePlot(projectId: String, plotId: String) {
        viewModelScope.launch {
            _state.update { it.copy(error = null, message = null) }
            try {
                repository.deletePlot(projectId, plotId)
                _state.update { it.copy(message = "Plot removed.") }
                load(projectId)
            } catch (e: Exception) {
                _state.update { it.copy(error = e.readableMessage()) }
            }
        }
    }
}

@Composable
fun ProjectDetailScreen(
    projectId: String,
    onBack: () -> Unit,
    onAddPlot: () -> Unit,
    onAddActivity: () -> Unit,
    onManageMonitors: () -> Unit,
    onEditProject: () -> Unit,
    onEditPlot: (String) -> Unit,
    viewModel: ProjectDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val archivedNow = ((state.dashboard as? LoadState.Success<*>)?.data as? DashboardDto)?.project?.status == "archived"
    var confirmArchive by remember { mutableStateOf(false) }
    var plotToDelete by remember { mutableStateOf<PlotDto?>(null) }
    LaunchedEffect(projectId) { viewModel.load(projectId) }

    LazyColumn(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                }
                Text("Project detail", style = MaterialTheme.typography.titleLarge)
            }
        }
        state.message?.let { msg ->
            item { Text(msg, color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodySmall) }
        }
        state.error?.let { err ->
            item { Text(err, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
        }
        when (val d = state.dashboard) {
            is LoadState.Loading -> item { LoadingState("Loading project…") }
            is LoadState.Error -> item { ErrorState(d.message) }
            is LoadState.Success<*> -> {
                val dash = d.data as DashboardDto
                val archived = dash.project.status == "archived"
                item {
                    BahariCard {
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text(dash.project.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                            if (archived) StatusChip("Archived", tone = ChipTone.NEUTRAL)
                        }
                        KeyValue("Methodology", dash.project.methodology ?: "—")
                        KeyValue("Sites", dash.project.siteCount.toString())
                        KeyValue("Boundary", if (dash.project.boundary != null) "Set · plots restricted inside" else "Not set")
                        KeyValue("Weather", state.weather)
                        KeyValue("Salinity", state.salinity)
                        KeyValue("Sentinel-1", state.satelliteS1)
                        KeyValue("Sentinel-2", state.satelliteS2)
                    }
                }
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (!archived) {
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(onClick = onAddPlot, modifier = Modifier.weight(1f).height(48.dp)) { Text("Add plot") }
                                Button(onClick = onAddActivity, modifier = Modifier.weight(1f).height(48.dp)) { Text("New activity") }
                            }
                        }
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            OutlinedButton(onClick = onEditProject, modifier = Modifier.weight(1f).height(48.dp), enabled = !archived) { Text("Edit project") }
                            OutlinedButton(onClick = { confirmArchive = true }, modifier = Modifier.weight(1f).height(48.dp)) { Text(if (archived) "Restore" else "Archive") }
                        }
                        OutlinedButton(onClick = onManageMonitors, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text("Manage monitors") }
                    }
                }
                item { SectionHeader("Plots") }
                items(dash.plots) { plot ->
                    BahariCard {
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(plot.label, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                            StatusChip(plot.status, tone = stateTone(plot.status))
                        }
                        Text("${plot.latitude}, ${plot.longitude}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        if (plot.boundary != null) {
                            Text("Surveyed outline", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.primary)
                        }
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            OutlinedButton(onClick = { onEditPlot(plot.id) }, modifier = Modifier.weight(1f).height(44.dp), enabled = !archived) { Text("Edit") }
                            OutlinedButton(onClick = { plotToDelete = plot }, modifier = Modifier.weight(1f).height(44.dp), enabled = !archived) { Text("Delete") }
                        }
                    }
                }
                if (dash.plots.isEmpty()) item { EmptyState("No plots", "Add a plot to start monitoring.") }
            }
        }
        item { SectionHeader("Activities") }
        when (val a = state.activities) {
            is LoadState.Loading -> item { LoadingState("Loading activities…") }
            is LoadState.Error -> item { ErrorState(a.message) }
            is LoadState.Success<*> -> {
                val activities = (a.data as? List<ProjectActivityDto>) ?: emptyList()
                if (activities.isEmpty()) item { EmptyState("No activities", "Create an activity to begin reporting.") }
                items(activities) { act ->
                    BahariCard {
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(act.activityType, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                            StatusChip(act.status, tone = stateTone(act.status))
                        }
                        Text("Period: ${act.periodStart ?: "—"} to ${act.periodEnd ?: "—"}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
    }

    if (confirmArchive) {
        AlertDialog(
            onDismissRequest = { confirmArchive = false },
            title = { Text(if (archivedNow) "Restore project?" else "Archive project?") },
            text = {
                Text(
                    if (archivedNow) "The project becomes editable again and new plots and activities are allowed."
                    else "Archiving hides nothing but blocks all new plots, activities and edits. It is reversible."
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    confirmArchive = false
                    viewModel.setArchived(projectId, !archivedNow)
                }) { Text(if (archivedNow) "Restore" else "Archive") }
            },
            dismissButton = { TextButton(onClick = { confirmArchive = false }) { Text("Cancel") } },
        )
    }

    plotToDelete?.let { plot ->
        AlertDialog(
            onDismissRequest = { plotToDelete = null },
            title = { Text("Delete plot ${plot.label}?") },
            text = { Text("The plot is removed from the project. Plots that already have activities cannot be deleted.") },
            confirmButton = {
                TextButton(onClick = {
                    plotToDelete = null
                    viewModel.deletePlot(projectId, plot.id)
                }) { Text("Delete") }
            },
            dismissButton = { TextButton(onClick = { plotToDelete = null }) { Text("Cancel") } },
        )
    }
}
