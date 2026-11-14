package ke.co.baharilink.baharimrv.data.remote

import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import ke.co.baharilink.baharimrv.data.local.TokenSet
import ke.co.baharilink.baharimrv.data.local.TokenStore
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

class SessionAuthenticatorTest {

    private class FakeTokens(
        @Volatile var access: String? = "old-access",
        @Volatile var refresh: String? = "old-refresh",
    ) : TokenStore {
        override fun accessToken(): String? = access
        override fun refreshToken(): String? = refresh
        override fun saveTokens(tokens: TokenSet) {
            access = tokens.accessToken
            refresh = tokens.refreshToken
        }
    }

    private lateinit var server: MockWebServer
    private lateinit var tokens: FakeTokens
    private val invalidations = AtomicInteger()
    private val refreshCalls = AtomicInteger()

    @Before
    fun setUp() {
        server = MockWebServer()
        tokens = FakeTokens()
        invalidations.set(0)
        refreshCalls.set(0)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun refreshJson(access: String, refresh: String): String = """
        {"user":{"id":"u1","email":"m@example.test","name":"Manager","role":"project_manager","status":"active"},
         "identity":{"id":"i1","userId":"u1","state":"active"},
         "sessionToken":"$access","expiresAt":"2030-01-01T00:00:00Z",
         "refreshToken":"$refresh","refreshExpiresAt":"2030-01-08T00:00:00Z"}
    """.trimIndent()

    private fun client(): OkHttpClient {
        val refreshApi = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(GsonConverterFactory.create())
            .build()
            .create(BahariMrvApi::class.java)
        val authenticator = SessionAuthenticator(tokens, refreshApi) {
            invalidations.incrementAndGet()
            tokens.access = null
            tokens.refresh = null
        }
        return OkHttpClient.Builder()
            .addInterceptor { chain ->
                val builder = chain.request().newBuilder()
                tokens.access?.let { builder.header("Authorization", "Bearer $it") }
                chain.proceed(builder.build())
            }
            .authenticator(authenticator)
            .build()
    }

    private fun get(path: String): okhttp3.Response =
        client().newCall(Request.Builder().url(server.url(path)).build()).execute()

    @Test
    fun `401 triggers one refresh and retries with the new bearer`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when {
                request.path == "/api/v1/auth/refresh" -> {
                    refreshCalls.incrementAndGet()
                    MockResponse().setBody(refreshJson("new-access", "new-refresh"))
                }
                request.getHeader("Authorization") == "Bearer new-access" -> MockResponse().setBody("{}")
                else -> MockResponse().setResponseCode(401)
            }
        }
        val response = get("/api/v1/projects")
        assertEquals(200, response.code)
        assertEquals(1, refreshCalls.get())
        assertEquals("new-access", tokens.access)
        assertEquals("new-refresh", tokens.refresh)
        assertEquals(0, invalidations.get())
        response.close()
    }

    @Test
    fun `rejected grant clears the session and propagates the 401`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse =
                MockResponse().setResponseCode(401)
        }
        val response = get("/api/v1/projects")
        assertEquals(401, response.code)
        assertEquals(1, invalidations.get())
        assertNull(tokens.access)
        assertNull(tokens.refresh)
        response.close()
    }

    @Test
    fun `missing refresh grant clears the session without a network call`() {
        tokens.refresh = null
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse =
                MockResponse().setResponseCode(401)
        }
        val response = get("/api/v1/projects")
        assertEquals(401, response.code)
        assertEquals(1, invalidations.get())
        assertEquals(0, refreshCalls.get())
        response.close()
    }

    @Test
    fun `failed sign-in never triggers a refresh`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse =
                MockResponse().setResponseCode(401)
        }
        val response = get("/api/v1/auth/login")
        assertEquals(401, response.code)
        assertEquals(0, invalidations.get())
        assertEquals("old-access", tokens.access)
        response.close()
    }

    @Test
    fun `network failure during refresh keeps the session`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
                "/api/v1/auth/refresh" -> MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START)
                else -> MockResponse().setResponseCode(401)
            }
        }
        val response = get("/api/v1/projects")
        assertEquals(401, response.code)
        assertEquals(0, invalidations.get())
        assertEquals("old-access", tokens.access)
        assertEquals("old-refresh", tokens.refresh)
        response.close()
    }

    @Test
    fun `concurrent 401s share a single refresh`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when {
                request.path == "/api/v1/auth/refresh" -> {
                    refreshCalls.incrementAndGet()
                    Thread.sleep(150)
                    MockResponse().setBody(refreshJson("new-access", "new-refresh"))
                }
                request.getHeader("Authorization") == "Bearer new-access" -> MockResponse().setBody("{}")
                else -> MockResponse().setResponseCode(401)
            }
        }
        val client = client()
        val pool = Executors.newFixedThreadPool(4)
        val start = CountDownLatch(1)
        val codes = (1..4).map {
            pool.submit<Int> {
                start.await()
                client.newCall(Request.Builder().url(server.url("/api/v1/projects")).build())
                    .execute().use { it.code }
            }
        }
        start.countDown()
        codes.forEach { assertEquals(200, it.get(10, TimeUnit.SECONDS)) }
        pool.shutdown()
        assertEquals(1, refreshCalls.get())
        assertEquals(0, invalidations.get())
    }
}