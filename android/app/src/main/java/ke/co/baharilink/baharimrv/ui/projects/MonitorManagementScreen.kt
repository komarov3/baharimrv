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
import ke.co.baharilink.baharimrv.data.remote.MonitorDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.SectionHeader
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MonitorManagementUiState(
    val monitors: LoadState<List<MonitorDto>> = LoadState.Loading,
    val message: String? = null,
)

@HiltViewModel
class MonitorManagementViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(MonitorManagementUiState())
    val state = _state.asStateFlow()

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(monitors = LoadState.Loading, message = null) }
            try {
                _state.update { it.copy(monitors = LoadState.Success(repository.monitors())) }
            } catch (e: Exception) {
                _state.update { it.copy(monitors = LoadState.Error(e.message ?: "Failed to load monitors")) }
            }
        }
    }

    fun assign(projectId: String, plotId: String, monitorId: String?) {
        viewModelScope.launch {
            try {
                repository.assignMonitor(projectId, plotId, monitorId)
                _state.update { it.copy(message = "Assignment updated") }
            } catch (e: Exception) {
                _state.update { it.copy(message = e.message ?: "Assignment failed") }
            }
        }
    }
}

@Composable
fun MonitorManagementScreen(
    projectId: String,
    plotId: String,
    onBack: () -> Unit,
    viewModel: MonitorManagementViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.load() }

    LazyColumn(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = onBack) { Text("Back") }
                Text("Manage monitors", style = MaterialTheme.typography.titleLarge)
            }
        }
        state.message?.let { item { Text(it, color = MaterialTheme.colorScheme.primary) } }
        item {
            when (val monitors = state.monitors) {
                is LoadState.Loading -> LoadingState("Loading monitors…")
                is LoadState.Error -> EmptyState("Unavailable", monitors.message)
                is LoadState.Success<*> -> {
                    if ((monitors.data as List<MonitorDto>).isEmpty()) EmptyState("No monitors", "No active field monitors are available.")
                }
            }
        }
        val monitors = ((state.monitors as? LoadState.Success<*>)?.data as? List<MonitorDto>) ?: emptyList()
        items(monitors) { monitor ->
            Card(modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(monitor.displayName, style = MaterialTheme.typography.titleSmall)
                    Text(monitor.email, style = MaterialTheme.typography.bodySmall)
                    Text("Status: ${monitor.status}", style = MaterialTheme.typography.bodySmall)
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = { viewModel.assign(projectId, plotId, monitor.id) }) { Text("Assign") }
                        OutlinedButton(onClick = { viewModel.assign(projectId, plotId, null) }) { Text("Unassign") }
                    }
                }
            }
        }
    }
}
