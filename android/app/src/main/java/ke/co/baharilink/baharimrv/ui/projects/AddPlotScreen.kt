package ke.co.baharilink.baharimrv.ui.projects

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import java.util.Locale
import ke.co.baharilink.baharimrv.domain.boundaryPolygon
import ke.co.baharilink.baharimrv.domain.closeRing
import ke.co.baharilink.baharimrv.domain.isValidBoundary
import ke.co.baharilink.baharimrv.domain.ringAreaHectares
import ke.co.baharilink.baharimrv.domain.ringCentroid
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.BahariChip
import ke.co.baharilink.baharimrv.ui.components.BoundaryCaptureMap
import ke.co.baharilink.baharimrv.ui.components.ChipColor
import ke.co.baharilink.baharimrv.ui.components.recentGpsFix

private enum class CaptureMode { TAP, WALK }

@Composable
fun AddPlotScreen(
    projectId: String,
    onBack: () -> Unit,
    onSaved: () -> Unit,
    viewModel: AddPlotViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val state by viewModel.state.collectAsStateWithLifecycle()
    var siteName by remember { mutableStateOf("") }
    var code by remember { mutableStateOf("P1") }
    var area by remember { mutableStateOf("1.0") }
    var latitude by remember { mutableStateOf("") }
    var longitude by remember { mutableStateOf("") }
    var mode by remember { mutableStateOf(CaptureMode.TAP) }
    val vertices = remember { mutableStateListOf<Pair<Double, Double>>() }
    var gpsAccuracy by remember { mutableStateOf<Double?>(null) }
    var message by remember { mutableStateOf("") }

    val ring = remember(vertices.toList()) { closeRing(vertices.toList()) }
    val surveyed = vertices.size >= 3 && isValidBoundary(ring)
    val derivedArea = if (surveyed) ringAreaHectares(ring) else null

    LaunchedEffect(state.savedPlotId) {
        if (state.savedPlotId != null) onSaved()
    }

    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { result ->
        val granted = result[Manifest.permission.ACCESS_FINE_LOCATION] == true ||
            result[Manifest.permission.ACCESS_COARSE_LOCATION] == true
        if (granted) {
            val fix = recentGpsFix(context)
            if (fix != null) {
                latitude = fix.latitude.toString()
                longitude = fix.longitude.toString()
                gpsAccuracy = fix.accuracyM
            } else {
                message = "Location is unavailable. Enter coordinates manually."
            }
        } else {
            message = "Location permission denied. Enter coordinates manually."
        }
    }

    fun requestLocation() {
        val fine = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED
        val coarse = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED
        if (!fine && !coarse) {
            launcher.launch(arrayOf(Manifest.permission.ACCESS_FINE_LOCATION, Manifest.permission.ACCESS_COARSE_LOCATION))
        } else {
            val fix = recentGpsFix(context)
            if (fix != null) {
                latitude = fix.latitude.toString()
                longitude = fix.longitude.toString()
                gpsAccuracy = fix.accuracyM
            } else {
                message = "Location is unavailable. Enter coordinates manually."
            }
        }
    }

    fun recordVertex() {
        val fine = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED
        val coarse = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED
        if (!fine && !coarse) {
            launcher.launch(arrayOf(Manifest.permission.ACCESS_FINE_LOCATION, Manifest.permission.ACCESS_COARSE_LOCATION))
            return
        }
        val fix = recentGpsFix(context)
        if (fix != null) {
            vertices.add(fix.longitude to fix.latitude)
            gpsAccuracy = fix.accuracyM
        } else {
            message = "Location is unavailable. Move outdoors or tap the map instead."
        }
    }

    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("Add plot", style = MaterialTheme.typography.titleLarge)

        BahariCard {
            OutlinedTextField(value = siteName, onValueChange = { siteName = it }, label = { Text("Site name") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(value = code, onValueChange = { code = it }, label = { Text("Plot code") }, singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
        }

        BahariCard {
            Text("Plot boundary", style = MaterialTheme.typography.titleSmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                BahariChip("Tap corners", selected = mode == CaptureMode.TAP, color = ChipColor.GREEN, onClick = { mode = CaptureMode.TAP })
                BahariChip("Walk perimeter", selected = mode == CaptureMode.WALK, color = ChipColor.GREEN, onClick = { mode = CaptureMode.WALK })
            }
            BoundaryCaptureMap(
                vertices = vertices,
                tapEnabled = { mode == CaptureMode.TAP },
                onVertex = { lon, lat -> vertices.add(lon to lat) },
                modifier = Modifier.fillMaxWidth().height(260.dp),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                BahariButton(onClick = { if (vertices.isNotEmpty()) vertices.removeAt(vertices.size - 1) }, variant = BahariButtonVariant.OUTLINED, small = true, enabled = vertices.isNotEmpty()) { Text("Undo") }
                BahariButton(onClick = { vertices.clear() }, variant = BahariButtonVariant.GHOST, small = true, enabled = vertices.isNotEmpty()) { Text("Clear") }
                if (mode == CaptureMode.WALK) {
                    BahariButton(onClick = { recordVertex() }, variant = BahariButtonVariant.GREEN, small = true) { Text("Record GPS vertex") }
                }
            }
            Text(
                if (mode == CaptureMode.TAP) "Tap the map to drop each corner of the plot. Drop at least three."
                else "Walk the plot edge and record a GPS vertex at each corner.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (surveyed && derivedArea != null) {
                Text(
                    "Surveyed outline · derived ${String.format(Locale.US, "%.2f", derivedArea)} ha",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            } else {
                Text(
                    "Approximate · point + stated area until an outline is captured",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        BahariCard {
            Text("Approximate fallback", style = MaterialTheme.typography.titleSmall)
            Text(
                "Used only when no outline is captured. The satellite sample becomes a square of this area around the point.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            OutlinedTextField(value = area, onValueChange = { area = it }, label = { Text("Area (ha)") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(value = latitude, onValueChange = { latitude = it }, label = { Text("Latitude") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(value = longitude, onValueChange = { longitude = it }, label = { Text("Longitude") }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal), singleLine = true, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth())
            BahariButton(onClick = { requestLocation() }, variant = BahariButtonVariant.OUTLINED, modifier = Modifier.fillMaxWidth()) { Text("Use current location") }
        }

        if (message.isNotBlank()) {
            Text(message, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        state.error?.let {
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }

        BahariButton(
            onClick = {
                message = ""
                if (siteName.isBlank() || code.isBlank()) {
                    message = "Site name and plot code are required."
                    return@BahariButton
                }
                if (surveyed && derivedArea != null) {
                    val (lat, lon) = ringCentroid(ring)
                    viewModel.save(
                        projectId, siteName, code, lat, lon,
                        String.format(Locale.US, "%.6f", derivedArea),
                        boundaryPolygon(ring), gpsAccuracy,
                    )
                } else {
                    val lat = latitude.toDoubleOrNull()
                    val lon = longitude.toDoubleOrNull()
                    if (lat == null || lon == null) {
                        message = "Capture an outline or enter coordinates."
                        return@BahariButton
                    }
                    if (area.toDoubleOrNull() == null) {
                        message = "Enter the plot area in hectares."
                        return@BahariButton
                    }
                    viewModel.save(projectId, siteName, code, lat, lon, area, null, gpsAccuracy)
                }
            },
            variant = BahariButtonVariant.GREEN,
            enabled = !state.saving,
            modifier = Modifier.fillMaxWidth(),
        ) { Text(if (state.saving) "Saving…" else "Save plot") }
        BahariButton(onClick = onBack, variant = BahariButtonVariant.OUTLINED, modifier = Modifier.fillMaxWidth()) { Text("Back") }
    }
}
