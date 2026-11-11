package ke.co.baharilink.baharimrv.ui.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.ProjectSelectionStore
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.ProjectsResponse
import ke.co.baharilink.baharimrv.data.remote.WalletDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class HomeUiState(
    val projects: LoadState<ProjectsResponse>? = null,
    val projectId: String = "",
    val dashboard: LoadState<DashboardDto>? = null,
    val wallet: LoadState<WalletDto>? = null,
    val satelliteChange: LoadState<NdviChangeDto?> = LoadState.Success(null),
    val choosing: Boolean = false,
)

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val repository: ProjectRepository,
    private val selection: ProjectSelectionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(HomeUiState())
    val state = _state.asStateFlow()

    init {
        viewModelScope.launch {
            selection.selectedProjectId.collect { id ->
                val current = _state.value
                if (current.projectId == id || (current.projectId.isNotBlank() && current.dashboard != null)) return@collect
                _state.update { it.copy(projectId = id) }
                if (id.isNotBlank()) load()
                if (current.projects == null) loadProjects()
            }
        }
        loadProjects()
    }

    fun updateProjectId(value: String) = _state.update { it.copy(projectId = value) }

    fun chooseProject() = _state.update { it.copy(choosing = true) }

    fun cancelChoosing() = _state.update { it.copy(choosing = false) }

    fun selectProject(id: String) {
        _state.update { it.copy(projectId = id, choosing = false, dashboard = null, wallet = null, satelliteChange = LoadState.Success(null)) }
        load()
    }

    fun loadProjects() {
        viewModelScope.launch {
            _state.update { it.copy(projects = LoadState.Loading) }
            try {
                _state.update { it.copy(projects = LoadState.Success(repository.projects())) }
            } catch (error: Exception) {
                _state.update { it.copy(projects = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    fun load() {
        val id = _state.value.projectId.trim()
        if (id.isBlank()) {
            _state.update { it.copy(dashboard = LoadState.Error("Select a project to load its dashboard.")) }
            return
        }
        _state.update { it.copy(dashboard = LoadState.Loading) }
        loadWallet(id)
        viewModelScope.launch {
            selection.select(id)
            try {
                coroutineScope {
                    val dashboardDeferred = async { runCatching { repository.dashboard(id) } }
                    val changeDeferred = async { runCatching { repository.satelliteChange(id) } }
                    val dashboardResult = dashboardDeferred.await()
                    val changeResult = changeDeferred.await()
                    dashboardResult.onSuccess { dashboard ->
                        _state.update {
                            it.copy(
                                dashboard = LoadState.Success(dashboard),
                                satelliteChange = LoadState.Success(changeResult.getOrNull()),
                            )
                        }
                    }.onFailure { error ->
                        _state.update { it.copy(dashboard = LoadState.Error(error.readableMessage())) }
                    }
                }
            } catch (error: Exception) {
                _state.update { it.copy(dashboard = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    fun loadWallet() = loadWallet(_state.value.projectId.trim())

    private fun loadWallet(projectId: String) {
        if (projectId.isBlank()) return
        viewModelScope.launch {
            _state.update { it.copy(wallet = LoadState.Loading) }
            try {
                _state.update { it.copy(wallet = LoadState.Success(repository.wallet(projectId))) }
            } catch (error: Exception) {
                _state.update { it.copy(wallet = LoadState.Error(error.readableMessage())) }
            }
        }
    }
}
