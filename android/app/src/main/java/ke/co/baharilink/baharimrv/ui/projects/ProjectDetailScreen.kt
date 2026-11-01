package ke.co.baharilink.baharimrv.ui.projects

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.SectionHeader
import ke.co.baharilink.baharimrv.ui.components.StatusChip
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
}

@Composable
fun ProjectDetailScreen(
    projectId: String,
    onBack: () -> Unit,
    onAddPlot: () -> Unit,
    onAddActivity: () -> Unit,
    onManageMonitors: () -> Unit,
    viewModel: ProjectDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(projectId) { viewModel.load(projectId) }

    LazyColumn(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = onBack) { Text("Back") }
                Text("Project detail", style = MaterialTheme.typography.titleLarge)
            }
        }
        when (val d = state.dashboard) {
            is LoadState.Loading -> item { LoadingState("Loading project…") }
            is LoadState.Error -> item { ErrorState(d.message) }
            is LoadState.Success<*> -> {
                val dash = d.data as DashboardDto
                item {
                    Card(modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(dash.project.name, style = MaterialTheme.typography.titleMedium)
                            Text("Methodology: ${dash.project.methodology}")
                            Text("Sites: ${dash.project.siteCount}")
                            Text("Weather: ${state.weather}")
                            Text("Salinity: ${state.salinity}")
                            Text("Sentinel-1: ${state.satelliteS1}")
                            Text("Sentinel-2: ${state.satelliteS2}")
                        }
                    }
                }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = onAddPlot, modifier = Modifier.weight(1f)) { Text("Add plot") }
                        Button(onClick = onAddActivity, modifier = Modifier.weight(1f)) { Text("New activity") }
                        OutlinedButton(onClick = onManageMonitors, modifier = Modifier.weight(1f)) { Text("Monitors") }
                    }
                }
                item { SectionHeader("Plots") }
                items(dash.plots) { plot ->
                    Card(modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp)) {
                            Text(plot.label, style = MaterialTheme.typography.titleSmall)
                            Text("${plot.latitude}, ${plot.longitude}")
                            Text("Status: ${plot.status}")
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
                    Card(modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp)) {
                            Text(act.activityType, style = MaterialTheme.typography.titleSmall)
                            Text("Status: ${act.status}")
                            Text("Period: ${act.periodStart ?: "—"} to ${act.periodEnd ?: "—"}")
                        }
                    }
                }
            }
        }
    }
}
