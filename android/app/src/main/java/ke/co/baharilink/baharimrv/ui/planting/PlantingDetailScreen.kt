package ke.co.baharilink.baharimrv.ui.planting

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
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

    val currentDetail = (state.detail as? LoadState.Success)?.data?.takeIf { it.id == activityId }
    val photoRefs = remember(activityId, currentDetail?.activityEvidence, currentDetail?.record?.evidence) {
        currentDetail?.activityEvidence.orEmpty()
            .filter { it.id.isNotBlank() && isPhoto(it.kind, it.mediaType) }
            .map { PlantingPhotoRef(it.id, activityId) } +
            currentDetail?.record?.evidence.orEmpty()
                .filter { it.id.isNotBlank() && isPhoto(it.kind, it.mediaType) }
                .map { PlantingPhotoRef(it.id) }
    }
    var selectedPhoto by remember(activityId) { mutableStateOf<PlantingPhotoRef?>(null) }
    val activePhoto = selectedPhoto?.takeIf { it in photoRefs } ?: photoRefs.firstOrNull()

    // A title change must not reset the selection or re-fetch the image.
    LaunchedEffect(activityId, activePhoto) {
        selectedPhoto = activePhoto
        if (activePhoto == null) viewModel.clearImage()
        else viewModel.loadImage(activePhoto.evidenceId, activePhoto.activityId)
    }
    DisposableEffect(activityId, viewModel) {
        onDispose { viewModel.clearImage() }
    }

    var renaming by remember { mutableStateOf(false) }
    var name by remember { mutableStateOf("") }
    if (renaming) {
        AlertDialog(
            onDismissRequest = { if (!state.renaming) renaming = false },
            title = { Text("Rename activity") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        value = name, onValueChange = { name = it },
                        label = { Text("Activity name (optional)") },
                        supportingText = { Text("Up to 120 characters. Clear to use the activity type.") },
                        singleLine = true, enabled = !state.renaming,
                        isError = name.codePointCount(0, name.length) > 120,
                    )
                    state.renameError?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !state.renaming && name.codePointCount(0, name.length) <= 120,
                    onClick = { viewModel.rename(activityId, name) { renaming = false } },
                ) { Text(if (state.renaming) "Saving…" else "Save") }
            },
            dismissButton = { TextButton(onClick = { renaming = false }, enabled = !state.renaming) { Text("Cancel") } },
        )
    }
    var showFullScreenPhoto by remember(activityId, activePhoto) { mutableStateOf(false) }
    val imageState = state.image.takeIf { activePhoto != null && state.imageRef == activePhoto }
    val photoBitmap = (imageState as? LoadState.Success)?.data

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Activity report") },
                actions = {
                    if (currentDetail?.canManage == true) {
                        TextButton(onClick = { name = currentDetail.title.orEmpty(); renaming = true }) { Text("Rename") }
                    }
                },
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
                    selectedPhoto = activePhoto,
                    imageState = imageState,
                    onSelectPhoto = { selectedPhoto = it },
                    onRetryPhoto = {
                        activePhoto?.let { viewModel.loadImage(it.evidenceId, it.activityId) }
                    },
                    onPhotoClick = { if (photoBitmap != null) showFullScreenPhoto = true },
                )
            }
            if (showFullScreenPhoto && photoBitmap != null) {
                FullScreenPhotoOverlay(bitmap = photoBitmap, onDismiss = { showFullScreenPhoto = false })
            }
        }
    }
}

@Composable
private fun ActivityDetailContent(
    dto: ActivityDetailDto,
    selectedPhoto: PlantingPhotoRef?,
    imageState: LoadState<ImageBitmap>?,
    onSelectPhoto: (PlantingPhotoRef) -> Unit,
    onRetryPhoto: () -> Unit,
    onPhotoClick: () -> Unit,
) {
    val measurements = dto.measurements.orEmpty()
    val species = measurements["species"]?.takeIf { it.isNotBlank() }
        ?: dto.record?.species?.takeIf { it.isNotBlank() }
    val plantedBaseline = measurements["seedlingsPlanted"]?.takeIf { it.isNotBlank() }
        ?: dto.record?.plantedCount?.takeIf { it > 0 }?.toString()
    val activityPhotos = dto.activityEvidence.orEmpty().filter { it.id.isNotBlank() && isPhoto(it.kind, it.mediaType) }
    val monitoringEvidence = dto.record?.evidence.orEmpty()

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
                            text = dto.title?.takeIf { it.isNotBlank() } ?: dto.activityType.replaceFirstChar { it.uppercase() },
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

        if (dto.activityType == "planting") {
            item { ActivityNdviSection(activityId = dto.id, canRecapture = dto.canManage) }
        }

        // Optional conditions & team — every remaining measurement from the form.
        item {
            ReportSection(title = "Planting conditions & team", icon = Icons.Default.Cloud) {
                // Legacy photoCount counted selections, not successfully stored uploads.
                val optional = measurements.filterKeys { it !in setOf("species", "seedlingsPlanted", "photoCount") }
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

        // Manager uploads are separate from the monitoring record and its provenance.
        item {
            ReportSection(title = "Activity photos", icon = Icons.Default.Image) {
                Text(
                    "Manager uploads, not attested or anchored",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (activityPhotos.isEmpty()) {
                    Text("No activity photos attached", style = MaterialTheme.typography.bodyMedium)
                } else {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        itemsIndexed(activityPhotos, key = { _, photo -> photo.id }) { index, photo ->
                            val ref = PlantingPhotoRef(photo.id, dto.id)
                            FilterChip(
                                selected = selectedPhoto == ref,
                                onClick = { onSelectPhoto(ref) },
                                label = { Text("Photo ${index + 1}") },
                            )
                        }
                    }
                }
            }
        }

        item {
            ReportSection(title = "Monitoring evidence", icon = Icons.Default.Assessment) {
                if (monitoringEvidence.isEmpty()) {
                    Text("No monitoring evidence attached", style = MaterialTheme.typography.bodyMedium)
                } else {
                    monitoringEvidence.forEachIndexed { index, evidence ->
                        if (evidence.id.isNotBlank() && isPhoto(evidence.kind, evidence.mediaType)) {
                            val ref = PlantingPhotoRef(evidence.id)
                            FilterChip(
                                selected = selectedPhoto == ref,
                                onClick = { onSelectPhoto(ref) },
                                label = { Text("Evidence ${index + 1} · Photo") },
                            )
                        } else {
                            Text("Evidence ${index + 1} · ${evidence.kind}", style = MaterialTheme.typography.labelLarge)
                            KeyValue(label = "Media type", value = evidence.mediaType)
                            KeyValue(label = "SHA-256", value = evidence.sha256, monospace = true)
                            Text(
                                "Image preview unavailable for this evidence.",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                }
            }
        }

        // One viewer for the selected photo, regardless of which proxy supplies it.
        if (selectedPhoto != null) {
            item {
                ReportSection(title = "Selected photo", icon = Icons.Default.Image) {
                    if (selectedPhoto.activityId != null) {
                        activityPhotos.withIndex().firstOrNull { it.value.id == selectedPhoto.evidenceId }?.let { (index, photo) ->
                            Text("Activity photo ${index + 1}", style = MaterialTheme.typography.labelLarge)
                            KeyValue(label = "SHA-256", value = photo.sha256, monospace = true)
                            KeyValue(label = "Uploaded at", value = photo.createdAt.ifBlank { "Not recorded" })
                        }
                    } else {
                        monitoringEvidence.withIndex().firstOrNull { it.value.id == selectedPhoto.evidenceId }?.let { (index, evidence) ->
                            Text("Monitoring evidence ${index + 1}", style = MaterialTheme.typography.labelLarge)
                            KeyValue(label = "SHA-256", value = evidence.sha256, monospace = true)
                        }
                    }
                    EvidencePhotoViewer(
                        imageState = imageState,
                        onRetry = onRetryPhoto,
                        onPhotoClick = onPhotoClick,
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

private fun isPhoto(kind: String, mediaType: String): Boolean =
    kind.equals("photo", ignoreCase = true) && mediaType.startsWith("image/", ignoreCase = true)

@Composable
private fun EvidencePhotoViewer(
    imageState: LoadState<ImageBitmap>?,
    onRetry: () -> Unit,
    onPhotoClick: () -> Unit,
) {
    BahariDashedBox(
        modifier = Modifier.fillMaxWidth().height(220.dp)
            .clickable(enabled = imageState is LoadState.Success, onClick = onPhotoClick),
    ) {
        when (imageState) {
            is LoadState.Success -> Image(
                bitmap = imageState.data,
                contentDescription = "Selected evidence photo",
                modifier = Modifier.fillMaxSize().clip(MaterialTheme.shapes.small),
                contentScale = ContentScale.Crop,
            )
            null, is LoadState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(modifier = Modifier.size(28.dp))
            }
            is LoadState.Error -> Icon(
                Icons.Default.BrokenImage,
                contentDescription = "Photo unavailable",
                modifier = Modifier.size(36.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
    if (imageState is LoadState.Error) {
        Text(imageState.message, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
        TextButton(onClick = onRetry) { Text("Retry") }
    }
    TextButton(onClick = onPhotoClick, enabled = imageState is LoadState.Success) {
        Text("View full screen")
    }
}

@Composable
private fun FullScreenPhotoOverlay(bitmap: ImageBitmap, onDismiss: () -> Unit) {
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
            Image(
                bitmap = bitmap,
                contentDescription = "Selected evidence photo",
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Fit,
            )
            IconButton(
                onClick = onDismiss,
                modifier = Modifier.align(Alignment.TopEnd).padding(16.dp),
            ) {
                Icon(Icons.Default.Close, contentDescription = "Close photo", tint = Color.White)
            }
        }
    }
}