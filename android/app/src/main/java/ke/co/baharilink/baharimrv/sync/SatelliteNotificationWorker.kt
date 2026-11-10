package ke.co.baharilink.baharimrv.sync

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import java.time.Instant

@HiltWorker
class SatelliteNotificationWorker @AssistedInject constructor(
    @Assisted appContext: Context,
    @Assisted workerParams: WorkerParameters,
    private val repository: ProjectRepository,
) : CoroutineWorker(appContext, workerParams) {

    override suspend fun doWork(): Result {
        return try {
            val projects = repository.projects().rows
            projects.forEach { project ->
                val pass = runCatching { repository.satelliteLatest(project.id, "S2") }.getOrNull()
                val windowStart = pass?.windowStart?.let { runCatching { Instant.parse(it) }.getOrNull() }
                val now = Instant.now()
                if (windowStart != null) {
                    when {
                        windowStart.isAfter(now) && windowStart.isBefore(now.plusSeconds(24 * 60 * 60)) ->
                            notify(project.name, "Sentinel-2 pass expected around $windowStart")
                        windowStart.isBefore(now) && windowStart.isAfter(now.minusSeconds(24 * 60 * 60)) ->
                            notify(project.name, "Sentinel-2 pass completed; NDVI corroboration may now be available")
                    }
                }
            }
            Result.success()
        } catch (e: Exception) {
            Result.retry()
        }
    }

    private fun notify(title: String, message: String) {
        val manager = applicationContext.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(CHANNEL_ID, "Satellite passes", NotificationManager.IMPORTANCE_DEFAULT)
            manager.createNotificationChannel(channel)
        }
        val notification = NotificationCompat.Builder(applicationContext, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_mylocation)
            .setContentTitle(title)
            .setContentText(message)
            .setAutoCancel(true)
            .build()
        manager.notify(title.hashCode(), notification)
    }

    companion object {
        const val CHANNEL_ID = "satellite_passes"
        const val UNIQUE_WORK = "satellite_pass_notifications"
    }
}
