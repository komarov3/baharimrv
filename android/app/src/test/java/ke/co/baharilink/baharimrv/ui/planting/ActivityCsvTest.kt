package ke.co.baharilink.baharimrv.ui.planting

import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ActivityCsvTest {
    private fun activity(
        id: String = "act-1",
        plotId: String = "Plot A",
        activityType: String = "planting",
        status: String = "open",
        methodologyVersion: String? = "VM0033",
        periodStart: String? = "2026-01-01",
        periodEnd: String? = "2026-03-31",
    ) = ProjectActivityDto(
        id = id,
        projectId = "proj-1",
        plotId = plotId,
        activityType = activityType,
        status = status,
        periodStart = periodStart,
        periodEnd = periodEnd,
        methodologyVersion = methodologyVersion,
    )

    @Test
    fun blankQueryMatchesEverything() {
        assertTrue(matchesQuery(activity(), ""))
        assertTrue(matchesQuery(activity(), "   "))
    }

    @Test
    fun queryMatchesAcrossFieldsCaseInsensitive() {
        val a = activity(plotId = "Kilifi Creek", status = "complete", methodologyVersion = "VM0033")
        assertTrue(matchesQuery(a, "kilifi"))
        assertTrue(matchesQuery(a, "COMPLETE"))
        assertTrue(matchesQuery(a, "vm0033"))
        assertTrue(matchesQuery(a, "planting"))
        assertTrue(matchesQuery(a, "2026-01"))
        assertTrue(matchesQuery(a, "act-1"))
    }

    @Test
    fun queryRejectsNonMatching() {
        assertFalse(matchesQuery(activity(plotId = "Kilifi"), "mombasa"))
    }

    @Test
    fun csvHasHeaderAndOneRowPerActivity() {
        val csv = activitiesToCsv(listOf(activity(id = "a"), activity(id = "b")))
        val lines = csv.trim().split("\r\n")
        assertEquals(3, lines.size)
        assertEquals(
            "Activity ID,Project ID,Plot,Activity Type,Status,Period Start,Period End,Methodology Version",
            lines[0],
        )
        assertTrue(lines[1].startsWith("a,proj-1,"))
    }

    @Test
    fun emptyListYieldsHeaderOnly() {
        val csv = activitiesToCsv(emptyList())
        assertEquals(1, csv.trim().split("\r\n").size)
    }

    @Test
    fun csvEscapesCommasAndQuotes() {
        val csv = activitiesToCsv(listOf(activity(plotId = "Plot, \"A\"")))
        assertTrue(csv.contains("\"Plot, \"\"A\"\"\""))
    }

    @Test
    fun nullFieldsBecomeEmpty() {
        val csv = activitiesToCsv(listOf(activity(methodologyVersion = null, periodStart = null, periodEnd = null)))
        val row = csv.trim().split("\r\n")[1]
        assertTrue(row.endsWith(",,"))
    }
}