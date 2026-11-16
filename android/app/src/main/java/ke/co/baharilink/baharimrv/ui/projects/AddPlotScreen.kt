package ke.co.baharilink.baharimrv.ui.projects

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.graphics.Color
import android.location.Location
import android.location.LocationManager
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
import androidx.compose.runtime.DisposableEffect
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
import androidx.compose.ui.viewinterop.AndroidView
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
import ke.co.baharilink.baharimrv.ui.components.ChipColor
import org.osmdroid.events.MapEventsReceiver
import org.osmdroid.tileprovider.tilesource.TileSourceFactory
import org.osmdroid.util.GeoPoint
import org.osmdroid.views.MapView
import org.osmdroid.views.overlay.MapEventsOverlay
import org.osmdroid.views.overlay.Marker
import org.osmdroid.views.overlay.Polygon

private enum class CaptureMode { TAP, WALK }

private data class LocationFix(val latitude: Double, val longitude: Double, val accuracyM: Double?)

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
            val fix = currentFix(context)
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
            val fix = currentFix(context)
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
        val fix = currentFix(context)
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
            BoundaryMap(
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

@Composable
private fun BoundaryMap(
    vertices: List<Pair<Double, Double>>,
    tapEnabled: () -> Boolean,
    onVertex: (Double, Double) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val mapView = remember {
        MapView(context).apply {
            setTileSource(TileSourceFactory.MAPNIK)
            setMultiTouchControls(true)
            setBuiltInZoomControls(false)
            controller.setZoom(16.0)
            val start = currentFix(context)
            controller.setCenter(if (start != null) GeoPoint(start.latitude, start.longitude) else GeoPoint(-4.05, 39.67))
            overlays.add(
                MapEventsOverlay(
                    object : MapEventsReceiver {
                        override fun singleTapConfirmedHelper(p: GeoPoint): Boolean {
                            if (tapEnabled()) {
                                onVertex(p.longitude, p.latitude)
                                return true
                            }
                            return false
                        }

                        override fun longPressHelper(p: GeoPoint): Boolean = false
                    },
                ),
            )
        }
    }
    DisposableEffect(mapView) {
        mapView.onResume()
        onDispose {
            mapView.onPause()
            mapView.onDetach()
        }
    }
    AndroidView(
        factory = { mapView },
        modifier = modifier,
        update = { mv ->
            while (mv.overlays.size > 1) mv.overlays.removeAt(mv.overlays.size - 1)
            if (vertices.size >= 2) {
                val polygon = Polygon().apply {
                    points = vertices.map { GeoPoint(it.second, it.first) }
                    fillColor = Color.argb(48, 0, 128, 0)
                    strokeColor = Color.GREEN
                    strokeWidth = 3f
                }
                mv.overlays.add(polygon)
            }
            vertices.forEach { vertex ->
                val marker = Marker(mv).apply {
                    position = GeoPoint(vertex.second, vertex.first)
                    setAnchor(Marker.ANCHOR_CENTER, Marker.ANCHOR_CENTER)
                }
                mv.overlays.add(marker)
            }
            mv.invalidate()
        },
    )
}

private fun currentFix(context: Context): LocationFix? {
    val manager = context.getSystemService(Context.LOCATION_SERVICE) as? LocationManager ?: return null
    val providers = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)
    for (provider in providers) {
        if (!manager.isProviderEnabled(provider)) continue
        val location: Location = runCatching { manager.getLastKnownLocation(provider) }.getOrNull() ?: continue
        val accuracy = if (location.hasAccuracy()) location.accuracy.toDouble() else null
        return LocationFix(location.latitude, location.longitude, accuracy)
    }
    return null
}