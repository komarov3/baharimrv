package ke.co.baharilink.baharimrv.ui.profile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.IdentityProfileDto
import ke.co.baharilink.baharimrv.data.remote.LinkedPlatformDto
import ke.co.baharilink.baharimrv.data.repository.AuthRepository
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ProfileUiState(
    val identity: LoadState<IdentityProfileDto>? = null,
    val links: LoadState<List<LinkedPlatformDto>>? = null,
    val working: Boolean = false,
    val message: String? = null,
    val error: String? = null,
)

@HiltViewModel
class ProfileViewModel @Inject constructor(
    private val repository: ProjectRepository,
    authRepository: AuthRepository,
) : ViewModel() {
    val session = authRepository.session

    private val _state = MutableStateFlow(ProfileUiState())
    val state = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(identity = LoadState.Loading, links = LoadState.Loading) }
            try {
                _state.update { it.copy(identity = LoadState.Success(repository.identity())) }
            } catch (error: Exception) {
                _state.update { it.copy(identity = LoadState.Error(error.readableMessage())) }
            }
            try {
                _state.update { it.copy(links = LoadState.Success(repository.links().links)) }
            } catch (error: Exception) {
                _state.update { it.copy(links = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    /** Provisioning is retried by the server; this device only reports the state it gets back. */
    fun retryProvisioning() {
        viewModelScope.launch {
            _state.update { it.copy(working = true, error = null, message = null) }
            try {
                val profile = repository.retryIdentity()
                _state.update {
                    it.copy(
                        working = false,
                        identity = LoadState.Success(profile),
                        message = "Provisioning state is now ${profile.status}.",
                    )
                }
            } catch (error: Exception) {
                _state.update { it.copy(working = false, error = error.readableMessage()) }
            }
        }
    }
}
