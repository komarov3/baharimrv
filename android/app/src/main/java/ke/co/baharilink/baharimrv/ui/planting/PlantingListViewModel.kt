package ke.co.baharilink.baharimrv.ui.planting

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PlantingListUiState(
    val activities: LoadState<List<ProjectActivityDto>> = LoadState.Success(emptyList()),
)

@HiltViewModel
class PlantingListViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlantingListUiState())
    val state = _state.asStateFlow()

    fun load(projectId: String) {
        if (projectId.isBlank()) {
            _state.update { it.copy(activities = LoadState.Success(emptyList())) }
            return
        }
        _state.update { it.copy(activities = LoadState.Loading) }
        viewModelScope.launch {
            try {
                val items = repository.activities(projectId)
                _state.update { it.copy(activities = LoadState.Success(items)) }
            } catch (e: Exception) {
                _state.update { it.copy(activities = LoadState.Error(e.readableMessage())) }
            }
        }
    }
}
