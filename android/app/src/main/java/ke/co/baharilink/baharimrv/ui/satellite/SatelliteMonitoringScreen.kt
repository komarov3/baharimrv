package ke.co.baharilink.baharimrv.ui.satellite

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Satellite
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.google.android.gms.maps.model.BitmapDescriptorFactory
import com.google.android.gms.maps.model.CameraPosition
import com.google.android.gms.maps.model.LatLng
import com.google.maps.android.compose.GoogleMap
import com.google.maps.android.compose.MapProperties
import com.google.maps.android.compose.MapType
import com.google.maps.android.compose.MapUiSettings
import com.google.maps.android.compose.Marker
import com.google.maps.android.compose.MarkerState
import com.google.maps.android.compose.rememberCameraPositionState
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.StatusChip

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SatelliteMonitoringScreen(
    viewModel: SatelliteMonitoringViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state   by viewModel.state.collectAsStateWithLifecycle()
    val markers = viewModel.plotMarkers()

    LaunchedEffect(projectId) {
        viewModel.setProjectId(projectId)
        if (projectId.isNotBlank()) viewModel.load(projectId, "")
    }

    // Centre map on first plot, or default to Kenyan coast
    val centre = remember(markers) {
        markers.firstOrNull()
            ?.let { LatLng(it.latitude, it.longitude) }
            ?: LatLng(-4.05, 39.67)
    }
    val cameraPositionState = rememberCameraPositionState {
        position = CameraPosition.fromLatLngZoom(centre, if (markers.isEmpty()) 10f else 14f)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Satellite monitoring") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back")
                    }
                },
            )
        },
    ) { padding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding),
            contentPadding = PaddingValues(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {

            // ── Map with NDVI plot markers ─────────────────────────────────
            // An unconfigured key authenticates every GoogleMap call as a failure,
            // so a checkout without one keeps the plot table and pass cards below.
            if (BuildConfig.MAPS_CONFIGURED) {
                item {
                    Box(modifier = Modifier.fillMaxWidth().height(280.dp)) {
                        GoogleMap(
                            modifier = Modifier.fillMaxSize(),
                            cameraPositionState = cameraPositionState,
                            properties = MapProperties(mapType = MapType.SATELLITE),
                            uiSettings = MapUiSettings(zoomControlsEnabled = true, myLocationButtonEnabled = false),
                        ) {
                            markers.forEach { marker ->
                                val hue = ndviToHue(marker.ndvi)
                                Marker(
                                    state = MarkerState(position = LatLng(marker.latitude, marker.longitude)),
                                    title = marker.label,
                                    snippet = marker.ndvi?.let { "NDVI: ${"%.2f".format(it)}" } ?: "NDVI: N/A",
                                    icon = BitmapDescriptorFactory.defaultMarker(hue),
                                )
                            }
                        }
                    }
                }
            }

            // ── NDVI legend ───────────────────────────────────────────────
            item {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    NdviLegendChip("Healthy ≥0.6", Color(0xFF388E3C))
                    NdviLegendChip("Moderate 0.4–0.6", Color(0xFFFB8C00))
                    NdviLegendChip("Stressed <0.4", Color(0xFFF44336))
                    NdviLegendChip("No data", Color(0xFF9E9E9E))
                }
            }

            // ── Plot NDVI table ───────────────────────────────────────────
            if (markers.isNotEmpty()) {
                item {
                    BahariCard(modifier = Modifier.padding(horizontal = 16.dp)) {
                        Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Icon(Icons.Default.Satellite, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(18.dp))
                            Text("Plot NDVI Overview", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                        }
                        Spacer(Modifier.height(8.dp))
                        markers.forEach { marker ->
                            Row(modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                                Text(marker.label, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                                val (ndviText, tone) = ndviDisplay(marker.ndvi)
                                StatusChip(text = ndviText, tone = tone)
                            }
                        }
                    }
                }
            }

            // ── Latest pass ───────────────────────────────────────────────
            item {
                Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    when (val lp = state.latestPass) {
                        is LoadState.Loading -> LoadingState("Loading satellite data…")
                        is LoadState.Error   -> ErrorState(message = lp.message, onRetry = viewModel::retry)
                        is LoadState.Success -> {
                            val pass = lp.data
                            if (pass != null) {
                                BahariCard {
                                    Text("Latest Pass", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                                    Spacer(Modifier.height(8.dp))
                                    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                        Text(pass.sceneId ?: pass.id, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                                        pass.cloudCoverPct?.let { Text("Cloud ${it.toInt()}%", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                                        StatusChip(text = pass.adapterMode ?: "sandbox", tone = ChipTone.NEUTRAL)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ── Passes list ───────────────────────────────────────────────
            item {
                Text("Satellite Passes", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp))
            }

            val passList = (state.passes as? LoadState.Success)?.data ?: emptyList()
            if (passList.isEmpty()) {
                item {
                    EmptyState(title = "No satellite passes yet", detail = "NDVI corroboration appears after a provider-neutral tile match returns.")
                }
            } else {
                items(passList) { pass ->
                    BahariCard(modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
                        Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                            Text(pass.sceneId ?: pass.id, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                            pass.cloudCoverPct?.let { Text("Cloud ${it.toInt()}%", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                            StatusChip(text = pass.adapterMode ?: "sandbox", tone = ChipTone.NEUTRAL)
                        }
                    }
                }
            }

            item {
                BahariButton(onClick = viewModel::retry, variant = BahariButtonVariant.OUTLINED, modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp)) {
                    Text("Refresh satellite data")
                }
            }
        }
    }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Map an NDVI value to a Google Maps marker hue (0=red … 120=green). */
private fun ndviToHue(ndvi: Double?): Float = when (ndvi) {
    null            -> BitmapDescriptorFactory.HUE_AZURE   // grey-ish — no data
    in 0.6..1.0     -> BitmapDescriptorFactory.HUE_GREEN
    in 0.4..0.6     -> BitmapDescriptorFactory.HUE_ORANGE
    in 0.2..0.4     -> BitmapDescriptorFactory.HUE_RED
    else            -> BitmapDescriptorFactory.HUE_ROSE
}

private fun ndviDisplay(ndvi: Double?): Pair<String, ChipTone> = when (ndvi) {
    null            -> "NDVI N/A" to ChipTone.NEUTRAL
    in 0.6..1.0     -> "NDVI ${"%.2f".format(ndvi)}" to ChipTone.POSITIVE
    in 0.4..0.6     -> "NDVI ${"%.2f".format(ndvi)}" to ChipTone.WARNING
    else            -> "NDVI ${"%.2f".format(ndvi)}" to ChipTone.WARNING
}

@Composable
private fun NdviLegendChip(label: String, color: Color) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(modifier = Modifier.size(10.dp).padding(0.dp), contentAlignment = Alignment.Center) {
            androidx.compose.foundation.Canvas(modifier = Modifier.size(10.dp)) {
                drawCircle(color = color)
            }
        }
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
