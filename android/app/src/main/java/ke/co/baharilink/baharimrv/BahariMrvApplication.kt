package ke.co.baharilink.baharimrv

import android.app.Application
import androidx.hilt.work.HiltWorkerFactory
import androidx.work.Configuration
import dagger.hilt.android.HiltAndroidApp
import javax.inject.Inject

@HiltAndroidApp
class BahariMrvApplication : Application(), Configuration.Provider {
    @Inject lateinit var workerFactory: HiltWorkerFactory

    override fun onCreate() {
        super.onCreate()
        // osmdroid needs its cache paths and an identifying User-Agent before any
        // MapView is built. The OSM tile policy requires a real app agent string.
        org.osmdroid.config.Configuration.getInstance().apply {
            load(this@BahariMrvApplication, getSharedPreferences("osmdroid", MODE_PRIVATE))
            userAgentValue = packageName
        }
    }

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().setWorkerFactory(workerFactory).build()
}
