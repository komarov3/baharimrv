package ke.co.baharilink.baharimrv.ui.planting

import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import java.io.File
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto

/** Case-insensitive match across the summary fields shown on the list cards. */
internal fun matchesQuery(activity: ProjectActivityDto, query: String): Boolean {
    val q = query.trim().lowercase()
    if (q.isEmpty()) return true
    return listOf(
        activity.plotId,
        activity.activityType,
        activity.status,
        activity.methodologyVersion.orEmpty(),
        activity.periodStart.orEmpty(),
        activity.periodEnd.orEmpty(),
        activity.id,
    ).any { it.lowercase().contains(q) }
}

internal fun activitiesToCsv(items: List<ProjectActivityDto>): String {
    val header = listOf(
        "Activity ID", "Project ID", "Plot", "Activity Type",
        "Status", "Period Start", "Period End", "Methodology Version",
    )
    val rows = items.map { a ->
        listOf(
            a.id, a.projectId, a.plotId, a.activityType,
            a.status, a.periodStart.orEmpty(), a.periodEnd.orEmpty(), a.methodologyVersion.orEmpty(),
        )
    }
    return (listOf(header) + rows).joinToString("\r\n") { row ->
        row.joinToString(",") { csvField(it) }
    } + "\r\n"
}

private fun csvField(value: String): String {
    val needsQuote = value.any { it == ',' || it == '"' || it == '\n' || it == '\r' }
    return if (needsQuote) "\"" + value.replace("\"", "\"\"") + "\"" else value
}

/** Writes the filtered activities to a cache CSV and opens the system share sheet. */
internal fun shareActivitiesCsv(context: Context, items: List<ProjectActivityDto>) {
    val dir = File(context.cacheDir, "exports").apply { mkdirs() }
    val file = File(dir, "planting-activities-${System.currentTimeMillis()}.csv")
    file.writeText(activitiesToCsv(items))
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "text/csv"
        putExtra(Intent.EXTRA_STREAM, uri)
        putExtra(Intent.EXTRA_SUBJECT, "Planting activities export")
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    context.startActivity(Intent.createChooser(send, "Export planting activities"))
}