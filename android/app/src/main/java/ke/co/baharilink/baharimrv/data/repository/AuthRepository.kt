package ke.co.baharilink.baharimrv.data.repository

import android.net.Uri
import java.net.URI
import java.time.Instant
import javax.inject.Inject
import javax.inject.Singleton
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.data.local.PendingAuthorization
import ke.co.baharilink.baharimrv.data.local.SecureSessionStore
import ke.co.baharilink.baharimrv.data.local.SessionMetadata
import ke.co.baharilink.baharimrv.data.local.SessionMetadataStore
import ke.co.baharilink.baharimrv.data.local.TokenSet
import ke.co.baharilink.baharimrv.data.remote.BahariMrvApi
import ke.co.baharilink.baharimrv.data.remote.FederationExchangeRequest
import ke.co.baharilink.baharimrv.data.remote.LoginRequest
import ke.co.baharilink.baharimrv.data.remote.LoginResponse
import ke.co.baharilink.baharimrv.data.remote.RegisterRequest
import ke.co.baharilink.baharimrv.domain.Pkce
import kotlinx.coroutines.flow.firstOrNull
import retrofit2.HttpException

/** The server's own registration floor; a shorter password is rejected on the wire. */
private const val MIN_PASSWORD = 12

/**
 * Two ways in, one session shape out. A Jahazi token is never replayed against the
 * API: the backend redeems the authorization code and issues its own session token,
 * so every BahariMRV authorisation keys off a BahariMRV account.
 */
@Singleton
class AuthRepository @Inject constructor(
    private val api: BahariMrvApi,
    private val secureStore: SecureSessionStore,
    private val metadataStore: SessionMetadataStore,
) {
    val session = metadataStore.session
    val isConfigured: Boolean = BuildConfig.AUTH_CONFIGURED

    suspend fun signInWithPassword(email: String, password: String) {
        val trimmed = email.trim()
        require(trimmed.contains('@') && trimmed.length > 3) { "Enter a valid email address" }
        require(password.length >= MIN_PASSWORD) { "Passwords are at least $MIN_PASSWORD characters" }
        storeSession(api.login(LoginRequest(trimmed, password)), nativeIssuer(), trimmed)
    }

    /**
     * A first registration can return 202 with no token while IOTA provisioning
     * runs. That is reported as recorded-but-not-signed-in, not as a session.
     */
    suspend fun register(email: String, password: String, name: String, phone: String, role: String): Boolean {
        val response = api.register(
            RegisterRequest(email.trim(), password, name.trim(), phone.trim().ifBlank { null }, role),
        )
        if (response.sessionToken.isNullOrEmpty()) return false
        storeSession(response, nativeIssuer(), email)
        return true
    }

    suspend fun beginAuthorization(mode: String = MODE_LOGIN): Uri {
        check(isConfigured) { "Jahazi sign-in is not configured for this build" }
        val issuer = federatedIssuer()
        val pending = PendingAuthorization(
            state = Pkce.generateState(),
            verifier = Pkce.generateVerifier(),
            nonce = Pkce.generateNonce(),
            mode = mode,
        )
        secureStore.savePendingAuthorization(pending)
        return Uri.parse("$issuer/authorize").buildUpon()
            .appendQueryParameter("response_type", "code")
            .appendQueryParameter("client_id", BuildConfig.JAHAZI_CLIENT_ID)
            .appendQueryParameter("redirect_uri", BuildConfig.JAHAZI_REDIRECT_URI)
            .appendQueryParameter("scope", "openid profile email")
            .appendQueryParameter("state", pending.state)
            .appendQueryParameter("nonce", pending.nonce)
            .appendQueryParameter("code_challenge", Pkce.s256Challenge(pending.verifier))
            .appendQueryParameter("code_challenge_method", "S256")
            .build()
    }

    /** Completes a Jahazi sign-in: the backend redeems the code and issues the local session. */
    suspend fun completeAuthorization(callback: Uri) {
        val pending = requirePending(callback)
        try {
            check(pending.mode != MODE_LINK) { "This authorization was started to link an identity, not to sign in" }
            storeSession(api.exchangeFederation(federationRequest(callback, pending)), federatedIssuer(), null)
        } finally {
            secureStore.clearPendingAuthorization()
        }
    }

    /** Binds a Jahazi identity to the signed-in account without replacing its session. */
    suspend fun linkJahazi(callback: Uri) {
        val pending = requirePending(callback)
        check(pending.mode == MODE_LINK) { "This authorization was started to sign in, not to link" }
        try {
            api.linkFederation(federationRequest(callback, pending))
            refreshSession()
        } finally {
            secureStore.clearPendingAuthorization()
        }
    }

    /**
     * A token can outlive the role the device last saw, so the session is re-read
     * from the server rather than trusted from local storage.
     */
    suspend fun refreshSession() {
        if (secureStore.accessToken() == null) return
        try {
            val me = api.me()
            val current = session.firstOrNull()?.takeIf { it.isAuthenticated } ?: return
            metadataStore.save(
                current.copy(
                    subject = me.user.id,
                    displayName = me.user.name.ifBlank { current.displayName },
                    role = me.user.role,
                ),
            )
        } catch (error: HttpException) {
            if (error.code() == 401 || error.code() == 403) signOut()
        }
    }

    suspend fun signOut() {
        runCatching { api.logout() }
        secureStore.clearTokens()
        metadataStore.clear()
    }

    private fun requirePending(callback: Uri): PendingAuthorization {
        callback.getQueryParameter("error")?.let { error ->
            secureStore.clearPendingAuthorization()
            throw IllegalStateException("Jahazi sign-in was denied: $error")
        }
        val pending = secureStore.pendingAuthorization()
            ?: throw IllegalStateException("No pending Jahazi authorization was found on this device")
        val state = callback.getQueryParameter("state")
            ?: throw IllegalStateException("The SSO callback did not contain state")
        check(Pkce.stateMatches(pending.state, state)) { "SSO state did not match; sign-in was rejected" }
        return pending
    }

    private fun federationRequest(callback: Uri, pending: PendingAuthorization): FederationExchangeRequest {
        val code = callback.getQueryParameter("code")?.takeIf { it.isNotBlank() }
            ?: throw IllegalStateException("The SSO callback did not contain an authorization code")
        return FederationExchangeRequest(
            issuer = federatedIssuer(),
            code = code,
            codeVerifier = pending.verifier,
            nonce = pending.nonce,
            redirectUri = BuildConfig.JAHAZI_REDIRECT_URI,
        )
    }

    private suspend fun storeSession(response: LoginResponse, issuer: String, fallbackName: String?) {
        val token = response.sessionToken
        require(!token.isNullOrBlank()) {
            "The account is recorded, but no session was issued until IOTA identity provisioning completes"
        }
        secureStore.saveTokens(
            TokenSet(
                accessToken = token,
                refreshToken = null,
                expiresAtEpochMs = parseEpochMs(response.expiresAt),
            ),
        )
        metadataStore.save(
            SessionMetadata(
                issuer = issuer,
                subject = response.user.id,
                displayName = response.user.name.ifBlank { fallbackName.orEmpty() }.ifBlank { "BahariMRV user" },
                role = response.user.role,
                establishedAtEpochMs = System.currentTimeMillis(),
            ),
        )
    }

    private fun federatedIssuer(): String {
        val issuer = BuildConfig.JAHAZI_ISSUER_URL.trimEnd('/')
        requireSecure(issuer, "Jahazi")
        return issuer
    }

    private fun nativeIssuer(): String = BuildConfig.API_BASE_URL.trimEnd('/')

    /** HTTPS always; plain HTTP only for a localhost development issuer in a debug build. */
    private fun requireSecure(value: String, label: String) {
        val uri = runCatching { URI(value) }.getOrNull()
        check(uri != null && !uri.host.isNullOrBlank()) { "$label endpoints must be a valid URL" }
        val localDebug = BuildConfig.DEBUG && uri.scheme == "http" &&
            (uri.host == "localhost" || uri.host == "127.0.0.1")
        check(uri.scheme == "https" || localDebug) { "$label endpoints must use HTTPS" }
    }

    private fun parseEpochMs(value: String?): Long? = value?.let { text ->
        runCatching { Instant.parse(text).toEpochMilli() }.getOrNull()
            ?: runCatching { Instant.ofEpochSecond(text.trim().toLong()).toEpochMilli() }.getOrNull()
    }

    companion object {
        const val MODE_LOGIN = "login"
        const val MODE_LINK = "link"
    }
}
