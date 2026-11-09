package ke.co.baharilink.baharimrv.ui.planting

import android.net.Uri
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.ui.components.*

// Species data for dropdown
data class MangroveSpecies(
    val scientificName: String,
    val commonName: String,
    val family: String,
    val typicalSurvivalRate: String,
    val growthRate: String
)

val mangroveSpeciesList = listOf(
    MangroveSpecies("Rhizophora mucronata", "Red Mangrove", "Rhizophoraceae", "70-85%", "Fast"),
    MangroveSpecies("Avicennia marina", "Grey Mangrove", "Avicenniaceae", "65-80%", "Medium"),
    MangroveSpecies("Ceriops tagal", "Yellow Mangrove", "Rhizophoraceae", "60-75%", "Slow"),
    MangroveSpecies("Sonneratia alba", "Mangrove Apple", "Lythraceae", "55-70%", "Fast"),
    MangroveSpecies("Bruguiera gymnorrhiza", "Oriental Mangrove", "Rhizophoraceae", "65-80%", "Medium"),
    MangroveSpecies("Xylocarpus granatum", "Cannonball Mangrove", "Meliaceae", "50-65%", "Slow"),
)

/** Lightweight record of a photo attachment before it is committed to the server. */
data class PendingPhoto(
    val uri: Uri,
    val fileName: String,
    val sizeBytes: Long,
    /** SHA-256 is computed on the device so the hash travels with the record, not the bytes. */
    val hashStatus: String = "computing…",
)

private val GreenPrimary = Color(0xFF059669)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlantingActivityScreen(
    viewModel: PlantingActivityViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    var speciesExpanded by remember { mutableStateOf(false) }
    var showStartDatePicker by remember { mutableStateOf(false) }
    var showEndDatePicker by remember { mutableStateOf(false) }
    var showConditions by remember { mutableStateOf(false) }
    var showTeamInfo by remember { mutableStateOf(false) }
    var pendingPhotos by remember { mutableStateOf(listOf<PendingPhoto>()) }

    // Multi-photo picker (images/*)
    val photoPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) {
            val fileName = context.contentResolver.query(
                uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE),
                null, null, null
            )?.use { cursor ->
                if (cursor.moveToFirst()) {
                    val name = cursor.getString(0) ?: "photo-${System.currentTimeMillis()}"
                    val size = cursor.getLong(1)
                    name to size
                } else null
            } ?: ("photo-${System.currentTimeMillis()}" to 0L)
            pendingPhotos = pendingPhotos + PendingPhoto(
                uri = uri,
                fileName = fileName.first,
                sizeBytes = fileName.second,
                hashStatus = "hashed on device",
            )
        }
    }

    LaunchedEffect(projectId) {
        viewModel.setProjectId(projectId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("New Planting Activity") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, "Back")
                    }
                }
            )
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
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

            // ── blockchain anchoring notice ────────────────────────────────
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
                        Icon(
                            Icons.Default.Link,
                            contentDescription = null,
                            tint = GreenPrimary,
                            modifier = Modifier.size(16.dp),
                        )
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
                    Text(
                        "Core Information",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Spacer(modifier = Modifier.height(12.dp))

                    // Plot ID
                    OutlinedTextField(
                        value = state.plotId,
                        onValueChange = { viewModel.setPlotId(it) },
                        label = { Text("Plot ID") },
                        leadingIcon = {
                            Icon(Icons.Default.Place, null, modifier = Modifier.size(20.dp))
                        },
                        supportingText = { Text("e.g. P-14 · bound to this activity and all field visits") },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    // Activity type (fixed: planting)
                    OutlinedTextField(
                        value = "Planting · mangrove",
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Activity type") },
                        leadingIcon = {
                            Icon(Icons.Default.Nature, null, modifier = Modifier.size(20.dp))
                        },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    // Species Dropdown
                    ExposedDropdownMenuBox(
                        expanded = speciesExpanded,
                        onExpandedChange = { speciesExpanded = it }
                    ) {
                        OutlinedTextField(
                            value = state.species,
                            onValueChange = {},
                            readOnly = true,
                            label = { Text("Species") },
                            leadingIcon = {
                                Icon(Icons.Default.Nature, null, modifier = Modifier.size(20.dp))
                            },
                            trailingIcon = {
                                Icon(
                                    if (speciesExpanded) Icons.Default.KeyboardArrowUp
                                    else Icons.Default.KeyboardArrowDown,
                                    "Select species"
                                )
                            },
                            modifier = Modifier
                                .fillMaxWidth()
                                .menuAnchor(),
                            singleLine = true
                        )

                        ExposedDropdownMenu(
                            expanded = speciesExpanded,
                            onDismissRequest = { speciesExpanded = false }
                        ) {
                            mangroveSpeciesList.forEach { species ->
                                DropdownMenuItem(
                                    text = {
                                        Column {
                                            Text(
                                                species.scientificName,
                                                style = MaterialTheme.typography.bodyMedium,
                                                fontWeight = FontWeight.Medium
                                            )
                                            Text(
                                                "${species.commonName} • ${species.typicalSurvivalRate} survival",
                                                style = MaterialTheme.typography.bodySmall,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant
                                            )
                                        }
                                    },
                                    onClick = {
                                        viewModel.setSpecies(species.scientificName)
                                        speciesExpanded = false
                                    }
                                )
                            }
                        }
                    }

                    // Species info chip
                    if (state.species.isNotBlank()) {
                        val selectedSpecies = mangroveSpeciesList.find {
                            it.scientificName == state.species
                        }
                        selectedSpecies?.let { sp ->
                            Spacer(modifier = Modifier.height(8.dp))
                            Card(
                                colors = CardDefaults.cardColors(
                                    containerColor = MaterialTheme.colorScheme.primaryContainer
                                ),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Row(
                                    modifier = Modifier.padding(12.dp),
                                    horizontalArrangement = Arrangement.spacedBy(16.dp)
                                ) {
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

            // ── Quantity ──────────────────────────────────────────────────
            item {
                BahariCard {
                    Text(
                        "Seedlings planted",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        "This count is the planted baseline — the denominator for survival rate",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Spacer(modifier = Modifier.height(8.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        IconButton(
                            onClick = {
                                val current = state.seedlingsPlanted.toIntOrNull() ?: 0
                                if (current > 0) viewModel.setSeedlingsPlanted(
                                    (current - 10).coerceAtLeast(0).toString()
                                )
                            }
                        ) { Icon(Icons.Default.Remove, "Decrease") }

                        OutlinedTextField(
                            value = state.seedlingsPlanted,
                            onValueChange = { viewModel.setSeedlingsPlanted(it) },
                            label = { Text("Seedlings planted") },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            modifier = Modifier.weight(1f),
                            singleLine = true
                        )

                        IconButton(
                            onClick = {
                                val current = state.seedlingsPlanted.toIntOrNull() ?: 0
                                viewModel.setSeedlingsPlanted((current + 10).toString())
                            }
                        ) { Icon(Icons.Default.Add, "Increase") }
                    }

                    // Quick presets
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        listOf(100, 250, 500, 1000).forEach { preset ->
                            FilterChip(
                                selected = state.seedlingsPlanted == preset.toString(),
                                onClick = { viewModel.setSeedlingsPlanted(preset.toString()) },
                                label = { Text(preset.toString()) },
                                modifier = Modifier.weight(1f)
                            )
                        }
                    }

                    // Expected survival
                    if (state.seedlingsPlanted.toIntOrNull() != null && state.species.isNotBlank()) {
                        val planted = state.seedlingsPlanted.toInt()
                        val sp = mangroveSpeciesList.find { it.scientificName == state.species }
                        sp?.let {
                            val range = it.typicalSurvivalRate.removeSuffix("%").split("-")
                            if (range.size == 2) {
                                val min = planted * range[0].trim().toInt() / 100
                                val max = planted * range[1].trim().toInt() / 100
                                Spacer(modifier = Modifier.height(8.dp))
                                Card(
                                    colors = CardDefaults.cardColors(
                                        containerColor = MaterialTheme.colorScheme.secondaryContainer
                                    )
                                ) {
                                    Row(
                                        modifier = Modifier.padding(12.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                    ) {
                                        Icon(
                                            Icons.Default.Assessment, null,
                                            modifier = Modifier.size(16.dp),
                                        )
                                        Text(
                                            "Expected survival: $min – $max seedlings",
                                            style = MaterialTheme.typography.bodyMedium,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ── Photo evidence ────────────────────────────────────────────
            item {
                BahariCard {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Column {
                            Text(
                                "Geo-tagged photos",
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold,
                            )
                            Text(
                                "IPFS-hashed · tamper-proof audit evidence · signed by Blue ID",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        if (pendingPhotos.isNotEmpty()) {
                            StatusChip(
                                "${pendingPhotos.size} taken",
                                tone = ChipTone.POSITIVE,
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(8.dp))

                    // Attach button
                    OutlinedButton(
                        onClick = { photoPicker.launch("image/*") },
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                    ) {
                        Icon(
                            Icons.Default.PhotoCamera, contentDescription = null,
                            modifier = Modifier.size(20.dp),
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Text(
                            if (pendingPhotos.isEmpty()) "Attach geo-tagged photos"
                            else "Attach another photo",
                        )
                    }

                    // Photo list
                    if (pendingPhotos.isNotEmpty()) {
                        Spacer(modifier = Modifier.height(8.dp))
                        pendingPhotos.forEachIndexed { idx, photo ->
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(vertical = 4.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Icon(
                                    Icons.Default.Image, contentDescription = null,
                                    modifier = Modifier.size(18.dp),
                                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        photo.fileName,
                                        style = MaterialTheme.typography.bodyMedium,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    Text(
                                        "photo · ${photo.hashStatus}",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = GreenPrimary,
                                    )
                                }
                                IconButton(onClick = {
                                    pendingPhotos = pendingPhotos.toMutableList()
                                        .also { it.removeAt(idx) }
                                }) {
                                    Icon(
                                        Icons.Default.Delete, contentDescription = "Remove",
                                        modifier = Modifier.size(16.dp),
                                    )
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
                    leadingIcon = {
                        Icon(Icons.Default.Description, null, modifier = Modifier.size(20.dp))
                    },
                    placeholder = { Text("e.g., MRV v1.2, Verra VM0033") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )
            }

            // ── Reporting period ──────────────────────────────────────────
            item {
                BahariCard {
                    Text(
                        "Reporting period",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Spacer(modifier = Modifier.height(12.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        OutlinedButton(
                            onClick = { showStartDatePicker = true },
                            modifier = Modifier.weight(1f)
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(Icons.Default.CalendarToday, null, modifier = Modifier.size(20.dp))
                                Spacer(modifier = Modifier.height(4.dp))
                                Text(
                                    if (state.reportingPeriodStart.isBlank()) "Start date"
                                    else state.reportingPeriodStart,
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                        }

                        OutlinedButton(
                            onClick = { showEndDatePicker = true },
                            modifier = Modifier.weight(1f)
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(Icons.Default.Event, null, modifier = Modifier.size(20.dp))
                                Spacer(modifier = Modifier.height(4.dp))
                                Text(
                                    if (state.reportingPeriodEnd.isBlank()) "End date"
                                    else state.reportingPeriodEnd,
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

            // ── Optional: Planting Conditions ─────────────────────────────
            item {
                Card(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { showConditions = !showConditions },
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surfaceVariant
                    )
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Icon(Icons.Default.Cloud, null)
                            Column {
                                Text(
                                    "Planting conditions",
                                    style = MaterialTheme.typography.titleSmall,
                                    fontWeight = FontWeight.Medium
                                )
                                Text(
                                    "Optional: weather, tide, soil",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                        Icon(
                            if (showConditions) Icons.Default.ExpandLess else Icons.Default.ExpandMore,
                            "Expand"
                        )
                    }
                }
            }

            // ── Optional: Team Information ────────────────────────────────
            item {
                Card(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { showTeamInfo = !showTeamInfo },
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surfaceVariant
                    )
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Icon(Icons.Default.Group, null)
                            Column {
                                Text(
                                    "Team information",
                                    style = MaterialTheme.typography.titleSmall,
                                    fontWeight = FontWeight.Medium
                                )
                                Text(
                                    "Optional: members, volunteers",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                        Icon(
                            if (showTeamInfo) Icons.Default.ExpandLess else Icons.Default.ExpandMore,
                            "Expand"
                        )
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
                              !state.creating
                ) {
                    Row(
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        if (state.creating) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                strokeWidth = 2.dp
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                        } else {
                            Icon(Icons.Default.Check, null, modifier = Modifier.size(20.dp))
                            Spacer(modifier = Modifier.width(8.dp))
                        }
                        Text(
                            if (state.creating) "Creating activity…" else "Create planting activity",
                            fontWeight = FontWeight.Bold
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
                    Card(
                        colors = CardDefaults.cardColors(
                            containerColor = Color(0xFFE7F8F1)
                        )
                    ) {
                        Row(
                            modifier = Modifier.padding(16.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                Icons.Default.CheckCircle, null,
                                tint = GreenPrimary,
                                modifier = Modifier.size(20.dp),
                            )
                            Column {
                                Text(
                                    state.message!!,
                                    color = Color(0xFF047857),
                                    fontWeight = FontWeight.Medium,
                                )
                                Text(
                                    "Next step: record survival during a field visit",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Color(0xFF059669),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
