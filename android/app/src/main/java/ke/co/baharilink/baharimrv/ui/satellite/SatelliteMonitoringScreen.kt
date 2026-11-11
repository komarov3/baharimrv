package ke.co.baharilink.baharimrv.ui.satellite

import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Satellite
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.google.android.gms.maps.CameraUpdateFactory
import com.google.android.gms.maps.model.CameraPosition
import com.google.android.gms.maps.model.LatLng
import com.google.android.gms.maps.model.LatLngBounds
import com.google.maps.android.compose.GoogleMap
import com.google.maps.android.compose.MapProperties
import com.google.maps.android.compose.MapType
import com.google.maps.android.compose.MapUiSettings
import com.google.maps.android.compose.Marker
import com.google.maps.android.compose.MarkerState
import com.google.maps.android.compose.TileOverlay
import com.google.maps.android.compose.rememberCameraPositionState
import com.google.maps.android.compose.rememberTileOverlayState
import java.time.Instant
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.temporal.ChronoUnit
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.R
import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteLegendStopDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.data.remote.SatelliteRefreshDto
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.stateTone

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SatelliteMonitoringScreen(
    viewModel: SatelliteMonitoringViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val markers = viewModel.plotMarkers()

    LaunchedEffect(projectId) {
        viewModel.setProjectId(projectId)
        if (projectId.isNotBlank()) viewModel.load(projectId, "")
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.satellite_title)) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, stringResource(R.string.common_back))
                    }
                },
            )
        },
    ) { padding ->
        val overlay = (state.overlay as? LoadState.Success)?.data
        val change = (state.change as? LoadState.Success)?.data
        val adapterMode = overlay?.adapterMode ?: "sandbox"
        val isSandbox = adapterMode.equals("sandbox", ignoreCase = true)
        val tileProvider = remember(state.activeScene) { viewModel.tileProvider() }

        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding),
            contentPadding = PaddingValues(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (BuildConfig.MAPS_CONFIGURED && overlay != null && !isSandbox) {
                item(key = "imagery") {
                    ImageryCard(
                        overlay = overlay,
                        markers = markers,
                        tileProvider = tileProvider,
                    )
                }
            }

            if (overlay == null || isSandbox) {
                item(key = "sandbox") {
                    SandboxNotice(modifier = Modifier.padding(horizontal = 16.dp))
                }
            }

            item(key = "latest") {
                when (val lp = state.latestPass) {
                    is LoadState.Loading -> LoadingState(stringResource(R.string.satellite_loading_passes))
                    is LoadState.Error -> ErrorState(message = lp.message, onRetry = viewModel::retry)
                    is LoadState.Success -> {
                        val pass = lp.data
                        if (pass != null) {
                            LastPassCard(
                                pass = pass,
                                modifier = Modifier.padding(horizontal = 16.dp),
                            )
                        }
                    }
                }
            }

            if (change != null) {
                item(key = "change") {
                    ChangeCard(
                        change = change,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
            } else if (state.loading) {
                item(key = "change-loading") {
                    LoadingState(stringResource(R.string.satellite_loading_change))
                }
            }

            item(key = "acquisitions-title") {
                Text(
                    stringResource(R.string.satellite_acquisitions_title),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                )
            }

            val passList = (state.passes as? LoadState.Success)?.data ?: emptyList()
            if (passList.isEmpty()) {
                item(key = "acquisitions-empty") {
                    EmptyState(
                        title = stringResource(R.string.satellite_acquisitions_empty_title),
                        detail = stringResource(R.string.satellite_acquisitions_empty_detail),
                    )
                }
            } else {
                items(passList, key = { it.id }) { pass ->
                    AcquisitionRow(
                        pass = pass,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 2.dp),
                    )
                }
            }

            item(key = "refresh") {
                RefreshAction(
                    refreshJob = state.refreshJob,
                    refreshError = state.refreshError,
                    onClick = viewModel::refresh,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                )
            }
        }
    }
}

@Composable
private fun ImageryCard(
    overlay: SatelliteOverlayDto,
    markers: List<PlotMarker>,
    tileProvider: NdviTileProvider?,
) {
    BahariCard(modifier = Modifier.padding(horizontal = 16.dp)) {
        val centre = remember(markers, overlay) {
            markers.firstOrNull()?.let { LatLng(it.latitude, it.longitude) }
                ?: overlay.bbox?.let {
                    LatLng((it.minLat + it.maxLat) / 2.0, (it.minLon + it.maxLon) / 2.0)
                }
                ?: LatLng(-4.05, 39.67)
        }
        val cameraPositionState = rememberCameraPositionState {
            position = CameraPosition.fromLatLngZoom(centre, if (markers.isEmpty()) 12f else 14f)
        }
        val bbox = overlay.bbox
        LaunchedEffect(bbox, markers) {
            if (cameraPositionState.position.target == centre && bbox != null) {
                val bounds = LatLngBounds(
                    LatLng(bbox.minLat, bbox.minLon),
                    LatLng(bbox.maxLat, bbox.maxLon),
                )
                runCatching {
                    cameraPositionState.animate(
                        CameraUpdateFactory.newLatLngBounds(bounds, 48.dp.value.toInt()),
                    )
                }
            }
        }
        Box(modifier = Modifier.fillMaxWidth().height(280.dp)) {
            GoogleMap(
                modifier = Modifier.fillMaxSize(),
                cameraPositionState = cameraPositionState,
                properties = MapProperties(
                    mapType = MapType.HYBRID,
                    isMyLocationEnabled = false,
                    isTrafficEnabled = false,
                ),
                uiSettings = MapUiSettings(
                    zoomControlsEnabled = false,
                    scrollGesturesEnabled = true,
                    zoomGesturesEnabled = true,
                    rotationGesturesEnabled = false,
                    myLocationButtonEnabled = false,
                ),
            ) {
                if (tileProvider != null) {
                    TileOverlay(
                        state = rememberTileOverlayState(),
                        tileProvider = tileProvider,
                        transparency = 0.35f,
                        zIndex = 2f,
                    )
                }
                markers.forEach { marker ->
                    Marker(
                        state = MarkerState(position = LatLng(marker.latitude, marker.longitude)),
                        title = marker.label,
                        snippet = marker.ndvi?.let { ndviBandLabel(marker.ndvi) }
                            ?: stringResource(R.string.satellite_marker_no_ndvi),
                    )
                }
            }
            StatusChip(
                text = overlay.adapterMode,
                tone = stateTone(overlay.adapterMode),
                modifier = Modifier.align(Alignment.TopEnd).padding(8.dp),
            )
            overlay.acquiredAt?.let { acquired ->
                Surface(
                    modifier = Modifier.align(Alignment.TopStart).padding(8.dp),
                    color = Color(0xCC000000),
                    contentColor = Color.White,
                    shape = RoundedCornerShape(4.dp),
                ) {
                    Text(
                        stringResource(R.string.satellite_acquired_label, formatAcquiredDate(acquired)),
                        style = MaterialTheme.typography.labelSmall,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                    )
                }
            }
            LegendStrip(
                legend = overlay.legend,
                modifier = Modifier.align(Alignment.BottomStart).padding(8.dp),
            )
        }
    }
}

@Composable
private fun LegendStrip(legend: List<SatelliteLegendStopDto>, modifier: Modifier = Modifier) {
    if (legend.isEmpty()) return
    Surface(
        modifier = modifier,
        color = Color(0xCCFFFFFF),
        shape = RoundedCornerShape(6.dp),
    ) {
        Column(modifier = Modifier.padding(horizontal = 8.dp, vertical = 6.dp), verticalArrangement = Arrangement.spacedBy(3.dp)) {
            legend.forEach { stop ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(
                        modifier = Modifier
                            .size(10.dp)
                            .background(Color(android.graphics.Color.parseColor(stop.color)), RoundedCornerShape(2.dp)),
                    )
                    Text(
                        stringResource(legendLabelRes(stop.labelKey)),
                        style = MaterialTheme.typography.labelSmall,
                        color = Color(0xFF1F2B3A),
                    )
                }
            }
        }
    }
}

@Composable
private fun SandboxNotice(modifier: Modifier = Modifier) {
    BahariCard(modifier = modifier) {
        Text(stringResource(R.string.satellite_sandbox_title), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(6.dp))
        Text(
            stringResource(R.string.satellite_sandbox_body),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(8.dp))
        StatusChip(
            text = stringResource(R.string.satellite_adapter_sandbox),
            tone = ChipTone.WARNING,
        )
    }
}

@Composable
private fun LastPassCard(pass: SatellitePassDto, modifier: Modifier = Modifier) {
    BahariCard(modifier = modifier) {
        Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Default.Satellite, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(18.dp))
            Spacer(Modifier.size(8.dp))
            Text(stringResource(R.string.satellite_last_pass_title), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
            StatusChip(text = pass.adapterMode ?: "sandbox", tone = stateTone(pass.adapterMode ?: "sandbox"))
        }
        Spacer(Modifier.height(8.dp))
        Text(
            pass.sceneId ?: pass.id,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.Medium,
        )
        Spacer(Modifier.height(4.dp))
        Text(
            stringResource(
                R.string.satellite_last_pass_meta,
                relativeAge(pass.acquiredAt),
                pass.cloudCoverPct?.toInt()?.toString() ?: "–",
            ),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        pass.acquiredAt?.let {
            Text(
                formatAcquiredDate(it),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (!pass.quality.isNullOrBlank()) {
            Spacer(Modifier.height(6.dp))
            StatusChip(text = pass.quality, tone = stateTone(pass.quality))
        }
    }
}

@Composable
private fun ChangeCard(change: NdviChangeDto, modifier: Modifier = Modifier) {
    BahariCard(modifier = modifier) {
        Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.satellite_change_title), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
            StatusChip(text = change.quality, tone = stateTone(change.quality))
        }
        Spacer(Modifier.height(8.dp))
        change.meanNdvi?.let {
            ChangeRow(stringResource(R.string.satellite_change_mean_ndvi), formatNdvi(it))
        }
        change.baselineMeanNdvi?.let {
            ChangeRow(stringResource(R.string.satellite_change_baseline), formatNdvi(it))
        }
        if (change.changeAbs != null) {
            val sign = if (change.changeAbs >= 0) "+" else ""
            ChangeRow(stringResource(R.string.satellite_change_delta), "$sign${formatNdvi(change.changeAbs)}")
        }
        ChangeRow(
            stringResource(R.string.satellite_change_coverage),
            stringResource(R.string.satellite_change_coverage_value, change.coveragePct.toInt(), change.plotCount),
        )
        if (change.coveragePct < 60.0 && change.plotCount > 0) {
            Spacer(Modifier.height(8.dp))
            InlineNotice(
                title = stringResource(R.string.satellite_partial_title),
                body = stringResource(R.string.satellite_partial_body),
            )
        }
    }
}

@Composable
private fun ChangeRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
    }
}

@Composable
private fun InlineNotice(title: String, body: String, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = MaterialTheme.colorScheme.tertiaryContainer,
        contentColor = MaterialTheme.colorScheme.onTertiaryContainer,
        shape = RoundedCornerShape(8.dp),
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Text(title, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
            Text(body, style = MaterialTheme.typography.bodySmall)
        }
    }
}

@Composable
private fun AcquisitionRow(pass: SatellitePassDto, modifier: Modifier = Modifier) {
    BahariCard(modifier = modifier) {
        Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(pass.sceneId ?: pass.id, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            pass.cloudCoverPct?.let {
                Text(
                    stringResource(R.string.satellite_cloud_label, it.toInt()),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Spacer(Modifier.size(8.dp))
            StatusChip(text = pass.adapterMode ?: "sandbox", tone = stateTone(pass.adapterMode ?: "sandbox"))
        }
        pass.acquiredAt?.let {
            Text(
                formatAcquiredDate(it),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun RefreshAction(
    refreshJob: SatelliteRefreshDto?,
    refreshError: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val inFlight = refreshJob != null && refreshJob.status != "complete" &&
        refreshJob.status != "failed" && refreshJob.status != "cancelled"
    Column(modifier = modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        BahariButton(
            onClick = onClick,
            variant = BahariButtonVariant.OUTLINED,
            modifier = Modifier.fillMaxWidth(),
            enabled = !inFlight,
        ) {
            Icon(Icons.Default.Refresh, null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.size(8.dp))
            Text(
                if (inFlight) stringResource(R.string.satellite_refresh_running)
                else stringResource(R.string.satellite_refresh_action),
            )
        }
        refreshError?.let {
            Text(
                it,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
    }
}

private fun formatNdvi(value: Double): String = "%.3f".format(value)

private fun ndviBandLabel(ndvi: Double?): String = when (ndvi) {
    null -> "NDVI unavailable"
    in 0.6..1.0 -> "NDVI dense vegetation"
    in 0.45..0.6 -> "NDVI moderate vegetation"
    in 0.3..0.45 -> "NDVI low vegetation"
    in 0.1..0.3 -> "NDVI very low cover"
    else -> "NDVI bare surface"
}

private fun formatAcquiredDate(value: String): String {
    val parsed = runCatching {
        OffsetDateTime.parse(value).toInstant().atZone(ZoneId.systemDefault())
            .format(DateTimeFormatter.ofPattern("d MMM yyyy • HH:mm"))
    }.getOrNull()
    return parsed ?: value
}

private fun relativeAge(value: String?): String {
    if (value.isNullOrBlank()) return "timestamp unavailable"
    val instant = runCatching { OffsetDateTime.parse(value).toInstant() }.getOrNull() ?: return value
    val now = Instant.now()
    val minutes = ChronoUnit.MINUTES.between(instant, now)
    return when {
        minutes < 1 -> "just now"
        minutes < 60 -> "$minutes min ago"
        minutes < 60 * 24 -> "${minutes / 60} h ago"
        else -> "${minutes / (60 * 24)} d ago"
    }
}

private fun legendLabelRes(labelKey: String): Int = when (labelKey) {
    "satellite.legend.bare" -> R.string.satellite_legend_bare
    "satellite.legend.verylow" -> R.string.satellite_legend_very_low
    "satellite.legend.low" -> R.string.satellite_legend_low
    "satellite.legend.moderate" -> R.string.satellite_legend_moderate
    "satellite.legend.dense" -> R.string.satellite_legend_dense
    else -> R.string.satellite_legend_generic
}
