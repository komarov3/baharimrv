package ke.co.baharilink.baharimrv.ui.verification

import android.content.Context
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.SessionMetadataStore
import ke.co.baharilink.baharimrv.data.remote.ProofDto
import ke.co.baharilink.baharimrv.data.remote.ProvenanceDto
import ke.co.baharilink.baharimrv.data.remote.QueueItemDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.domain.saveDownload
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.firstOrNull
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class VerificationUiState(
    val activityId: String = "",
    val isVerifier: Boolean = false,
    val provenance: LoadState<ProvenanceDto>? = null,
    val queue: LoadState<List<QueueItemDto>>? = null,
    val proofs: LoadState<List<ProofDto>>? = null,
    val working: Boolean = false,
    val decision: String = "",
    val notes: String = "",
    val methodologyVersion: String = "",
    val password: String = "",
    val disputeReason: String = "",
    val message: String? = null,
    val error: String? = null,
) {
    val requestId: String? get() = (provenance as? LoadState.Success)?.data?.activity?.verificationRequestId
    val achievementId: String? get() = (provenance as? LoadState.Success)?.data?.activity?.achievementId
}

/**
 * One destination, two modes: with no activity selected a verifier sees the real
 * queue, and any activity opens the provenance timeline the server publishes.
 */
@HiltViewModel
class VerificationViewModel @Inject constructor(
    private val repository: ProjectRepository,
    private val sessionStore: SessionMetadataStore,
    @ApplicationContext private val appContext: Context,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {
    private val _state = MutableStateFlow(VerificationUiState(activityId = savedStateHandle["activityId"] ?: ""))
    val state = _state.asStateFlow()

    init {
        viewModelScope.launch {
            val session = sessionStore.session.firstOrNull()
            _state.update { it.copy(isVerifier = session?.isVerifier == true) }
        }
        if (_state.value.activityId.isNotBlank()) load() else loadQueue()
    }

    fun update(transform: (VerificationUiState) -> VerificationUiState) = _state.update(transform)

    fun updateActivityId(value: String) = _state.update { it.copy(activityId = value) }

    fun openActivity(id: String) {
        _state.update { it.copy(activityId = id, queue = null, provenance = null, proofs = null, message = null, error = null) }
        load()
    }

    fun showQueue() {
        _state.update { it.copy(activityId = "", provenance = null, proofs = null, message = null, error = null) }
        loadQueue()
    }

    fun loadQueue() {
        viewModelScope.launch {
            _state.update { it.copy(queue = LoadState.Loading) }
            try {
                _state.update { it.copy(queue = LoadState.Success(repository.verificationQueue().items)) }
            } catch (error: Exception) {
                _state.update { it.copy(queue = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    fun load() {
        val id = _state.value.activityId.trim()
        if (id.isBlank()) {
            _state.update { it.copy(provenance = LoadState.Error("Enter an activity ID to load its real verification history.")) }
            return
        }
        viewModelScope.launch {
            _state.update { it.copy(provenance = LoadState.Loading, proofs = null, message = null, error = null) }
            try {
                val provenance = repository.provenance(id)
                _state.update { it.copy(provenance = LoadState.Success(provenance)) }
                loadProofs(provenance.activity.achievementId ?: id)
            } catch (error: Exception) {
                _state.update { it.copy(provenance = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    private fun loadProofs(achievementId: String) {
        viewModelScope.launch {
            _state.update { it.copy(proofs = LoadState.Loading) }
            try {
                _state.update { it.copy(proofs = LoadState.Success(repository.proofs(achievementId).proofs)) }
            } catch (error: Exception) {
                _state.update { it.copy(proofs = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    /** Proofs are rebuilt from server state; nothing here renders a claim the backend did not return. */
    fun generateProof() {
        val achievementId = _state.value.achievementId
        if (achievementId.isNullOrBlank()) {
            _state.update { it.copy(error = "No achievement has been recorded for this activity yet, so there is nothing to prove.") }
            return
        }
        submit({ "Proof v${it.version} (${it.status}) generated." }, { repository.generateProof(achievementId) }) {
            loadProofs(achievementId)
        }
    }

    fun downloadProof(proof: ProofDto, format: String) {
        submit({ "Saved $format proof v${proof.version} to this device's app storage." }, { saveProof(proof, format) })
    }

    fun markAchieved() {
        val id = _state.value.activityId.trim()
        submit({ "Recorded: ${it.activity.status}. Provenance anchoring is tracked as a separate event." }, { repository.completeActivity(id) }) { load() }
    }

    fun retryAnchor() {
        val id = _state.value.activityId.trim()
        submit({ "Anchor retry returned ${it.status} on ${chainNetworkLabel(it.event?.network)}." }, { repository.retryAnchor(id) }) { load() }
    }

    fun dispute() {
        val id = _state.value.activityId.trim()
        val reason = _state.value.disputeReason
        submit({ "Dispute ${it.id} recorded as ${it.status}." }, { repository.dispute(id, reason) }) {
            _state.update { it.copy(disputeReason = "") }
            load()
        }
    }

    fun submitDecision() {
        val current = _state.value
        val requestId = current.requestId
        if (requestId == null) {
            _state.update { it.copy(error = "This activity has no verification request assigned yet.") }
            return
        }
        submit(
            { "Decision recorded as ${it.status}." },
            { repository.decide(requestId, current.password, current.decision, current.notes, current.methodologyVersion) },
        ) {
            _state.update { it.copy(password = "", notes = "", decision = "") }
            load()
        }
    }

    private suspend fun saveProof(proof: ProofDto, format: String): String {
        val body = repository.downloadProof(proof.id, format)
        return appContext.saveDownload("proofs", "proof-v${proof.version}-${proof.id}.$format", body)
    }

    /** Runs one gated write and reports the server's own answer, never a local assumption. */
    private fun <T> submit(report: (T) -> String, call: suspend () -> T, then: (() -> Unit)? = null) {
        viewModelScope.launch {
            _state.update { it.copy(working = true, error = null, message = null) }
            try {
                _state.update { it.copy(working = false, message = report(call())) }
            } catch (error: Exception) {
                _state.update { it.copy(working = false, error = error.readableMessage()) }
            }
            then?.invoke()
        }
    }
}
