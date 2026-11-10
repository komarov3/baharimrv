package ke.co.baharilink.baharimrv.ui.planting

import android.content.ContentValues
import android.net.Uri
import android.os.Build
import android.provider.MediaStore
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.ui.components.*
import java.text.SimpleDateFormat
import java.util.*

// ── Species data ──────────────────────────────────────────────────────────────

data class MangroveSpecies(
    val scientificName: String,
    val commonName: String,
    val family: String,
    val typicalSurvivalRate: String,
    val growthRate: String,
)

val mangroveSpeciesList = listOf(
    MangroveSpecies("Rhizophora mucronata", "Red Mangrove",        "Rhizophoraceae", "70-85%", "Fast"),
    MangroveSpecies("Avicennia marina",     "Grey Mangrove",       "Avicenniaceae",  "65-80%", "Medium"),
    MangroveSpecies("Ceriops tagal",        "Yellow Mangrove",     "Rhizophoraceae", "60-75%", "Slow"),
    MangroveSpecies("Sonneratia alba",      "Mangrove Apple",      "Lythraceae",     "55-70%", "Fast"),
    MangroveSpecies("Bruguiera gymnorrhiza","Oriental Mangrove",   "Rhizophoraceae", "65-80%", "Medium"),
    MangroveSpecies("Xylocarpus granatum",  "Cannonball Mangrove", "Meliaceae",      "50-65%", "Slow"),
)

data class PendingPhoto(
    val uri: Uri,
    val fileName: String,
    val sizeBytes: Long,
    val hashStatus: String = "hashed on device",
)

private val GreenPrimary = Color(0xFF059669)
private val dateFmt = SimpleDateFormat("yyyy-MM-dd", Locale.US)

// ── Screen ────────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlantingActivityScreen(
    viewModel: PlantingActivityViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state   by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    var speciesExpanded      by remember { mutableStateOf(false) }
    var showStartDatePicker  by remember { mutableStateOf(false) }
    var showEndDatePicker    by remember { mutableStateOf(false) }
    var showConditions       by remember { mutableStateOf(false) }
    var showTeamInfo         by remember { mutableStateOf(false) }
    var showPhotoMenu        by remember { mutableStateOf(false) }
    var pendingPhotos        by remember { mutableStateOf(listOf<PendingPhoto>()) }

    // Extra conditions / team state
    var weatherCondition by remember { mutableStateOf("") }
    var tideCondition    by remember { mutableStateOf("") }
    var soilCondition    by remember { mutableStateOf("") }
    var teamSize         by remember { mutableStateOf("") }
    var teamLead         by remember { mutableStateOf("") }
    var volunteers       by remember { mutableStateOf("") }
    var notes            by remember { mutableStateOf("") }

    // Camera URI holder
    var cameraUri by remember { mutableStateOf<Uri?>(null) }

    // Helper: resolve display name + size from a URI
    fun resolvePhotoMeta(uri: Uri): PendingPhoto {
        val (name, size) = context.contentResolver
            .query(uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)
            ?.use { c ->
                if (c.moveToFirst()) c.getString(0) to c.getLong(1)
                else "photo-${System.currentTimeMillis()}" to 0L
            } ?: ("photo-${System.currentTimeMillis()}" to 0L)
        return PendingPhoto(uri = uri, fileName = name, sizeBytes = size)
    }

    // Gallery / file picker
    val galleryPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri -> uri?.let { pendingPhotos = pendingPhotos + resolvePhotoMeta(it) } }

    // Camera capture
    val cameraPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { captured ->
        if (captured) {
            cameraUri?.let { uri -> pendingPhotos = pendingPhotos + resolvePhotoMeta(uri) }
        }
    }

    fun launchCamera() {
        val values = ContentValues().apply {
            put(MediaStore.Images.Media.DISPLAY_NAME, "bahari_${System.currentTimeMillis()}.jpg")
            put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg")
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                put(MediaStore.Images.Media.RELATIVE_PATH, "Pictures/BahariMRV")
            }
        }
        val uri = context.contentResolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
        cameraUri = uri
        uri?.let { cameraPicker.launch(it) }
    }

    // Date picker state
    val startPickerState = rememberDatePickerState()
    val endPickerState   = rememberDatePickerState()

    LaunchedEffect(projectId) { viewModel.setProjectId(projectId) }

    // ── Date pickers ───────────────────────────────────────────────────────────
    if (showStartDatePicker) {
        DatePickerDialog(
            onDismissRequest = { showStartDatePicker = false },
            confirmButton = {
                TextButton(onClick = {
                    startPickerState.selectedDateMillis?.let { ms ->
                        viewModel.setReportingPeriodStart(dateFmt.format(Date(ms)))
                    }
                    showStartDatePicker = false
                }) { Text("OK") }
            },
            dismissButton = {
                TextButton(onClick = { showStartDatePicker = false }) { Text("Cancel") }
            },
        ) {
            DatePicker(state = startPickerState)
        }
    }

    if (showEndDatePicker) {
        DatePickerDialog(
            onDismissRequest = { showEndDatePicker = false },
            confirmButton = {
                TextButton(onClick = {
                    endPickerState.selectedDateMillis?.let { ms ->
                        viewModel.setReportingPeriodEnd(dateFmt.format(Date(ms)))
                    }
                    showEndDatePicker = false
                }) { Text("OK") }
            },
            dismissButton = {
                TextButton(onClick = { showEndDatePicker = false }) { Text("Cancel") }
            },
        ) {
            DatePicker(state = endPickerState)
        }
    }

    // ── Photo source menu ─────────────────────────────────────────────────────
    if (showPhotoMenu) {
        AlertDialog(
            onDismissRequest = { showPhotoMenu = false },
            title = { Text("Add photo") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(
                        onClick = { showPhotoMenu = false; launchCamera() },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Icon(Icons.Default.PhotoCamera, null, modifier = Modifier.size(20.dp))
                        Spacer(Modifier.width(8.dp))
                        Text("Take photo with camera")
                    }
                    OutlinedButton(
                        onClick = { showPhotoMenu = false; galleryPicker.launch("image/*") },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Icon(Icons.Default.PhotoLibrary, null, modifier = Modifier.size(20.dp))
                        Spacer(Modifier.width(8.dp))
                        Text("Choose from gallery / files")
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = { showPhotoMenu = false }) { Text("Cancel") }
            },
        )
    }

    // ── Main layout ───────────────────────────────────────────────────────────
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("New Planting Activity") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, "Back")
                    }
                },
            )
        },
    ) { padding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {

            // Header
            item {
                ContextHeader(
                    title = "New planting activity",
                    subtitle = "Plot ${state.plotId.ifBlank { "not set" }} · ${
                        if (state.projectId.isNotBlank()) state.projectId else "no project"
                    }",
                )
            }

            // Chain notice
            item {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    color = Color(0xFFE7F8F1),
                    shape = MaterialTheme.shapes.small,
                ) {
                    Row(
                        modifier = Modifier.padding(12.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(Icons.Default.Link, null, tint = GreenPrimary, modifier = Modifier.size(16.dp))
                        Column {
                            Text(
                                "Activity becomes the baseline for survival monitoring.",
                                style = MaterialTheme.typography.bodySmall,
                                fontWeight = FontWeight.SemiBold,
                                color = GreenPrimary,
                            )
                            Text(
                                "Photo hashes anchor to CreditMRV contract on sync · signed by Blue ID",
                                style = MaterialTheme.typography.bodySmall,
                                color = GreenPrimary.copy(alpha = 0.8f),
                            )
                        }
                    }
                }
            }

            // ── Core Information ──────────────────────────────────────────
            item {
                BahariCard {
                    Text("Core Information", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(12.dp))

                    // ── Plot selector ─────────────────────────────────────
                    var plotExpanded by remember { mutableStateOf(false) }
                    val selectedPlot = state.availablePlots.find { it.id == state.plotId }
                    ExposedDropdownMenuBox(
                        expanded = plotExpanded,
                        onExpandedChange = { if (!state.plotsLoading) plotExpanded = it },
                    ) {
                        OutlinedTextField(
                            value = when {
                                state.plotsLoading -> "Loading plots…"
                                selectedPlot != null -> "${selectedPlot.label} · ${selectedPlot.status}"
                                state.availablePlots.isEmpty() -> "No plots found for project"
                                else -> "Select a plot"
                            },
                            onValueChange = {},
                            readOnly = true,
                            label = { Text("Plot") },
                            leadingIcon = { Icon(Icons.Default.Place, null, modifier = Modifier.size(20.dp)) },
                            trailingIcon = {
                                if (state.plotsLoading)
                                    CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                                else
                                    Icon(if (plotExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown, "Select plot")
                            },
                            modifier = Modifier.fillMaxWidth().menuAnchor(),
                            singleLine = true,
                        )
                        if (state.availablePlots.isNotEmpty()) {
                            ExposedDropdownMenu(expanded = plotExpanded, onDismissRequest = { plotExpanded = false }) {
                                state.availablePlots.forEach { plot ->
                                    DropdownMenuItem(
                                        text = {
                                            Column {
                                                Text(plot.label, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
                                                Text("${plot.status} · ${plot.areaHectares ?: "??"} ha", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                            }
                                        },
                                        onClick = { viewModel.setPlotId(plot.id); plotExpanded = false },
                                    )
                                }
                            }
                        }
                    }

                    Spacer(Modifier.height(12.dp))

                    OutlinedTextField(
                        value = "Planting · mangrove",
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Activity type") },
                        leadingIcon = { Icon(Icons.Default.Nature, null, modifier = Modifier.size(20.dp)) },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                    )

                    Spacer(Modifier.height(12.dp))

                    // Species dropdown
                    ExposedDropdownMenuBox(
                        expanded = speciesExpanded,
                        onExpandedChange = { speciesExpanded = it },
                    ) {
                        OutlinedTextField(
                            value = state.species,
                            onValueChange = {},
                            readOnly = true,
                            label = { Text("Species") },
                            leadingIcon = { Icon(Icons.Default.Nature, null, modifier = Modifier.size(20.dp)) },
                            trailingIcon = {
                                Icon(
                                    if (speciesExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                                    "Select species",
                                )
                            },
                            modifier = Modifier.fillMaxWidth().menuAnchor(),
                            singleLine = true,
                        )
                        ExposedDropdownMenu(
                            expanded = speciesExpanded,
                            onDismissRequest = { speciesExpanded = false },
                        ) {
                            mangroveSpeciesList.forEach { sp ->
                                DropdownMenuItem(
                                    text = {
                                        Column {
                                            Text(sp.scientificName, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
                                            Text("${sp.commonName} · ${sp.typicalSurvivalRate} survival", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                        }
                                    },
                                    onClick = { viewModel.setSpecies(sp.scientificName); speciesExpanded = false },
                                )
                            }
                        }
                    }

                    // Species info chip
                    if (state.species.isNotBlank()) {
                        mangroveSpeciesList.find { it.scientificName == state.species }?.let { sp ->
                            Spacer(Modifier.height(8.dp))
                            Card(
                                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer),
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Row(modifier = Modifier.padding(12.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text("Growth Rate", style = MaterialTheme.typography.labelSmall)
                                        Text(sp.growthRate, fontWeight = FontWeight.Bold)
                                    }
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text("Survival Rate", style = MaterialTheme.typography.labelSmall)
                                        Text(sp.typicalSurvivalRate, fontWeight = FontWeight.Bold)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ── Seedlings planted ─────────────────────────────────────────
            item {
                BahariCard {
                    Text("Seedlings planted", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                    Text(
                        "This count is the planted baseline — the denominator for survival rate",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Spacer(Modifier.height(8.dp))

                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        IconButton(onClick = {
                            val n = state.seedlingsPlanted.toIntOrNull() ?: 0
                            if (n > 0) viewModel.setSeedlingsPlanted((n - 10).coerceAtLeast(0).toString())
                        }) { Icon(Icons.Default.Remove, "Decrease") }
                        OutlinedTextField(
                            value = state.seedlingsPlanted,
                            onValueChange = { viewModel.setSeedlingsPlanted(it) },
                            label = { Text("Seedlings planted") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            modifier = Modifier.weight(1f),
                            singleLine = true,
                        )
                        IconButton(onClick = {
                            val n = state.seedlingsPlanted.toIntOrNull() ?: 0
                            viewModel.setSeedlingsPlanted((n + 10).toString())
                        }) { Icon(Icons.Default.Add, "Increase") }
                    }

                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        listOf(100, 250, 500, 1000).forEach { preset ->
                            FilterChip(
                                selected = state.seedlingsPlanted == preset.toString(),
                                onClick = { viewModel.setSeedlingsPlanted(preset.toString()) },
                                label = { Text(preset.toString()) },
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }

                    // Expected survival card
                    val planted = state.seedlingsPlanted.toIntOrNull()
                    val sp = mangroveSpeciesList.find { it.scientificName == state.species }
                    if (planted != null && sp != null) {
                        val range = sp.typicalSurvivalRate.removeSuffix("%").split("-")
                        if (range.size == 2) {
                            val min = planted * range[0].trim().toInt() / 100
                            val max = planted * range[1].trim().toInt() / 100
                            Spacer(Modifier.height(8.dp))
                            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
                                Row(modifier = Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    Icon(Icons.Default.Assessment, null, modifier = Modifier.size(16.dp))
                                    Text("Expected survival: $min – $max seedlings", style = MaterialTheme.typography.bodyMedium)
                                }
                            }
                        }
                    }
                }
            }

            // ── Photo evidence ────────────────────────────────────────────
            item {
                BahariCard {
                    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
                        Column {
                            Text("Geo-tagged photos", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                            Text("IPFS-hashed · tamper-proof audit evidence · signed by Blue ID", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        if (pendingPhotos.isNotEmpty()) {
                            StatusChip("${pendingPhotos.size} taken", tone = ChipTone.POSITIVE)
                        }
                    }

                    Spacer(Modifier.height(8.dp))

                    // Two buttons: camera and gallery
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(
                            onClick = { launchCamera() },
                            modifier = Modifier.weight(1f),
                        ) {
                            Icon(Icons.Default.PhotoCamera, null, modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(6.dp))
                            Text("Camera")
                        }
                        OutlinedButton(
                            onClick = { galleryPicker.launch("image/*") },
                            modifier = Modifier.weight(1f),
                        ) {
                            Icon(Icons.Default.PhotoLibrary, null, modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(6.dp))
                            Text("Gallery / files")
                        }
                    }

                    // Photo list
                    if (pendingPhotos.isNotEmpty()) {
                        Spacer(Modifier.height(8.dp))
                        pendingPhotos.forEachIndexed { idx, photo ->
                            Row(
                                modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Icon(Icons.Default.Image, null, modifier = Modifier.size(18.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(photo.fileName, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    Text("photo · ${photo.hashStatus}", style = MaterialTheme.typography.bodySmall, color = GreenPrimary)
                                }
                                IconButton(onClick = {
                                    pendingPhotos = pendingPhotos.toMutableList().also { it.removeAt(idx) }
                                }) {
                                    Icon(Icons.Default.Delete, "Remove", modifier = Modifier.size(16.dp))
                                }
                            }
                        }
                        Text(
                            "${pendingPhotos.size} photo(s) · SHA-256 hashed on device · will upload to IPFS on sync",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }

            // ── Methodology ───────────────────────────────────────────────
            item {
                OutlinedTextField(
                    value = state.methodologyVersion,
                    onValueChange = { viewModel.setMethodologyVersion(it) },
                    label = { Text("Methodology") },
                    leadingIcon = { Icon(Icons.Default.Description, null, modifier = Modifier.size(20.dp)) },
                    placeholder = { Text("e.g., MRV v1.2, Verra VM0033") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                )
            }

            // ── Reporting period ──────────────────────────────────────────
            item {
                BahariCard {
                    Text("Reporting period", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(12.dp))

                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(
                            onClick = { showStartDatePicker = true },
                            modifier = Modifier.weight(1f),
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(Icons.Default.CalendarToday, null, modifier = Modifier.size(20.dp))
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    if (state.reportingPeriodStart.isBlank()) "Start date" else state.reportingPeriodStart,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                        OutlinedButton(
                            onClick = { showEndDatePicker = true },
                            modifier = Modifier.weight(1f),
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(Icons.Default.Event, null, modifier = Modifier.size(20.dp))
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    if (state.reportingPeriodEnd.isBlank()) "End date" else state.reportingPeriodEnd,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }
                    }

                    Text(
                        "e.g. Jan – Mar 2026 · Monitoring period typically 90 days",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            // ── Planting conditions (expandable) ──────────────────────────
            item {
                Card(
                    modifier = Modifier.fillMaxWidth().clickable { showConditions = !showConditions },
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(16.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Cloud, null)
                            Column {
                                Text("Planting conditions", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
                                Text("Optional: weather, tide, soil", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                        Icon(if (showConditions) Icons.Default.ExpandLess else Icons.Default.ExpandMore, "Expand")
                    }

                    AnimatedVisibility(
                        visible = showConditions,
                        enter = expandVertically(),
                        exit = shrinkVertically(),
                    ) {
                        Column(
                            modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp),
                            verticalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            OutlinedTextField(
                                value = weatherCondition,
                                onValueChange = { weatherCondition = it },
                                label = { Text("Weather") },
                                placeholder = { Text("e.g. Sunny, 28°C, light breeze") },
                                leadingIcon = { Icon(Icons.Default.WbSunny, null, modifier = Modifier.size(20.dp)) },
                                modifier = Modifier.fillMaxWidth(),
                                singleLine = true,
                            )
                            OutlinedTextField(
                                value = tideCondition,
                                onValueChange = { tideCondition = it },
                                label = { Text("Tide conditions") },
                                placeholder = { Text("e.g. Low tide, calm water") },
                                leadingIcon = { Icon(Icons.Default.Water, null, modifier = Modifier.size(20.dp)) },
                                modifier = Modifier.fillMaxWidth(),
                                singleLine = true,
                            )
                            OutlinedTextField(
                                value = soilCondition,
                                onValueChange = { soilCondition = it },
                                label = { Text("Soil / substrate") },
                                placeholder = { Text("e.g. Soft mud, well-drained") },
                                leadingIcon = { Icon(Icons.Default.Landscape, null, modifier = Modifier.size(20.dp)) },
                                modifier = Modifier.fillMaxWidth(),
                                singleLine = true,
                            )
                            OutlinedTextField(
                                value = notes,
                                onValueChange = { notes = it },
                                label = { Text("Additional notes") },
                                placeholder = { Text("Any other relevant observations") },
                                modifier = Modifier.fillMaxWidth(),
                                minLines = 2,
                                maxLines = 4,
                            )
                        }
                    }
                }
            }

            // ── Team information (expandable) ─────────────────────────────
            item {
                Card(
                    modifier = Modifier.fillMaxWidth().clickable { showTeamInfo = !showTeamInfo },
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(16.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Group, null)
                            Column {
                                Text("Team information", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Medium)
                                Text("Optional: members, volunteers", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                        Icon(if (showTeamInfo) Icons.Default.ExpandLess else Icons.Default.ExpandMore, "Expand")
                    }

                    AnimatedVisibility(
                        visible = showTeamInfo,
                        enter = expandVertically(),
                        exit = shrinkVertically(),
                    ) {
                        Column(
                            modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp),
                            verticalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            OutlinedTextField(
                                value = teamLead,
                                onValueChange = { teamLead = it },
                                label = { Text("Team lead") },
                                placeholder = { Text("Name of the lead field officer") },
                                leadingIcon = { Icon(Icons.Default.Person, null, modifier = Modifier.size(20.dp)) },
                                modifier = Modifier.fillMaxWidth(),
                                singleLine = true,
                            )
                            OutlinedTextField(
                                value = teamSize,
                                onValueChange = { teamSize = it },
                                label = { Text("Team size") },
                                placeholder = { Text("Total number of participants") },
                                leadingIcon = { Icon(Icons.Default.Group, null, modifier = Modifier.size(20.dp)) },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                                modifier = Modifier.fillMaxWidth(),
                                singleLine = true,
                            )
                            OutlinedTextField(
                                value = volunteers,
                                onValueChange = { volunteers = it },
                                label = { Text("Volunteers / community members") },
                                placeholder = { Text("Names or groups involved") },
                                leadingIcon = { Icon(Icons.Default.People, null, modifier = Modifier.size(20.dp)) },
                                modifier = Modifier.fillMaxWidth(),
                                minLines = 2,
                                maxLines = 3,
                            )
                        }
                    }
                }
            }

            // ── Submit ────────────────────────────────────────────────────
            item {
                BahariButton(
                    onClick = viewModel::create,
                    variant = BahariButtonVariant.GREEN,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = state.plotId.isNotBlank() &&
                              state.species.isNotBlank() &&
                              state.methodologyVersion.isNotBlank() &&
                              state.seedlingsPlanted.isNotBlank() &&
                              !state.creating,
                ) {
                    Row(horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                        if (state.creating) {
                            CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp)
                            Spacer(Modifier.width(8.dp))
                        } else {
                            Icon(Icons.Default.Check, null, modifier = Modifier.size(20.dp))
                            Spacer(Modifier.width(8.dp))
                        }
                        Text(
                            if (state.creating) "Creating activity…" else "Create planting activity",
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }

            // ── Feedback ──────────────────────────────────────────────────
            item {
                if (state.error != null) {
                    ErrorState(message = state.error!!, onRetry = {})
                }
                if (state.message != null) {
                    Card(colors = CardDefaults.cardColors(containerColor = Color(0xFFE7F8F1))) {
                        Row(
                            modifier = Modifier.padding(16.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(Icons.Default.CheckCircle, null, tint = GreenPrimary, modifier = Modifier.size(20.dp))
                            Column {
                                Text(state.message!!, color = Color(0xFF047857), fontWeight = FontWeight.Medium)
                                Text("Next step: record survival during a field visit", style = MaterialTheme.typography.bodySmall, color = Color(0xFF059669))
                            }
                        }
                    }
                }
            }
        }
    }
}
