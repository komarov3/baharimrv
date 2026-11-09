package ke.co.baharilink.baharimrv.ui.moduleentry

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.SessionMetadata
import ke.co.baharilink.baharimrv.data.repository.AuthRepository
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn

data class ModuleEntryUiState(
    val loading: Boolean = true,
    val error: String? = null,
    val ready: Boolean = false,
    val session: SessionMetadata? = null,
)

@HiltViewModel
class ModuleEntryViewModel @Inject constructor(
    private val auth: AuthRepository,
) : ViewModel() {
    val state: StateFlow<ModuleEntryUiState> = auth.session
        .map { sessionMeta ->
            ModuleEntryUiState(
                loading = !sessionMeta.isAuthenticated,
                error = null,
                ready = sessionMeta.isAuthenticated,
                session = if (sessionMeta.isAuthenticated) sessionMeta else null,
            )
        }
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(), ModuleEntryUiState())

    fun retry() {
        // Session is restored automatically by AuthRepository; nothing to do client-side.
    }
}
