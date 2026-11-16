package ke.co.baharilink.baharimrv.data.remote

import com.google.gson.Gson
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.Assert.*
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

class ActivityNdviTest {
    private val gson = Gson()

    @Test fun `missing imagery remains null rather than zero`() {
        for (status in listOf("waiting", "queued", "running", "no_observation", "cloudy", "failed")) {
            val view = gson.fromJson("""{"status":"$status","observation":null,"changeAbs":null,"changePct":null}""", ActivityNdviDto::class.java)
            assertEquals(status, view.status)
            assertNull(view.observation)
            assertNull(view.changeAbs)
            assertNull(view.changePct)
            assertNull(view.latestPass)
        }
    }

    @Test fun `sandbox retains synthetic label and no acquisition`() {
        val view = gson.fromJson("""
            {"status":"synthetic","observation":{"id":"o1","meanNdvi":0.64,
             "acquiredAt":null,"cloudPct":null,"coveragePct":null,"quality":"synthetic",
             "adapterMode":"sandbox","provider":"sandbox","processingVersion":"sandbox-ndvi/1","resultHash":"hash"}}
        """.trimIndent(), ActivityNdviDto::class.java)
        assertEquals("sandbox", view.observation!!.adapterMode)
        assertEquals("synthetic", view.observation.quality)
        assertNull(view.observation.acquiredAt)
        assertNull(view.observation.coveragePct)
        assertNull(view.changeAbs)
    }

    @Test fun `server change is not reconstructed from displayed readings`() {
        val view = gson.fromJson("""
            {"status":"complete","changeAbs":0.1234,"changePct":24.68,
             "observation":{"id":"o1","meanNdvi":0.9,"quality":"complete","adapterMode":"live"},
             "baseline":{"id":"b1","meanNdvi":0.2,"quality":"partial","adapterMode":"live"}}
        """.trimIndent(), ActivityNdviDto::class.java)
        assertEquals(0.1234, view.changeAbs!!, 0.0)
        assertEquals(24.68, view.changePct!!, 0.0)
        assertEquals("partial", view.baseline!!.quality)
    }

    @Test fun `capture and status use activity scoped endpoints`() = runBlocking {
        val server = MockWebServer()
        try {
            val repository = ProjectRepository(Retrofit.Builder().baseUrl(server.url("/"))
                .addConverterFactory(GsonConverterFactory.create()).build().create(BahariMrvApi::class.java))
            server.enqueue(MockResponse().setResponseCode(202).setBody("""{"status":"waiting","centerAt":"2026-11-16T00:00:00Z"}"""))
            assertEquals("waiting", repository.recaptureActivityNdvi("a1").status)
            val capture = server.takeRequest()
            assertEquals("POST", capture.method)
            assertEquals("/api/v1/activities/a1/ndvi/recapture", capture.path)
            server.enqueue(MockResponse().setBody("""{"status":"failed","error":"The imagery provider did not answer."}"""))
            val status = repository.activityNdvi("a1")
            assertEquals("failed", status.status)
            assertEquals("The imagery provider did not answer.", status.error)
            val read = server.takeRequest()
            assertEquals("GET", read.method)
            assertEquals("/api/v1/activities/a1/ndvi", read.path)
        } finally { server.shutdown() }
    }
}
