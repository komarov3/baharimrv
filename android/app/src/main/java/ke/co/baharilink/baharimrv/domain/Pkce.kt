package ke.co.baharilink.baharimrv.domain

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

object Pkce {
    private val random = SecureRandom()

    fun generateVerifier(): String = randomUrlToken(32)
    fun generateState(): String = randomUrlToken(32)
    fun generateNonce(): String = randomUrlToken(32)

    fun s256Challenge(verifier: String): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(verifier.toByteArray(Charsets.US_ASCII))
        return Base64.getUrlEncoder().withoutPadding().encodeToString(digest)
    }

    fun stateMatches(expected: String, actual: String): Boolean = MessageDigest.isEqual(
        expected.toByteArray(Charsets.UTF_8),
        actual.toByteArray(Charsets.UTF_8),
    )

    private fun randomUrlToken(byteCount: Int): String {
        val bytes = ByteArray(byteCount)
        random.nextBytes(bytes)
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes)
    }
}
