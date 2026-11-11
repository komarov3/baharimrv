package ke.co.baharilink.baharimrv.ui.planting

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Image
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.ActivityDetailDto
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.BahariDashedBox
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.KeyValue
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.NavyLedgerCard
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlantingDetailScreen(
    activityId: String,
    onBack: () -> Unit,
    viewModel: PlantingDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    // Trigger load exactly once on entry
    LaunchedEffect(activityId) { viewModel.load(activityId) }

    // Once the detail resolves with at least one evidence object, fetch and decode
    // its bytes so the thumbnail and full-screen overlay render the real photo.
    LaunchedEffect(state.detail) {
        val detail = state.detail
        if (detail is LoadState.Success) {
            val evidenceId = detail.data.record?.evidence?.firstOrNull()?.id
            if (!evidenceId.isNullOrBlank()) viewModel.loadImage(evidenceId)
        }
    }

    // Full-screen photo overlay — pure navigation state, NOT in ViewModel
    var showFullScreenPhoto by remember { mutableStateOf(false) }
    val photoBitmap = (state.image as? LoadState.Success)?.data

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Activity detail") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { padding ->
        Box(modifier = Modifier.fillMaxSize().padding(padding)) {
            when (val detail = state.detail) {
                null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    LoadingState("Preparing…")
                }
                is LoadState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    LoadingState("Loading activity detail…")
                }
                is LoadState.Error -> ErrorState(
                    message = detail.message,
                    onRetry = { viewModel.load(activityId) },
                )
                is LoadState.Success -> ActivityDetailContent(
                    dto = detail.data,
                    imageState = state.image,
                    onPhotoClick = { showFullScreenPhoto = true },
                )
            }
            if (showFullScreenPhoto) {
                FullScreenPhotoOverlay(bitmap = photoBitmap, onDismiss = { showFullScreenPhoto = false })
            }
        }
    }
}

@Composable
private fun ActivityDetailContent(
    dto: ActivityDetailDto,
    imageState: LoadState<ImageBitmap>?,
    onPhotoClick: () -> Unit,
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // 1. Context header
        item {
            ContextHeader(
                title = dto.activityType.replaceFirstChar { it.uppercase() },
                subtitle = "${dto.plotId} · ${dto.status}",
            )
        }

        // 1b. Activity details card (immutable summary of the collected form)
        item {
            BahariCard {
                Text(
                    "Activity details",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
                KeyValue(label = "Activity ID", value = dto.id, monospace = true)
                KeyValue(label = "Project ID", value = dto.projectId, monospace = true)
                KeyValue(label = "Plot ID", value = dto.plotId, monospace = true)
                KeyValue(label = "Activity type", value = dto.activityType.replaceFirstChar { it.uppercase() })
                KeyValue(label = "Status", value = dto.status.replaceFirstChar { it.uppercase() })
                KeyValue(
                    label = "Methodology version",
                    value = dto.methodologyVersion?.takeIf { it.isNotBlank() } ?: "—",
                )
                KeyValue(
                    label = "Reporting period",
                    value = when {
                        dto.periodStart.isNullOrBlank() && dto.periodEnd.isNullOrBlank() -> "—"
                        dto.periodStart.isNullOrBlank() -> dto.periodEnd!!
                        dto.periodEnd.isNullOrBlank() -> dto.periodStart!!
                        else -> "${dto.periodStart} → ${dto.periodEnd}"
                    },
                )
                KeyValue(
                    label = "Seedlings planted",
                    value = dto.record?.plantedCount?.toString() ?: "Not recorded yet",
                )
            }
        }

        // 2. Monitoring record card
        item {
            val record = dto.record
            if (record == null) {
                EmptyState(
                    title = "No monitoring data",
                    detail = "No records have been synchronised for this activity yet.",
                )
            } else {
                BahariCard {
                    Text(
                        "Monitoring record",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                    )
                    KeyValue(label = "Alive count", value = record.aliveCount.toString())
                    KeyValue(label = "Planted count", value = record.plantedCount.toString())
                    KeyValue(label = "Average height (m)", value = record.averageHeightMetres.ifBlank { "—" })
                    KeyValue(label = "Species", value = record.species.ifBlank { "Unspecified" })
                    KeyValue(label = "Methodology version", value = record.methodologyVersion.ifBlank { "—" })
                    KeyValue(label = "GPS latitude", value = record.latitude.toString())
                    KeyValue(label = "GPS longitude", value = record.longitude.toString())
                    KeyValue(label = "GPS accuracy (m)", value = record.gpsAccuracyMeters.toString())
                    KeyValue(
                        label = "Device captured at",
                        value = record.deviceTimestamp?.takeIf { it.isNotBlank() } ?: "—",
                    )
                    KeyValue(
                        label = "Server receipt at",
                        value = record.serverReceiptTime?.takeIf { it.isNotBlank() } ?: "—",
                    )
                    KeyValue(
                        label = "Observations",
                        // The backend stores observations only in the content commitment,
                        // not as a column, so this is empty in practice. Treat blank as none.
                        value = record.observations?.takeIf { it.isNotBlank() } ?: "No observations",
                    )
                }
            }
        }

        // 3. Evidence photo card
        item {
            val evidence = dto.record?.evidence.orEmpty()
            if (evidence.isNotEmpty()) {
                val first = evidence.first()
                BahariCard(onClick = onPhotoClick) {
                    Text(
                        "Evidence",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                    )
                    BahariDashedBox(
                        modifier = Modifier.fillMaxWidth().height(120.dp),
                    ) {
                        when (imageState) {
                            is LoadState.Success -> Image(
                                bitmap = imageState.data,
                                contentDescription = "Evidence photo",
                                modifier = Modifier.fillMaxSize(),
                                contentScale = ContentScale.Crop,
                            )
                            is LoadState.Loading -> Box(
                                Modifier.fillMaxSize(),
                                contentAlignment = Alignment.Center,
                            ) {
                                CircularProgressIndicator(modifier = Modifier.size(28.dp))
                            }
                            is LoadState.Error -> EvidencePlaceholder(sha256 = first.sha256)
                            null -> EvidencePlaceholder(sha256 = first.sha256)
                        }
                    }
                    Text(
                        "Tap the photo to view it full screen.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            } else {
                BahariCard {
                    Text(
                        "Evidence",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        "No evidence attached",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        // 4. On-chain anchor card
        item {
            val provenance = dto.provenance
            NavyLedgerCard {
                Text(
                    "On-chain anchor",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
                if (provenance != null) {
                    KeyValue(
                        label = "Status",
                        value = provenance.status,
                        valueColor = statusColor(provenance.status),
                    )
                    KeyValue(label = "Network", value = chainNetworkLabel(provenance.network))
                    provenance.objectId?.let { KeyValue(label = "Object ID", value = it, monospace = true) }
                    provenance.explorerUrl?.let { explorerUrl ->
                        val context = LocalContext.current
                        BahariButton(
                            onClick = {
                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(explorerUrl)))
                            },
                            variant = BahariButtonVariant.GREEN,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text(text = "View on IOTA ↗")
                        }
                    }
                } else {
                    Text(
                        "Not yet anchored on-chain",
                        style = MaterialTheme.typography.bodyMedium,
                    )
                }
            }
        }

        // 5. Attestation card
        item {
            val attestation = dto.record?.attestationStatement
            BahariCard {
                Text(
                    "Attestation",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
                if (!attestation.isNullOrBlank()) {
                    Text(
                        attestation,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                } else {
                    Text(
                        "No attestation recorded",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
    }
}

private fun statusColor(status: String): Color = when (status.lowercase()) {
    "anchored" -> Color(0xFF059669)
    "pending" -> Color(0xFFD97706)
    "failed" -> Color(0xFFDC2626)
    else -> Color.Unspecified
}

@Composable
private fun EvidencePlaceholder(sha256: String) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Icon(
            Icons.Default.Image,
            contentDescription = null,
            modifier = Modifier.size(36.dp),
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            text = sha256.take(8),
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun FullScreenPhotoOverlay(bitmap: ImageBitmap?, onDismiss: () -> Unit) {
    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(Color.Black)
                .clickable { onDismiss() },
            contentAlignment = Alignment.Center,
        ) {
            if (bitmap != null) {
                Image(
                    bitmap = bitmap,
                    contentDescription = "Evidence photo",
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Fit,
                )
            } else {
                Icon(
                    Icons.Default.Image,
                    contentDescription = "Close",
                    modifier = Modifier.size(64.dp),
                    tint = Color.White,
                )
            }
        }
    }
}
