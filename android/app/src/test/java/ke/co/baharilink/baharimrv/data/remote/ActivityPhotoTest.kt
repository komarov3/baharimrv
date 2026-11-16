package ke.co.baharilink.baharimrv.data.remote

import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.*
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

class ActivityPhotoTest {
    private val server = MockWebServer()
    private val repository = ProjectRepository(
        Retrofit.Builder().baseUrl(server.url("/"))
            .client(OkHttpClient.Builder().addInterceptor { chain ->
                chain.proceed(chain.request().newBuilder().header("Authorization", "Bearer test-token").build())
            }.build())
            .addConverterFactory(GsonConverterFactory.create()).build().create(BahariMrvApi::class.java)
    )

    @After fun close() { server.shutdown() }

    @Test fun `upload hashes actual bytes and uses activity endpoint`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(201).setBody("""
            {"id":"photo-1","activityId":"activity-1","kind":"photo","sha256":"server-hash",
             "mediaType":"image/png","sizeBytes":3,"createdAt":"2026-11-16T10:00:00Z"}
        """.trimIndent()))
        val photo = repository.uploadActivityPhoto("activity-1", "abc".toByteArray(), "image/png")
        assertEquals("photo-1", photo.id)
        assertNull(photo.capturedAt)
        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertEquals("/api/v1/activities/activity-1/evidence", request.path)
        assertEquals("Bearer test-token", request.getHeader("Authorization"))
        val multipart = request.body.readUtf8()
        assertTrue(multipart.contains("name=\"file\"; filename=\"photo\""))
        assertTrue(multipart.contains("Content-Type: image/png"))
        assertTrue(multipart.contains("\r\nabc\r\n"))
        assertTrue(multipart.contains("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"))
        assertTrue(multipart.contains("name=\"kind\""))
        assertTrue(multipart.contains("name=\"mediaType\""))
    }

    @Test fun `rejected upload remains a failure and does not recreate activity`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(422).setBody("""{"message":"hash mismatch"}"""))
        try {
            repository.uploadActivityPhoto("saved-id", byteArrayOf(1), "image/png")
            fail("Rejected upload must not report success")
        } catch (e: HttpException) { assertEquals(422, e.code()) }
        assertEquals(1, server.requestCount)
        assertEquals("/api/v1/activities/saved-id/evidence", server.takeRequest().path)
    }

    @Test fun `activity photo download uses authenticated proxy`() = runBlocking {
        server.enqueue(MockResponse().setBody("photo bytes"))
        val bytes = repository.activityEvidenceMedia("a1", "p1").use { it.string() }
        assertEquals("photo bytes", bytes)
        val request = server.takeRequest()
        assertEquals("/api/v1/activities/a1/evidence/p1", request.path)
        assertEquals("Bearer test-token", request.getHeader("Authorization"))
    }

    @Test fun `detail keeps activity photos separate from monitoring evidence`() = runBlocking {
        server.enqueue(MockResponse().setBody("""
            {"id":"a1","projectId":"project-1","plotId":"plot-1","activityType":"planting",
             "title":"North creek","status":"draft","canManage":true,
             "activityEvidence":[{"id":"p1","activityId":"a1","kind":"photo","sha256":"abc",
             "mediaType":"image/png","sizeBytes":12,"createdAt":"2026-11-16T10:00:00Z"}],"record":null}
        """.trimIndent()))
        val detail = repository.activityDetail("a1")
        assertEquals("North creek", detail.title)
        assertTrue(detail.canManage)
        assertEquals("p1", detail.activityEvidence.orEmpty().single().id)
        assertNull(detail.record)
    }
}
