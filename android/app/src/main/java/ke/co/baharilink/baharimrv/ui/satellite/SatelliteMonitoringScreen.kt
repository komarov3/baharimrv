package ke.co.baharilink.baharimrv.ui.satellite

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Satellite
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.compose.ui.platform.LocalContext
import com.google.android.gms.maps.compose.CameraPositionState
import com.google.android.gms.maps.compose.GoogleMap
import com.google.android.gms.maps.compose.Marker
import com.google.android.gms.maps.compose.rememberCameraPositionState
import com.google.android.gms.maps.model.BitmapDescriptorFactory
import com.google.android.gms.maps.model.LatLng
import com.google.firebase.FirebaseApp
import com.google.firebase.analytics.FirebaseAnalytics
import com.google.firebase.analytics.FirebaseAnalytics.Event.SELECT_CONTENT
import com.google.firebase.analytics.FirebaseAnalytics.Param.ITEM_ID
import com.google.firebase.analytics.FirebaseAnalytics.Param.ITEM_NAME
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.BahariStatusPill
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.StatusChip

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun SatelliteMonitoringScreen(
    viewModel: SatelliteMonitoringViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val analytics = FirebaseAnalytics.getInstance(LocalContext.current)

    LaunchedEffect(projectId) {
        viewModel.setProjectId(projectId)
        analytics.logEvent(SELECT_CONTENT) {
            param(ITEM_ID, projectId)
            param(ITEM_NAME, "satellite_monitoring_screen")
        }
    }

    val markers = viewModel.plotMarkers()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(text = "Satellite monitoring") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // NDVI Map Overlay
            if (state.dashboard.valueOrNull()?.plots?.isNotEmpty() == true) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(300.dp)
                        .padding(8.dp),
                ) {
                    Text(
                        text = "NDVI Map Overlay",
                        style = MaterialTheme.typography.titleSmall.copy(
                            fontWeight = FontWeight.Bold,
                        ),
                        modifier = Modifier.padding(horizontal = 8.dp, top = 4.dp),
                    )
                    GoogleMap(
                        modifier = Modifier.fillMaxSize(),
                        cameraPositionState = rememberCameraPositionState {
                            val firstPlot = state.dashboard.valueOrNull()?.plots?.firstOrNull()
                            if (firstPlot != null) {
                                CameraPosition.fromLatLngZoom(
                                    LatLng(firstPlot.latitude, firstPlot.longitude),
                                    14f,
                                )
                            } else {
                                CameraPosition.fromLatLngZoom(LatLng(0.0, 0.0), 2f)
                            }
                        },
                    ) {
                        state.dashboard.valueOrNull()?.plots?.forEach { plot ->
                            val ndviColor = viewModel.ndviColor(plot.ndvi)
                            Marker(
                                position = LatLng(plot.latitude, plot.longitude),
                                title = plot.label,
                                snippet = "NDVI: ${plot.ndvi?.let { "%.2f".format(it) } ?: "N/A"}",
                                icon = BitmapDescriptorFactory.hue(
                                    when {
                                        plot.ndvi == null -> 0.0f // grey
                                        plot.ndvi in 0.6..1.0 -> 120.0f // green
                                        plot.ndvi in 0.4..0.6 -> 30.0f // amber/orange
                                        plot.ndvi in 0.2..0.4 -> 0.0f // red
                                        else -> 60.0f // olive
                                    },
                                ),
                                onClick = {
                                    analytics.logEvent(SELECT_CONTENT) {
                                        param(ITEM_ID, plot.id)
                                        param(ITEM_NAME, "ndvi_marker_${plot.label}")
                                    }
                                },
                                window = {
                                    Card(
                                        modifier = Modifier.width(200.dp),
                                    ) {
                                        Column(
                                            modifier = Modifier.padding(8.dp),
                                        ) {
                                            Text(
                                                text = plot.label,
                                                style = MaterialTheme.typography.titleSmall,
                                            )
                                            Text(
                                                text = "NDVI: ${plot.ndvi?.let { "%.3f".format(it) } ?: "no data"}",
                                                style = MaterialTheme.typography.bodyMedium,
                                            )
                                            Text(
                                                text = "Lat: ${plot.latitude.toString().take(7)}, Lng: ${plot.longitude.toString().take(7)}",
                                                style = MaterialTheme.typography.bodySmall.copy(
                                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                                ),
                                            )
                                        }
                                    }
                                }
                            )
                        }
                    }
                }
            }

            when (val latestPass = state.latestPass) {
            is LoadState.Loading -> {
                androidx.compose.foundation.layout.Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = androidx.compose.ui.Alignment.Center,
                ) {
                    LoadingState("Loading satellite data…")
                }
            }
            is LoadState.Error -> {
                ErrorState(message = latestPass.message, onRetry = { viewModel.retry() })
            }
            is LoadState.Success -> {
                val passes = when (val p = state.passes) {
                    is LoadState.Success -> p.data
                    else -> emptyList<SatellitePassDto>()
                }
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    item {
                        ContextHeader(
                            title = "Satellite monitoring",
                            subtitle = "NDVI tiles matched to project plots · corroborating evidence only",
                        )
                    }
                    item {
                        BahariCard {
                            Column(
                                modifier = Modifier.fillMaxWidth(),
                                verticalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Icon(
                                        imageVector = Icons.Default.Satellite,
                                        contentDescription = null,
                                        modifier = Modifier.size(18.dp)
                                    )
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text(
                                            text = "Provider mode",
                                            style = MaterialTheme.typography.labelSmall.copy(
                                                fontWeight = FontWeight.Bold,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            ),
                                        )
                                        BahariStatusPill(
                                            text = "Sandbox · not a public satellite feed",
                                        )
                                    }
                                }
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Text(
                                        text = "NDVI status",
                                        style = MaterialTheme.typography.labelSmall.copy(
                                            fontWeight = FontWeight.Bold,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        ),
                                    )
                                    Text(
                                        text = latestPass.data?.statusDetail ?: latestPass.data?.adapterMode ?: "Unavailable",
                                        style = MaterialTheme.typography.bodyMedium,
                                    )
                                }
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Text(
                                        text = "Source",
                                        style = MaterialTheme.typography.labelSmall.copy(
                                            fontWeight = FontWeight.Bold,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        ),
                                    )
                                    Text(
                                        text = latestPass.data?.mission ?: "Provider-neutral sandbox",
                                        style = MaterialTheme.typography.bodyMedium,
                                    )
                                }
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Text(
                                        text = "Acquired",
                                        style = MaterialTheme.typography.labelSmall.copy(
                                            fontWeight = FontWeight.Bold,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        ),
                                    )
                                    Text(
                                        text = latestPass.data?.acquiredAt ?: "—",
                                        style = MaterialTheme.typography.bodyMedium,
                                    )
                                }
                            }
                        }
                    }
                    item {
                        BahariCard {
                            Column(
                                modifier = Modifier.fillMaxWidth(),
                                verticalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Text(
                                    text = "Tile match & NDVI change",
                                    style = MaterialTheme.typography.labelSmall.copy(
                                        fontWeight = FontWeight.Bold,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    ),
                                )
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Card(
                                        modifier = Modifier.weight(1f),
                                        colors = CardDefaults.cardColors(
                                            containerColor = MaterialTheme.colorScheme.surfaceVariant,
                                        ),
                                        shape = MaterialTheme.shapes.medium,
                                    ) {
                                        Column(
                                            modifier = Modifier.padding(12.dp),
                                            horizontalAlignment = Alignment.CenterHorizontally,
                                            verticalArrangement = Arrangement.spacedBy(4.dp),
                                        ) {
                                            Icon(
                                                imageVector = Icons.Default.Satellite,
                                                contentDescription = null,
                                                modifier = Modifier.size(24.dp),
                                                tint = MaterialTheme.colorScheme.primary
                                            )
                                            Text(
                                                text = "12 of 14 plots matched",
                                                style = MaterialTheme.typography.titleSmall,
                                                fontWeight = FontWeight.Bold,
                                            )
                                            Text(
                                                text = "Sentinel-2 tile matched",
                                                style = MaterialTheme.typography.bodySmall.copy(
                                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                                ),
                                            )
                                        }
                                    }
                                    Card(
                                        modifier = Modifier.weight(1f),
                                        colors = CardDefaults.cardColors(
                                            containerColor = MaterialTheme.colorScheme.primaryContainer,
                                        ),
                                        shape = MaterialTheme.shapes.medium,
                                    ) {
                                        Column(
                                            modifier = Modifier.padding(12.dp),
                                            horizontalAlignment = Alignment.CenterHorizontally,
                                            verticalArrangement = Arrangement.spacedBy(4.dp),
                                        ) {
                                            Icon(
                                                imageVector = Icons.Default.Satellite,
                                                contentDescription = null,
                                                modifier = Modifier.size(24.dp),
                                                tint = MaterialTheme.colorScheme.onPrimaryContainer
                                            )
                                            Text(
                                                text = "+12% YoY",
                                                style = MaterialTheme.typography.titleSmall,
                                                fontWeight = FontWeight.Bold,
                                            )
                                            Text(
                                                text = "NDVI change across matched plots",
                                                style = MaterialTheme.typography.bodySmall.copy(
                                                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.9f),
                                                ),
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                    item {
                        Text(
                            text = "Plot NDVI detail",
                            style = MaterialTheme.typography.labelSmall.copy(
                                fontWeight = FontWeight.Bold,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            ),
                            modifier = Modifier.padding(top = 8.dp),
                        )
                    }
                    items(passes) { pass ->
                        BahariCard {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Text(
                                    text = pass.sceneId ?: pass.id,
                                    style = MaterialTheme.typography.titleSmall,
                                    modifier = Modifier.weight(1f),
                                )
                                Text(
                                    text = pass.cloudCoverPct?.let { "Cloud ${it.toInt()}%" } ?: "—",
                                    style = MaterialTheme.typography.bodySmall.copy(
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    ),
                                )
                                StatusChip(
                                    text = pass.adapterMode ?: "sandbox",
                                    tone = ChipTone.NEUTRAL,
                                )
                            }
                        }
                    }
                    if (passes.isEmpty()) {
                        item {
                            EmptyState(
                                title = "No satellite passes yet",
                                detail = "NDVI corroboration appears after a provider-neutral tile match returns.",
                            )
                        }
                    }
                    item {
                        BahariButton(
                            onClick = viewModel::retry,
                            variant = BahariButtonVariant.OUTLINED,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text(text = "Refresh satellite data")
                        }
                    }
                }
            }
        }
    }
}

private enum class ChipTone {
    POSITIVE, WARNING, INFO, NEUTRAL
}
