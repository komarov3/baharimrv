package ke.co.baharilink.baharimrv.data.local

import android.content.Context
import android.content.SharedPreferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

private val Context.sessionDataStore by preferencesDataStore(name = "session_metadata")

data class SessionMetadata(
    val issuer: String = "",
    val subject: String = "",
    val displayName: String = "",
    val role: String = "",
    val establishedAtEpochMs: Long = 0L,
) {
    val isAuthenticated: Boolean get() = issuer.isNotBlank() && subject.isNotBlank()
    val isVerifier: Boolean get() = role.equals("independent_verifier", ignoreCase = true)
}

data class TokenSet(
    val accessToken: String,
    val refreshToken: String?,
    val expiresAtEpochMs: Long?,
)

data class PendingAuthorization(
    val state: String,
    val verifier: String,
    val nonce: String,
    val mode: String,
)

@Singleton
class SecureSessionStore @Inject constructor(@ApplicationContext context: Context) {
    private val masterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val secrets: SharedPreferences = EncryptedSharedPreferences.create(
        context,
        "sovereign_session_secrets",
        masterKey,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    fun accessToken(): String? = secrets.getString("access_token", null)
    fun refreshToken(): String? = secrets.getString("refresh_token", null)

    fun saveTokens(tokens: TokenSet) {
        secrets.edit()
            .putString("access_token", tokens.accessToken)
            .putString("refresh_token", tokens.refreshToken)
            .apply {
                if (tokens.expiresAtEpochMs == null) remove("expires_at")
                else putLong("expires_at", tokens.expiresAtEpochMs)
            }
            .apply()
    }

    fun savePendingAuthorization(pending: PendingAuthorization) {
        secrets.edit()
            .putString("pending_state", pending.state)
            .putString("pending_verifier", pending.verifier)
            .putString("pending_nonce", pending.nonce)
            .putString("pending_mode", pending.mode)
            .apply()
    }

    fun pendingAuthorization(): PendingAuthorization? {
        val state = secrets.getString("pending_state", null)
        val verifier = secrets.getString("pending_verifier", null)
        val nonce = secrets.getString("pending_nonce", null)
        val mode = secrets.getString("pending_mode", null)
        return if (state == null || verifier == null || nonce == null || mode == null) null
        else PendingAuthorization(state, verifier, nonce, mode)
    }

    fun clearPendingAuthorization() {
        secrets.edit()
            .remove("pending_state")
            .remove("pending_verifier")
            .remove("pending_nonce")
            .remove("pending_mode")
            .apply()
    }

    fun clearTokens() {
        secrets.edit()
            .remove("access_token")
            .remove("refresh_token")
            .remove("expires_at")
            .apply()
        clearPendingAuthorization()
    }
}

@Singleton
class SessionMetadataStore @Inject constructor(@ApplicationContext private val context: Context) {
    private object Keys {
        val issuer = stringPreferencesKey("issuer")
        val subject = stringPreferencesKey("subject")
        val displayName = stringPreferencesKey("display_name")
        val role = stringPreferencesKey("role")
        val establishedAt = longPreferencesKey("established_at")
    }

    val session: Flow<SessionMetadata> = context.sessionDataStore.data.map { values ->
        SessionMetadata(
            issuer = values[Keys.issuer].orEmpty(),
            subject = values[Keys.subject].orEmpty(),
            displayName = values[Keys.displayName].orEmpty(),
            role = values[Keys.role].orEmpty(),
            establishedAtEpochMs = values[Keys.establishedAt] ?: 0L,
        )
    }

    suspend fun save(session: SessionMetadata) {
        context.sessionDataStore.edit { values ->
            values[Keys.issuer] = session.issuer
            values[Keys.subject] = session.subject
            values[Keys.displayName] = session.displayName
            values[Keys.role] = session.role
            values[Keys.establishedAt] = session.establishedAtEpochMs
        }
    }

    suspend fun clear() {
        context.sessionDataStore.edit { it.clear() }
    }
}
