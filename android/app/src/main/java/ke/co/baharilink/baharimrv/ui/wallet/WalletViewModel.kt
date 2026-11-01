package ke.co.baharilink.baharimrv.ui.wallet

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.ProjectSelectionStore
import ke.co.baharilink.baharimrv.data.remote.DocumentDto
import ke.co.baharilink.baharimrv.data.remote.ReconciliationDto
import ke.co.baharilink.baharimrv.data.remote.WalletDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.domain.saveDownload
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class WalletUiState(
    val projectId: String = "",
    val wallet: LoadState<WalletDto>? = null,
    val reconciliation: LoadState<ReconciliationDto>? = null,
    val documents: LoadState<List<DocumentDto>>? = null,
    val working: Boolean = false,
    val message: String? = null,
    val error: String? = null,
)

/** Every figure here is the server's own; this screen never derives a balance locally. */
@HiltViewModel
class WalletViewModel @Inject constructor(
    private val repository: ProjectRepository,
    private val selection: ProjectSelectionStore,
    @ApplicationContext private val appContext: Context,
) : ViewModel() {
    private val _state = MutableStateFlow(WalletUiState())
    val state = _state.asStateFlow()

    init {
        viewModelScope.launch {
            selection.selectedProjectId.collect { id ->
                if (_state.value.projectId.isBlank() && id.isNotBlank()) {
                    _state.update { it.copy(projectId = id) }
                    load(id)
                }
            }
        }
    }

    fun update(transform: (WalletUiState) -> WalletUiState) = _state.update(transform)

    fun updateProjectId(value: String) = _state.update { it.copy(projectId = value) }

    fun load() {
        val id = _state.value.projectId.trim()
        if (id.isBlank()) {
            _state.update { it.copy(wallet = LoadState.Error("Enter a project ID to load its wallet.")) }
            return
        }
        load(id)
    }

    private fun load(projectId: String) {
        viewModelScope.launch {
            _state.update { it.copy(wallet = LoadState.Loading, reconciliation = LoadState.Loading, documents = LoadState.Loading) }
            selection.select(projectId)
            loadInto({ repository.wallet(projectId) }) { wallet -> _state.update { it.copy(wallet = wallet) } }
            loadInto({ repository.reconciliation(projectId) }) { reconciliation ->
                _state.update { it.copy(reconciliation = reconciliation) }
            }
            loadInto({ repository.documents(projectId).documents }) { documents ->
                _state.update { it.copy(documents = documents) }
            }
        }
    }

    fun download(document: DocumentDto) {
        viewModelScope.launch {
            _state.update { it.copy(working = true, error = null, message = null) }
            try {
                val body = repository.downloadDocument(document.id)
                _state.update {
                    it.copy(working = false, message = appContext.saveDownload("documents", document.name, body))
                }
            } catch (error: Exception) {
                _state.update { it.copy(working = false, error = error.readableMessage()) }
            }
        }
    }

    private suspend fun <T> loadInto(call: suspend () -> T, sink: (LoadState<T>) -> Unit) {
        try {
            sink(LoadState.Success(call()))
        } catch (error: Exception) {
            sink(LoadState.Error(error.readableMessage()))
        }
    }
}
