package ke.co.baharilink.baharimrv.location

import android.annotation.SuppressLint
import android.content.Context
import android.location.LocationListener
import android.location.LocationManager
import android.os.Build
import android.os.CancellationSignal
import android.os.Looper
import androidx.core.content.ContextCompat
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import kotlin.coroutines.resume

data class LocationFix(val latitude: Double, val longitude: Double, val accuracyMeters: Double)

class DeviceLocationRepository @Inject constructor(@ApplicationContext private val context: Context) {
    @SuppressLint("MissingPermission")
    suspend fun currentLocation(): LocationFix = withTimeout(20_000) {
        withContext(Dispatchers.Main.immediate) {
            suspendCancellableCoroutine { continuation ->
                val manager = context.getSystemService(LocationManager::class.java)
                val provider = when {
                    manager.isProviderEnabled(LocationManager.GPS_PROVIDER) -> LocationManager.GPS_PROVIDER
                    manager.isProviderEnabled(LocationManager.NETWORK_PROVIDER) -> LocationManager.NETWORK_PROVIDER
                    else -> {
                        continuation.cancel(IllegalStateException("Enable device location services and retry"))
                        return@suspendCancellableCoroutine
                    }
                }
                val receive: (android.location.Location) -> Unit = { location ->
                    if (continuation.isActive) {
                        if (!location.hasAccuracy()) continuation.cancel(IllegalStateException("The location fix has no accuracy; retry outdoors"))
                        else continuation.resume(LocationFix(location.latitude, location.longitude, location.accuracy.toDouble()))
                    }
                }
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                    val signal = CancellationSignal()
                    continuation.invokeOnCancellation { signal.cancel() }
                    manager.getCurrentLocation(provider, signal, ContextCompat.getMainExecutor(context), receive)
                } else {
                    lateinit var listener: LocationListener
                    listener = LocationListener { location ->
                        manager.removeUpdates(listener)
                        receive(location)
                    }
                    continuation.invokeOnCancellation { manager.removeUpdates(listener) }
                    manager.requestSingleUpdate(provider, listener, Looper.getMainLooper())
                }
            }
        }
    }
}
