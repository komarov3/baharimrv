package ke.co.baharilink.baharimrv.data.remote

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.distinctUntilChanged

/** Online means the system reports a network whose capabilities are validated, not merely connected. */
@Singleton
class NetworkMonitor @Inject constructor(@ApplicationContext context: Context) {
    private val manager: ConnectivityManager? = context.getSystemService(ConnectivityManager::class.java)

    val isOnline: Flow<Boolean> = callbackFlow {
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) { trySend(isValidated(network)) }

            override fun onLost(network: Network) { trySend(isValidated(manager?.activeNetwork)) }

            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                trySend(isValidated(network))
            }
        }
        trySend(isValidated(manager?.activeNetwork))
        manager?.registerNetworkCallback(NetworkRequest.Builder().build(), callback)
        awaitClose { manager?.unregisterNetworkCallback(callback) }
    }.distinctUntilChanged()

    /** Named apart from `ConnectivityManager.isConnected` so the call cannot resolve against the core-ktx extension. */
    private fun isValidated(network: Network?): Boolean {
        if (network == null) return false
        val capabilities = manager?.getNetworkCapabilities(network) ?: return false
        return capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) &&
            capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
    }
}
