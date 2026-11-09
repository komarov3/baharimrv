package ke.co.baharilink.baharimrv.ui.planting

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ActivityDetailDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PlantingDetailUiState(
    // null = not yet requested; Loading = in flight; Success/Error = done
    val detail: LoadState<ActivityDetailDto>? = null,
)

@HiltViewModel
class PlantingDetailViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlantingDetailUiState())
    val state = _state.asStateFlow()

    fun load(activityId: String) {
        _state.update { it.copy(detail = LoadState.Loading) }
        viewModelScope.launch {
            try {
                val detail = repository.activityDetail(activityId)
                _state.update { it.copy(detail = LoadState.Success(detail)) }
            } catch (e: Exception) {
                _state.update { it.copy(detail = LoadState.Error(e.readableMessage())) }
            }
        }
    }
}
