package ke.co.baharilink.baharimrv

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dagger.hilt.android.AndroidEntryPoint
import ke.co.baharilink.baharimrv.ui.AppRoot
import ke.co.baharilink.baharimrv.ui.theme.BahariMrvTheme
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import ke.co.baharilink.baharimrv.sync.SatelliteNotificationWorker

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    private var authCallback by mutableStateOf<Uri?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        authCallback = intent?.data.takeIf { intent?.action == Intent.ACTION_VIEW }
        val workRequest = PeriodicWorkRequestBuilder<SatelliteNotificationWorker>(12, java.util.concurrent.TimeUnit.HOURS).build()
        WorkManager.getInstance(this).enqueueUniquePeriodicWork(
            SatelliteNotificationWorker.UNIQUE_WORK,
            ExistingPeriodicWorkPolicy.KEEP,
            workRequest,
        )
        setContent {
            BahariMrvTheme {
                AppRoot(
                    authCallback = authCallback,
                    onAuthCallbackHandled = { authCallback = null },
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        if (intent.action == Intent.ACTION_VIEW) authCallback = intent.data
    }
}
