package ke.co.baharilink.baharimrv.ui.monitor

import android.Manifest
import android.content.ContentValues
import android.content.Context
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.MediaStore
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.LocationOn
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.PhotoCamera
import androidx.compose.material.icons.filled.PhotoLibrary
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import android.widget.Toast
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.local.EvidenceKind
import ke.co.baharilink.baharimrv.data.local.MonitoringWithEvidence
import ke.co.baharilink.baharimrv.data.repository.EvidenceSelection
import ke.co.baharilink.baharimrv.location.AudioRecorder
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.ExpandableSection
import ke.co.baharilink.baharimrv.ui.components.MicroLabel
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.dashedBorder
import ke.co.baharilink.baharimrv.ui.components.stateTone

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun MonitorScreen(viewModel: MonitorViewModel = hiltViewModel()) {
    val state    by viewModel.state.collectAsStateWithLifecycle()
    val context  = LocalContext.current

    LaunchedEffect(state.message, state.error) {
        state.message?.let { Toast.makeText(context, it, Toast.LENGTH_LONG).show() }
        state.error?.let { Toast.makeText(context, it, Toast.LENGTH_LONG).show() }
    }
    val recorder = remember { AudioRecorder(context) }
    var showPlotSheet  by remember { mutableStateOf(false) }
    var showPhotoMenu  by remember { mutableStateOf(false) }
    var cameraUri      by remember { mutableStateOf<Uri?>(null) }
    DisposableEffect(Unit) { onDispose { recorder.cancel() } }

    fun selection(uri: Uri, kind: EvidenceKind): EvidenceSelection = EvidenceSelection(
        uri = uri,
        kind = kind,
        originalFileName = context.displayName(uri) ?: "${kind.name.lowercase()}-${System.currentTimeMillis()}",
        mimeType = context.contentResolver.getType(uri) ?: if (kind == EvidenceKind.PHOTO) "image/*" else "audio/*",
        capturedAtEpochMs = System.currentTimeMillis(),
    )

    val galleryPicker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        uri?.let { viewModel.addEvidence(selection(it, EvidenceKind.PHOTO)) }
    }
    val cameraPicker = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { captured ->
        if (captured) cameraUri?.let { viewModel.addEvidence(selection(it, EvidenceKind.PHOTO)) }
    }
    fun launchCamera() {
        val values = ContentValues().apply {
            put(MediaStore.Images.Media.DISPLAY_NAME, "bahari_monitor_${System.currentTimeMillis()}.jpg")
            put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg")
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q)
                put(MediaStore.Images.Media.RELATIVE_PATH, "Pictures/BahariMRV")
        }
        val uri = context.contentResolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
        cameraUri = uri
        uri?.let { cameraPicker.launch(it) }
    }

    val audioPicker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        uri?.let { viewModel.addEvidence(selection(it, EvidenceKind.VOICE)) }
    }
    val locationPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { grants ->
        if (grants.values.any { it }) viewModel.locate()
        else viewModel.update { it.copy(error = "Location permission is needed for a device GPS fix.") }
    }
    val audioPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) {
            runCatching { recorder.start() }
                .onSuccess { viewModel.update { it.copy(recording = true, error = null) } }
                .onFailure { e -> viewModel.update { it.copy(error = e.message ?: "Voice recording could not start") } }
        } else viewModel.update { it.copy(error = "Microphone permission is needed to record a voice observation.") }
    }

    fun recordVoice() {
        if (state.recording) {
            runCatching { recorder.stop() }.onSuccess { viewModel.addEvidence(it) }
                .onFailure { e -> viewModel.update { it.copy(error = e.message ?: "Recording could not be saved") } }
            viewModel.update { it.copy(recording = false) }
        } else if (ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
            runCatching { recorder.start() }
                .onSuccess { viewModel.update { it.copy(recording = true, error = null) } }
                .onFailure { e -> viewModel.update { it.copy(error = e.message ?: "Voice recording could not start") } }
        } else audioPermission.launch(Manifest.permission.RECORD_AUDIO)
    }

    fun readGps() {
        val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_COARSE_LOCATION) == PackageManager.PERMISSION_GRANTED
        if (granted) viewModel.locate()
        else locationPermission.launch(arrayOf(Manifest.permission.ACCESS_FINE_LOCATION, Manifest.permission.ACCESS_COARSE_LOCATION))
    }

    val photoCount = state.evidence.count { it.kind == EvidenceKind.PHOTO }
    val voiceCount = state.evidence.count { it.kind == EvidenceKind.VOICE }

    // Photo source dialog
    if (showPhotoMenu) {
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { showPhotoMenu = false },
            title = { Text("Add photo evidence") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(onClick = { showPhotoMenu = false; launchCamera() }, modifier = Modifier.fillMaxWidth()) {
                        Icon(Icons.Default.PhotoCamera, null, modifier = Modifier.size(18.dp))
                        Text("  Take photo with camera")
                    }
                    OutlinedButton(onClick = { showPhotoMenu = false; galleryPicker.launch("image/*") }, modifier = Modifier.fillMaxWidth()) {
                        Icon(Icons.Default.PhotoLibrary, null, modifier = Modifier.size(18.dp))
                        Text("  Choose from gallery / files")
                    }
                }
            },
            confirmButton = { TextButton(onClick = { showPhotoMenu = false }) { Text("Cancel") } },
        )
    }

    LazyColumn(
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            ContextHeader(
                title = "Field monitoring",
                subtitle = "Plot ${state.plotId.ifBlank { "not set" }} · MRV capture",
                trailing = {
                    StatusChip(
                        text = if (state.pendingCount > 0) "${state.pendingCount} queued" else "Synced",
                        tone = if (state.pendingCount > 0) ChipTone.WARNING else ChipTone.POSITIVE,
                    )
                },
            )
        }

        item {
            FlowRow(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(7.dp),
                verticalArrangement = Arrangement.spacedBy(7.dp),
            ) {
                val (gpsLabel, gpsTone) = when (state.coordinateSource) {
                    CoordinateSource.DEVICE_FIX -> "GPS ±${state.gpsAccuracy.ifBlank { "?" }} m" to ChipTone.POSITIVE
                    CoordinateSource.MANUAL     -> "Manual coords · unverified" to ChipTone.WARNING
                    CoordinateSource.NONE       -> "No GPS fix yet" to ChipTone.NEUTRAL
                }
                StatusChip(gpsLabel, tone = gpsTone)
                StatusChip("Plot ${state.plotId.ifBlank { "not set" }}", tone = if (state.plotId.isBlank()) ChipTone.WARNING else ChipTone.NEUTRAL)
                if (state.methodologyVersion.isNotBlank()) StatusChip("v${state.methodologyVersion}")
            }
        }

        item {
            MicroLabel("Survival count")
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                CaptureField(state.aliveCount,   { v -> viewModel.update { it.copy(aliveCount = v) } },   "Alive",   Modifier.weight(1f), KeyboardType.Number)
                CaptureField(state.plantedCount, { v -> viewModel.update { it.copy(plantedCount = v) } }, "Planted", Modifier.weight(1f), KeyboardType.Number)
            }
        }

        item {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                CaptureField(state.averageHeight, { v -> viewModel.update { it.copy(averageHeight = v) } }, "Avg height (m)", Modifier.weight(1f), KeyboardType.Decimal)
                CaptureField(state.species,       { v -> viewModel.update { it.copy(species = v) } },       "Species",        Modifier.weight(1f))
            }
        }

        // Photo evidence — tappable dashed box
        item {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 88.dp)
                    .dashedBorder(MaterialTheme.colorScheme.outline, 12.dp)
                    .clickable { showPhotoMenu = true }
                    .padding(14.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Icon(Icons.Default.PhotoCamera, null, modifier = Modifier.size(26.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(
                    if (photoCount > 0) "Geo-tagged photos · $photoCount taken" else "Geo-tagged photos · none taken",
                    style = MaterialTheme.typography.titleSmall,
                    textAlign = TextAlign.Center,
                )
                Text("Tap to add · camera or gallery · hashed on this device", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
            }
        }

        state.evidence.forEachIndexed { index, evidence ->
            item {
                Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Column(Modifier.weight(1f)) {
                        Text(evidence.originalFileName, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        val coords = evidence.latitude?.let { lat -> evidence.longitude?.let { lon -> "GPS $lat, $lon" } } ?: "Coordinates pending"
                        Text("${evidence.kind.name.lowercase()} · hash computed · $coords", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    IconButton(onClick = { viewModel.removeEvidence(index) }) {
                        Icon(Icons.Default.Delete, "Remove ${evidence.originalFileName}")
                    }
                }
            }
        }

        item {
            OutlinedButton(onClick = ::recordVoice, modifier = Modifier.fillMaxWidth().height(48.dp)) {
                Icon(Icons.Default.Mic, null, modifier = Modifier.size(18.dp))
                Text(
                    text = when { state.recording -> "Stop and attach voice note"; voiceCount > 0 -> "Record another voice observation"; else -> "Voice observation" },
                    modifier = Modifier.padding(start = 8.dp),
                )
            }
        }

        if (voiceCount > 0 || photoCount > 0) {
            item {
                Text("$photoCount photo · $voiceCount voice attached. Media is SHA-256 hashed before the record is saved locally.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }

        item {
            TextButton(onClick = { showPlotSheet = true }, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
                Icon(Icons.Default.Edit, null, modifier = Modifier.size(18.dp))
                Text("Plot, activity & GPS", modifier = Modifier.padding(start = 8.dp), style = MaterialTheme.typography.bodyMedium)
                Text(
                    if (state.projectId.isBlank() || state.activityId.isBlank() || state.plotId.isBlank()) " · required" else " · set",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        state.error?.let { item { Notice(it, MaterialTheme.colorScheme.errorContainer, MaterialTheme.colorScheme.onErrorContainer, Icons.Default.Warning) } }
        state.message?.let { item { Notice(it, MaterialTheme.colorScheme.primaryContainer, MaterialTheme.colorScheme.onPrimaryContainer, Icons.Default.Check) } }

        if (state.error == null && state.message == null) {
            item {
                Text("Hash anchors to CreditMRV contract on sync", style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Medium, color = Color(0xFF079B76), modifier = Modifier.fillMaxWidth())
            }
        }

        item {
            Row(
                modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp).toggleable(value = state.attested, role = Role.Checkbox, onValueChange = { checked -> viewModel.update { it.copy(attested = checked) } }),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Checkbox(checked = state.attested, onCheckedChange = null)
                Text("I attest that this monitoring record reflects my field observation.", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
            }
        }

        item { Button(onClick = viewModel::save, enabled = !state.saving, modifier = Modifier.fillMaxWidth().height(52.dp)) { Text(if (state.saving) "Saving on device…" else "Save monitoring record") } }
        item { OutlinedButton(onClick = viewModel::syncNow, enabled = state.pendingCount > 0, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text(if (state.pendingCount > 0) "Sync ${state.pendingCount} queued records" else "No queued records") } }

        item {
            ExpandableSection("Submission history (${state.records.size})") {
                if (state.records.isEmpty()) {
                    Text("No field records on this device.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                } else {
                    state.records.forEach { QueuedRecordRow(it) }
                }
            }
        }
    }

    // ── Plot & GPS bottom sheet ────────────────────────────────────────────────
    if (showPlotSheet) {
        ModalBottomSheet(onDismissRequest = { showPlotSheet = false }) {
            Column(
                modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 16.dp).padding(bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                ContextHeader(
                    title = "Plot & GPS",
                    subtitle = "Identifiers and coordinates carried by this record",
                    trailing = { TextButton(onClick = { showPlotSheet = false }, modifier = Modifier.height(48.dp)) { Text("Done") } },
                )

                // ── Project selector ─────────────────────────────────────
                var projectExpanded by remember { mutableStateOf(false) }
                val selectedProject = state.availableProjects.find { it.id == state.projectId }
                ExposedDropdownMenuBox(expanded = projectExpanded, onExpandedChange = { if (!state.projectsLoading) projectExpanded = it }) {
                    OutlinedTextField(
                        value = when {
                            state.projectsLoading -> "Loading projects…"
                            selectedProject != null -> selectedProject.name
                            state.availableProjects.isEmpty() -> state.projectId.ifBlank { "No projects found" }
                            else -> "Select a project"
                        },
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Project") },
                        trailingIcon = {
                            if (state.projectsLoading) CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                            else Icon(if (projectExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown, "Select")
                        },
                        modifier = Modifier.fillMaxWidth().menuAnchor(),
                        singleLine = true,
                    )
                    if (state.availableProjects.isNotEmpty()) {
                        ExposedDropdownMenu(expanded = projectExpanded, onDismissRequest = { projectExpanded = false }) {
                            state.availableProjects.forEach { p ->
                                DropdownMenuItem(
                                    text = { Column { Text(p.name, fontWeight = FontWeight.Medium); Text(p.status, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) } },
                                    onClick = { viewModel.selectProject(p.id); projectExpanded = false },
                                )
                            }
                        }
                    }
                }

                // ── Plot selector ────────────────────────────────────────
                var plotExpanded by remember { mutableStateOf(false) }
                val selectedPlot = state.availablePlots.find { it.id == state.plotId }
                ExposedDropdownMenuBox(expanded = plotExpanded, onExpandedChange = { if (!state.plotsLoading) plotExpanded = it }) {
                    OutlinedTextField(
                        value = when {
                            state.plotsLoading -> "Loading plots…"
                            selectedPlot != null -> "${selectedPlot.label} · ${selectedPlot.status}"
                            state.availablePlots.isEmpty() && state.projectId.isNotBlank() -> "No plots found"
                            else -> if (state.plotId.isBlank()) "Select a plot" else state.plotId
                        },
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Plot") },
                        trailingIcon = {
                            if (state.plotsLoading) CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                            else Icon(if (plotExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown, "Select")
                        },
                        modifier = Modifier.fillMaxWidth().menuAnchor(),
                        singleLine = true,
                    )
                    if (state.availablePlots.isNotEmpty()) {
                        ExposedDropdownMenu(expanded = plotExpanded, onDismissRequest = { plotExpanded = false }) {
                            state.availablePlots.forEach { plot ->
                                DropdownMenuItem(
                                    text = { Column { Text(plot.label, fontWeight = FontWeight.Medium); Text("${plot.status} · ${plot.areaHectares ?: "??"} ha", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) } },
                                    onClick = { viewModel.update { it.copy(plotId = plot.id) }; plotExpanded = false },
                                )
                            }
                        }
                    }
                }

                // ── Activity selector ─────────────────────────────────────
                var activityExpanded by remember { mutableStateOf(false) }
                val selectedActivity = state.availableActivities.find { it.id == state.activityId }
                ExposedDropdownMenuBox(expanded = activityExpanded, onExpandedChange = { if (!state.activitiesLoading) activityExpanded = it }) {
                    OutlinedTextField(
                        value = when {
                            state.activitiesLoading -> "Loading activities…"
                            selectedActivity != null -> "${selectedActivity.activityType} · ${selectedActivity.status}"
                            state.availableActivities.isEmpty() && state.projectId.isNotBlank() -> "No activities found"
                            else -> if (state.activityId.isBlank()) "Select an activity" else state.activityId
                        },
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Activity") },
                        trailingIcon = {
                            if (state.activitiesLoading) CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                            else Icon(if (activityExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown, "Select")
                        },
                        modifier = Modifier.fillMaxWidth().menuAnchor(),
                        singleLine = true,
                    )
                    if (state.availableActivities.isNotEmpty()) {
                        ExposedDropdownMenu(expanded = activityExpanded, onDismissRequest = { activityExpanded = false }) {
                            state.availableActivities.forEach { act ->
                                DropdownMenuItem(
                                    text = { Column { Text("${act.activityType} · ${act.status}", fontWeight = FontWeight.Medium); Text(act.id.take(16) + "…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) } },
                                    onClick = { viewModel.update { it.copy(activityId = act.id) }; activityExpanded = false },
                                )
                            }
                        }
                    }
                }

                // ── Methodology dropdown ──────────────────────────────────
                var methodExpanded by remember { mutableStateOf(false) }
                val selectedMethod = state.availableMethodologies.find { "${it.code} ${it.version}" == state.methodologyVersion || it.id == state.methodologyVersion }
                ExposedDropdownMenuBox(expanded = methodExpanded, onExpandedChange = { if (!state.methodologiesLoading) methodExpanded = it }) {
                    OutlinedTextField(
                        value = when {
                            state.methodologiesLoading -> "Loading methodologies…"
                            selectedMethod != null -> "${selectedMethod.name} · v${selectedMethod.version}"
                            state.availableMethodologies.isEmpty() -> "No methodologies found"
                            else -> if (state.methodologyVersion.isBlank()) "Select methodology" else state.methodologyVersion
                        },
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Methodology") },
                        trailingIcon = {
                            if (state.methodologiesLoading) CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                            else Icon(if (methodExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown, "Select")
                        },
                        modifier = Modifier.fillMaxWidth().menuAnchor(),
                        singleLine = true,
                    )
                    if (state.availableMethodologies.isNotEmpty()) {
                        ExposedDropdownMenu(expanded = methodExpanded, onDismissRequest = { methodExpanded = false }) {
                            state.availableMethodologies.forEach { m ->
                                DropdownMenuItem(
                                    text = { Column { Text(m.name, fontWeight = FontWeight.Medium); Text("${m.code} · v${m.version}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) } },
                                    onClick = { viewModel.update { it.copy(methodologyVersion = m.version) }; methodExpanded = false },
                                )
                            }
                        }
                    }
                }

                // ── GPS ───────────────────────────────────────────────────
                OutlinedButton(onClick = ::readGps, enabled = !state.locating, modifier = Modifier.fillMaxWidth().height(48.dp)) {
                    Icon(Icons.Default.LocationOn, null, modifier = Modifier.size(18.dp))
                    Text(if (state.locating) "  Reading GPS…" else "  Use device GPS", modifier = Modifier.padding(start = 4.dp))
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    CaptureField(state.latitude,  viewModel::setLatitude,  "Latitude",  Modifier.weight(1f), KeyboardType.Decimal)
                    CaptureField(state.longitude, viewModel::setLongitude, "Longitude", Modifier.weight(1f), KeyboardType.Decimal)
                }
                CaptureField(state.gpsAccuracy, viewModel::setGpsAccuracy, "GPS accuracy (m)", keyboardType = KeyboardType.Decimal)
                Text("Manual coordinates are allowed but the record is marked unverified until a device fix is read.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

// ── Supporting composables ────────────────────────────────────────────────────

@Composable
private fun Notice(text: String, container: Color, content: Color, icon: ImageVector?) {
    Surface(modifier = Modifier.fillMaxWidth(), shape = MaterialTheme.shapes.medium, color = container, contentColor = content) {
        Row(modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (icon != null) Icon(icon, null, modifier = Modifier.size(16.dp))
            Text(text, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun QueuedRecordRow(item: MonitoringWithEvidence) {
    Column(modifier = Modifier.fillMaxWidth().dashedBorder(MaterialTheme.colorScheme.outlineVariant, 12.dp).padding(12.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("${item.record.plotId} • ${item.record.species}", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            StatusChip(item.record.syncState.name.replace('_', ' '), tone = stateTone(item.record.syncState.name))
        }
        Text("${item.record.aliveCount}/${item.record.plantedCount} alive • ${item.record.averageHeightMeters} m avg • GPS ±${item.record.gpsAccuracyMeters} m", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text("${item.evidence.size} evidence • ${item.evidence.joinToString { "${it.kind.name.lowercase()}: ${it.uploadState.name}" }}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
        ExpandableSection("Record IDs & commitment", initiallyExpanded = false) {
            Text("Client ID: ${item.record.clientId}", style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace)
            Text("Server ID: ${item.record.serverId ?: "Not assigned"}", style = MaterialTheme.typography.bodySmall)
            Text("Commitment: ${item.record.contentCommitment.take(20)}…", style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace)
        }
        item.record.lastError?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
    }
}

@Composable
private fun CaptureField(value: String, onValueChange: (String) -> Unit, label: String, modifier: Modifier = Modifier, keyboardType: KeyboardType = KeyboardType.Text) {
    OutlinedTextField(value = value, onValueChange = onValueChange, label = { Text(label) }, keyboardOptions = KeyboardOptions(keyboardType = keyboardType), singleLine = true, modifier = modifier.fillMaxWidth())
}

private fun Context.displayName(uri: Uri): String? =
    contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c -> if (c.moveToFirst()) c.getString(0) else null }
