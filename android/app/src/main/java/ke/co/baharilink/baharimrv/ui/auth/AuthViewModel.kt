package ke.co.baharilink.baharimrv.ui.auth

import android.net.Uri
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.SessionMetadata
import ke.co.baharilink.baharimrv.data.repository.AuthRepository
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class AuthUiState(
    val email: String = "",
    val password: String = "",
    val displayName: String = "",
    val phone: String = "",
    val role: String = "field_monitor",
    val registering: Boolean = false,
    val loading: Boolean = false,
    val message: String? = null,
    val error: String? = null,
)

@HiltViewModel
class AuthViewModel @Inject constructor(private val repository: AuthRepository) : ViewModel() {
    val session: StateFlow<SessionMetadata?> = repository.session
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), null)
    val configured: Boolean = repository.isConfigured

    private val _state = MutableStateFlow(AuthUiState())
    val state = _state.asStateFlow()

    private val _openAuthorization = MutableSharedFlow<Uri>()
    val openAuthorization: SharedFlow<Uri> = _openAuthorization.asSharedFlow()

    fun update(transform: (AuthUiState) -> AuthUiState) = _state.update(transform)

    fun signIn() {
        val current = _state.value
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null, message = null) }
            try {
                repository.signInWithPassword(current.email, current.password)
                _state.update { it.copy(loading = false) }
            } catch (error: Exception) {
                _state.update { it.copy(loading = false, error = error.readableMessage()) }
            }
        }
    }

    /**
     * Registration can return 202 while IOTA provisioning runs. The account exists
     * but no session is claimed here; sign-in works once the server activates it.
     */
    fun register() {
        val current = _state.value
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null, message = null) }
            try {
                val signedIn = repository.register(
                    email = current.email,
                    password = current.password,
                    name = current.displayName,
                    phone = current.phone,
                    role = current.role,
                )
                _state.update {
                    it.copy(
                        loading = false,
                        message = if (signedIn) null else "Account recorded. Sign in once the server finishes provisioning your identity.",
                    )
                }
            } catch (error: Exception) {
                _state.update { it.copy(loading = false, error = error.readableMessage()) }
            }
        }
    }

    fun startJahaziSignIn() = startAuthorization(AuthRepository.MODE_LOGIN)

    fun startJahaziLink() = startAuthorization(AuthRepository.MODE_LINK)

    /** A callback means different things depending on whether a session already exists. */
    fun handleCallback(callback: Uri) {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null, message = null) }
            try {
                if (session.value?.isAuthenticated == true) repository.linkJahazi(callback)
                else repository.completeAuthorization(callback)
                _state.update { it.copy(loading = false) }
            } catch (error: Exception) {
                _state.update { it.copy(loading = false, error = error.readableMessage()) }
            }
        }
    }

    fun signOut() {
        viewModelScope.launch { repository.signOut() }
    }

    private fun startAuthorization(mode: String) {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null, message = null) }
            try {
                _openAuthorization.emit(repository.beginAuthorization(mode))
                _state.update { it.copy(loading = false) }
            } catch (error: Exception) {
                _state.update { it.copy(loading = false, error = error.readableMessage()) }
            }
        }
    }
}
