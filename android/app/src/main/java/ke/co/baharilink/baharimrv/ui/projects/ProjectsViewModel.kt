package ke.co.baharilink.baharimrv.ui.projects

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.CreatePlotRequestDto
import ke.co.baharilink.baharimrv.data.remote.MethodologyDto
import ke.co.baharilink.baharimrv.data.remote.ProjectRowDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ProjectsUiState(
    val projects: LoadState<List<ProjectRowDto>> = LoadState.Loading,
    val methodologies: List<MethodologyDto> = emptyList(),
    val creating: Boolean = false,
    val createError: String? = null,
    val createdProjectId: String? = null,
)

@HiltViewModel
class ProjectsViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(ProjectsUiState())
    val state = _state.asStateFlow()

    init {
        load()
        loadMethodologies()
    }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(projects = LoadState.Loading, createError = null) }
            try {
                _state.update { it.copy(projects = LoadState.Success(repository.projects().rows)) }
            } catch (error: Exception) {
                _state.update { it.copy(projects = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    private fun loadMethodologies() {
        viewModelScope.launch {
            try {
                _state.update { it.copy(methodologies = repository.methodologies().items) }
            } catch (e: Exception) {
                _state.update { it.copy(createError = "Could not load methodologies: ${e.readableMessage()}") }
            }
        }
    }

    fun createProject(
        name: String,
        methodologyId: String,
        conservationType: String,
        communityGroup: String,
        species: String,
        communityShareBasisPoints: Int,
        sites: List<String>,
        plots: List<CreatePlotRequestDto>,
    ) {
        viewModelScope.launch {
            _state.update { it.copy(creating = true, createError = null, createdProjectId = null) }
            try {
                val response = repository.createProject(
                    name = name,
                    methodologyId = methodologyId,
                    conservationType = conservationType,
                    communityGroup = communityGroup,
                    species = species,
                    communityShareBasisPoints = communityShareBasisPoints,
                    sites = sites,
                    plots = plots,
                )
                _state.update { it.copy(creating = false, createdProjectId = response.id) }
                load()
            } catch (error: Exception) {
                _state.update { it.copy(creating = false, createError = error.readableMessage()) }
            }
        }
    }

    fun clearCreated() = _state.update { it.copy(createdProjectId = null) }
}
