package ke.co.baharilink.baharimrv.ui.credits

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ListingDto
import ke.co.baharilink.baharimrv.data.remote.PurchaseResultDto
import ke.co.baharilink.baharimrv.data.remote.RetirementResultDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class CreditsUiState(
    val listings: LoadState<List<ListingDto>>? = null,
    val selectedListing: ListingDto? = null,
    val quantityTonnes: String = "",
    val password: String = "",
    val retirementBatchId: String = "",
    val retirementQuantity: String = "",
    val retirementReason: String = "",
    val retirementPassword: String = "",
    val working: Boolean = false,
    val purchase: PurchaseResultDto? = null,
    val retirement: RetirementResultDto? = null,
    val message: String? = null,
    val error: String? = null,
)

@HiltViewModel
class CreditsViewModel @Inject constructor(private val repository: ProjectRepository) : ViewModel() {
    private val _state = MutableStateFlow(CreditsUiState())
    val state = _state.asStateFlow()

    init {
        load()
    }

    fun update(transform: (CreditsUiState) -> CreditsUiState) = _state.update(transform)

    fun select(listing: ListingDto) = _state.update {
        it.copy(selectedListing = listing, purchase = null, message = null, error = null)
    }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(listings = LoadState.Loading) }
            try {
                _state.update { it.copy(listings = LoadState.Success(repository.listings().listings)) }
            } catch (error: Exception) {
                _state.update { it.copy(listings = LoadState.Error(error.readableMessage())) }
            }
        }
    }

    /** A purchase is a real money movement: the server's step-up ticket authorises it, not this device. */
    fun purchase() {
        val current = _state.value
        val listing = current.selectedListing
        if (listing == null) {
            _state.update { it.copy(error = "Select a listing first.") }
            return
        }
        viewModelScope.launch {
            _state.update { it.copy(working = true, error = null, message = null) }
            try {
                val result = repository.purchase(current.password, listing.id, current.quantityTonnes)
                _state.update { it.copy(working = false, purchase = result, password = "") }
                load()
            } catch (error: Exception) {
                _state.update { it.copy(working = false, error = error.readableMessage()) }
            }
        }
    }

    fun retire() {
        val current = _state.value
        viewModelScope.launch {
            _state.update { it.copy(working = true, error = null, message = null) }
            try {
                val result = repository.retire(
                    password = current.retirementPassword,
                    creditBatchId = current.retirementBatchId,
                    quantityTonnes = current.retirementQuantity,
                    reason = current.retirementReason,
                )
                _state.update { it.copy(working = false, retirement = result, retirementPassword = "") }
            } catch (error: Exception) {
                _state.update { it.copy(working = false, error = error.readableMessage()) }
            }
        }
    }
}
