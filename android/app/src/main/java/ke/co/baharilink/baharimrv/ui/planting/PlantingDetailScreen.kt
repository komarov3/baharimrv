package ke.co.baharilink.baharimrv.ui.planting

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.ActivityDetailDto
import ke.co.baharilink.baharimrv.ui.components.*

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlantingDetailScreen(
    activityId: String,
    onBack: () -> Unit,
    viewModel: PlantingDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

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

    var showFullScreenPhoto by remember { mutableStateOf(false) }
    val photoBitmap = (state.image as? LoadState.Success)?.data

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Activity report") },
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
                null, is LoadState.Loading -> Column(
                    modifier = Modifier.fillMaxSize().padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    repeat(3) { SkeletonCard() }
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
    val measurements = dto.measurements.orEmpty()
    val species = measurements["species"]?.takeIf { it.isNotBlank() }
        ?: dto.record?.species?.takeIf { it.isNotBlank() }
    val plantedBaseline = measurements["seedlingsPlanted"]?.takeIf { it.isNotBlank() }
        ?: dto.record?.plantedCount?.takeIf { it > 0 }?.toString()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Hero header
        item {
            BahariCard(highlighted = dto.status.equals("open", ignoreCase = true)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        modifier = Modifier
                            .size(52.dp)
                            .background(
                                color = when {
                                    dto.status.equals("complete", ignoreCase = true) -> MaterialTheme.colorScheme.primaryContainer
                                    dto.status.equals("open", ignoreCase = true) -> MaterialTheme.colorScheme.tertiaryContainer
                                    else -> MaterialTheme.colorScheme.surfaceVariant
                                },
                                shape = MaterialTheme.shapes.medium,
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            Icons.Default.Forest,
                            contentDescription = null,
                            tint = when {
                                dto.status.equals("complete", ignoreCase = true) -> MaterialTheme.colorScheme.onPrimaryContainer
                                dto.status.equals("open", ignoreCase = true) -> MaterialTheme.colorScheme.onTertiaryContainer
                                else -> MaterialTheme.colorScheme.onSurfaceVariant
                            },
                            modifier = Modifier.size(28.dp),
                        )
                    }
                    Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            text = dto.activityType.replaceFirstChar { it.uppercase() },
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Bold,
                        )
                        Text(
                            text = "Plot ${dto.plotId}",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        StatusChip(
                            text = dto.status.replaceFirstChar { it.uppercase() },
                            tone = when {
                                dto.status.equals("complete", ignoreCase = true) -> ChipTone.POSITIVE
                                dto.status.equals("open", ignoreCase = true) -> ChipTone.WARNING
                                dto.status.equals("due", ignoreCase = true) -> ChipTone.ERROR
                                else -> ChipTone.NEUTRAL
                            },
                        )
                    }
                }
            }
        }

        // Core details — the immutable summary of everything collected at planting.
        item {
            ReportSection(title = "Activity details", icon = Icons.Default.Info) {
                KeyValue(label = "Activity ID", value = dto.id, monospace = true)
                KeyValue(label = "Project ID", value = dto.projectId, monospace = true)
                KeyValue(label = "Plot ID", value = dto.plotId, monospace = true)
                KeyValue(label = "Activity type", value = dto.activityType.replaceFirstChar { it.uppercase() })
                KeyValue(label = "Status", value = dto.status.replaceFirstChar { it.uppercase() })
                KeyValue(label = "Species", value = species ?: "Not recorded")
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
                KeyValue(label = "Seedlings planted", value = plantedBaseline ?: "Not recorded yet")
            }
        }

        // Optional conditions & team — every remaining measurement from the form.
        item {
            ReportSection(title = "Planting conditions & team", icon = Icons.Default.Cloud) {
                val optional = measurements.filterKeys { it != "species" && it != "seedlingsPlanted" }
                if (optional.values.all { it.isNullOrBlank() }) {
                    Text(
                        "No optional details recorded for this activity.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                } else {
                    val labels = mapOf(
                        "weather" to "Weather",
                        "tide" to "Tide conditions",
                        "soil" to "Soil / substrate",
                        "notes" to "Additional notes",
                        "teamLead" to "Team lead",
                        "teamSize" to "Team size",
                        "volunteers" to "Volunteers / community",
                        "photoCount" to "Photos captured",
                    )
                    optional.toSortedMap().forEach { (key, value) ->
                        val text = value?.trim().orEmpty()
                        if (text.isNotBlank()) {
                            KeyValue(label = labels[key] ?: key.replaceFirstChar { it.uppercase() }, value = text)
                        }
                    }
                }
            }
        }

        // Monitoring record
        item {
            val record = dto.record
            if (record == null) {
                EmptyState(
                    title = "No monitoring data",
                    detail = "No records have been synchronised for this activity yet.",
                )
            } else {
                ReportSection(title = "Monitoring record", icon = Icons.Default.Assessment) {
                    KeyValue(label = "Alive count", value = record.aliveCount.toString())
                    KeyValue(label = "Planted count", value = record.plantedCount.toString())
                    KeyValue(label = "Average height (m)", value = record.averageHeightMetres.ifBlank { "—" })
                    KeyValue(label = "Species", value = record.species.ifBlank { "Unspecified" })
                    KeyValue(label = "Methodology version", value = record.methodologyVersion.ifBlank { "—" })
                    KeyValue(label = "Device captured at", value = record.deviceTimestamp?.takeIf { it.isNotBlank() } ?: "—")
                    KeyValue(label = "Server receipt at", value = record.serverReceiptTime?.takeIf { it.isNotBlank() } ?: "—")
                    KeyValue(
                        label = "Observations",
                        value = record.observations?.takeIf { it.isNotBlank() } ?: "No observations",
                    )
                }
            }
        }

        // Field location
        item {
            val record = dto.record
            ReportSection(title = "Field location", icon = Icons.Default.Place) {
                if (record != null) {
                    KeyValue(label = "Latitude", value = record.latitude.toString())
                    KeyValue(label = "Longitude", value = record.longitude.toString())
                    KeyValue(label = "GPS accuracy", value = "${record.gpsAccuracyMeters} m")
                    val context = LocalContext.current
                    BahariButton(
                        onClick = {
                            val uri = Uri.parse("geo:${record.latitude},${record.longitude}?q=${record.latitude},${record.longitude}(Planting activity)")
                            context.startActivity(Intent(Intent.ACTION_VIEW, uri))
                        },
                        variant = BahariButtonVariant.OUTLINED,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text("Open in maps")
                    }
                } else {
                    Text(
                        "No GPS location recorded yet.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        // Evidence photo
        item {
            val evidence = dto.record?.evidence.orEmpty()
            if (evidence.isNotEmpty()) {
                val first = evidence.first()
                ReportSection(title = "Evidence", icon = Icons.Default.Image, onClick = onPhotoClick) {
                    BahariDashedBox(modifier = Modifier.fillMaxWidth().height(220.dp)) {
                        when (imageState) {
                            is LoadState.Success -> Image(
                                bitmap = imageState.data,
                                contentDescription = "Evidence photo",
                                modifier = Modifier.fillMaxSize().clip(MaterialTheme.shapes.small),
                                contentScale = ContentScale.Crop,
                            )
                            is LoadState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
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
                ReportSection(title = "Evidence", icon = Icons.Default.Image) {
                    Text(
                        "No evidence attached",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        // On-chain anchor
        item {
            val provenance = dto.provenance
            NavyLedgerCard {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Icon(Icons.Default.Link, contentDescription = null, tint = Color(0xFF6EE7B7), modifier = Modifier.size(18.dp))
                    Text("On-chain anchor", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                }
                if (provenance != null) {
                    KeyValue(label = "Status", value = provenance.status, valueColor = statusColor(provenance.status))
                    KeyValue(label = "Network", value = chainNetworkLabel(provenance.network))
                    provenance.objectId?.let { KeyValue(label = "Object ID", value = it, monospace = true) }
                    provenance.explorerUrl?.let { explorerUrl ->
                        val context = LocalContext.current
                        BahariButton(
                            onClick = { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(explorerUrl))) },
                            variant = BahariButtonVariant.GREEN,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text("View on IOTA ↗")
                        }
                    }
                } else {
                    Text("Not yet anchored on-chain", style = MaterialTheme.typography.bodyMedium)
                }
            }
        }

        // Attestation
        item {
            val attestation = dto.record?.attestationStatement
            ReportSection(title = "Attestation", icon = Icons.Default.Verified) {
                if (!attestation.isNullOrBlank()) {
                    Text(attestation, style = MaterialTheme.typography.bodyMedium)
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

@Composable
private fun ReportSection(
    title: String,
    icon: ImageVector,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    BahariCard(onClick = onClick) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(18.dp))
            Text(title, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
        }
        content()
    }
}

private fun statusColor(status: String): Color = when (status.lowercase()) {
    "anchored" -> Color(0xFF6EE7B7)
    "pending" -> Color(0xFFFCD34D)
    "failed" -> Color(0xFFFCA5A5)
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
                    contentDescription = "Evidence photo unavailable",
                    modifier = Modifier.size(64.dp),
                    tint = Color.White,
                )
            }
            IconButton(
                onClick = onDismiss,
                modifier = Modifier.align(Alignment.TopEnd).padding(16.dp),
            ) {
                Icon(Icons.Default.Close, contentDescription = "Close photo", tint = Color.White)
            }
        }
    }
}