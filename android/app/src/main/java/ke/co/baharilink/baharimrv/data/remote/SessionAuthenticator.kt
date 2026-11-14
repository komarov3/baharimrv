package ke.co.baharilink.baharimrv.data.remote

import java.io.IOException
import ke.co.baharilink.baharimrv.data.local.TokenSet
import ke.co.baharilink.baharimrv.data.local.TokenStore
import kotlinx.coroutines.runBlocking
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route
import retrofit2.HttpException

/**
 * Silent session renewal. A 401 on any API call triggers one refresh attempt,
 * then the original request is retried with the new bearer. The refresh call
 * rides a separate client with no authenticator, so a dead grant can never
 * recurse. Refresh is single-flight: concurrent 401s share one rotation, and a
 * thread that finds a newer token than the one its request carried just retries.
 *
 * Only a rejected grant ends the session. A network failure or a 5xx leaves the
 * stored tokens alone so an offline device stays signed in.
 */
class SessionAuthenticator(
    private val tokens: TokenStore,
    private val refreshApi: BahariMrvApi,
    private val onSessionInvalid: () -> Unit,
) : Authenticator {

    private val lock = Any()

    override fun authenticate(route: Route?, response: Response): Request? {
        val failedRequest = response.request
        if (isAuthEntry(failedRequest.url.encodedPath)) return null
        if (responseCount(response) >= 2) {
            onSessionInvalid()
            return null
        }
        synchronized(lock) {
            val current = tokens.accessToken() ?: return null
            val failedToken = bearer(failedRequest)
            if (current != failedToken) {
                return failedRequest.withBearer(current)
            }
            val grant = tokens.refreshToken()
            if (grant.isNullOrBlank()) {
                onSessionInvalid()
                return null
            }
            val renewed = try {
                runBlocking { refreshApi.refresh(RefreshRequest(grant)) }
            } catch (rejected: HttpException) {
                if (rejected.code() == 401 || rejected.code() == 403) onSessionInvalid()
                return null
            } catch (_: IOException) {
                return null
            }
            val access = renewed.sessionToken
            if (access.isNullOrBlank()) {
                onSessionInvalid()
                return null
            }
            tokens.saveTokens(
                TokenSet(
                    accessToken = access,
                    refreshToken = renewed.refreshToken,
                    expiresAtEpochMs = renewed.expiresAt?.let { parseEpochMs(it) },
                ),
            )
            return failedRequest.withBearer(access)
        }
    }

    private fun isAuthEntry(path: String): Boolean =
        path == "/api/v1/auth/login" ||
            path == "/api/v1/auth/register" ||
            path == "/api/v1/auth/refresh" ||
            path.startsWith("/api/v1/auth/federation/")

    private fun bearer(request: Request): String? =
        request.header("Authorization")?.removePrefix("Bearer ")?.trim()

    private fun Request.withBearer(token: String): Request =
        newBuilder().header("Authorization", "Bearer $token").build()

    private fun responseCount(response: Response): Int {
        var count = 1
        var prior = response.priorResponse
        while (prior != null) {
            count++
            prior = prior.priorResponse
        }
        return count
    }

    private fun parseEpochMs(value: String): Long? =
        runCatching { java.time.Instant.parse(value).toEpochMilli() }.getOrNull()
}