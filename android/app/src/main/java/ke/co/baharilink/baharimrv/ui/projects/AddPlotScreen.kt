package ke.co.baharilink.baharimrv.ui.projects

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.location.LocationManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import ke.co.baharilink.baharimrv.data.remote.CreatePlotRequestDto

@Composable
fun AddPlotScreen(
    projectId: String,
    onBack: () -> Unit,
    onSaved: () -> Unit,
) {
    val context = LocalContext.current
    var siteName by remember { mutableStateOf("") }
    var code by remember { mutableStateOf("P1") }
    var area by remember { mutableStateOf("1.0") }
    var latitude by remember { mutableStateOf("") }
    var longitude by remember { mutableStateOf("") }
    var permissionMessage by remember { mutableStateOf("") }

    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { result ->
        val granted = result[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
            result[Manifest.permission.ACCESS_COARSE_LOCATION] == true
        if (granted) {
            val location = currentLocation(context)
            if (location != null) {
                latitude = location.first.toString()
                longitude = location.second.toString()
            } else {
                permissionMessage = "Location is unavailable. Enter coordinates manually."
            }
        } else {
            permissionMessage = "Location permission denied. Enter coordinates manually."
        }
    }

    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text("Add plot", style = MaterialTheme.typography.titleLarge)
        OutlinedTextField(value = siteName, onValueChange = { siteName = it }, label = { Text("Site name") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = code, onValueChange = { code = it }, label = { Text("Plot code") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = area, onValueChange = { area = it }, label = { Text("Area (ha)") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = latitude, onValueChange = { latitude = it }, label = { Text("Latitude") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = longitude, onValueChange = { longitude = it }, label = { Text("Longitude") }, modifier = Modifier.fillMaxWidth())
        if (permissionMessage.isNotBlank()) {
            Text(permissionMessage, color = MaterialTheme.colorScheme.error)
        }
        Button(onClick = {
            val fineGranted = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED
            val coarseGranted = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED
            if (!fineGranted && !coarseGranted) {
                launcher.launch(arrayOf(Manifest.permission.ACCESS_FINE_LOCATION, Manifest.permission.ACCESS_COARSE_LOCATION))
            } else {
                val location = currentLocation(context)
                if (location != null) {
                    latitude = location.first.toString()
                    longitude = location.second.toString()
                } else {
                    permissionMessage = "Location is unavailable. Enter coordinates manually."
                }
            }
        }, modifier = Modifier.fillMaxWidth()) {
            Text("Use current location")
        }
        Button(onClick = onSaved, modifier = Modifier.fillMaxWidth()) {
            Text("Save plot (wire to repository in next step)")
        }
        OutlinedButton(onClick = onBack, modifier = Modifier.fillMaxWidth()) { Text("Back") }
    }
}

private fun currentLocation(context: Context): Pair<Double, Double>? {
    val manager = context.getSystemService(Context.LOCATION_SERVICE) as? LocationManager ?: return null
    val providers = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)
    for (provider in providers) {
        if (!manager.isProviderEnabled(provider)) continue
        val location = runCatching { manager.getLastKnownLocation(provider) }.getOrNull() ?: continue
        return location.latitude to location.longitude
    }
    return null
}
