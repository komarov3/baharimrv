package ke.co.baharilink.baharimrv.ui.projects

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Card
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
import androidx.compose.material3.MenuAnchorType
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.CreatePlotRequestDto
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.SectionHeader
import ke.co.baharilink.baharimrv.ui.components.StatusChip

@Composable
fun ProjectsScreen(
    onProjectClick: (String) -> Unit,
    viewModel: ProjectsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var showCreate by remember { mutableStateOf(false) }

    LazyColumn(
        modifier = Modifier.padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            SectionHeader(title = "Projects", action = {
                TextButton(onClick = { showCreate = !showCreate }) {
                    Text(if (showCreate) "Cancel" else "Create project")
                }
            })
        }

        if (showCreate) {
            item { CreateProjectForm(state = state, viewModel = viewModel, onDone = { showCreate = false }) }
        }

        state.createdProjectId?.let { id ->
            item {
                Text("Project created: $id", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.primary)
            }
        }
        state.createError?.let { message ->
            item { Text(message, color = MaterialTheme.colorScheme.error) }
        }

        item {
            when (val projects = state.projects) {
                is LoadState.Loading -> LoadingState("Loading projects…")
                is LoadState.Error -> ErrorState(projects.message, onRetry = viewModel::load)
                is LoadState.Success -> {
                    if (projects.data.isEmpty()) {
                        EmptyState("No projects yet", "Create your first conservation project to begin monitoring.")
                    }
                }
            }
        }

        val projects = (state.projects as? LoadState.Success)?.data ?: emptyList()
        items(projects) { project ->
             Card(modifier = Modifier.fillMaxWidth().clickable { onProjectClick(project.id) }) {
                Column(modifier = Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(project.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                        StatusChip(project.status)
                    }
                    Text("Site: ${project.site ?: "—"}", style = MaterialTheme.typography.bodySmall)
                    Text("Species: ${project.species ?: "—"}", style = MaterialTheme.typography.bodySmall)
                    Text("Manager: ${project.managerName ?: "—"}", style = MaterialTheme.typography.bodySmall)
                    if (project.sandboxDemo) {
                        Text("Sandbox demo project", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.tertiary)
                    }
                }
            }
        }
    }
}

@Composable
@OptIn(ExperimentalMaterial3Api::class)
private fun CreateProjectForm(
    state: ProjectsUiState,
    viewModel: ProjectsViewModel,
    onDone: () -> Unit,
) {
    var name by remember { mutableStateOf("") }
    var conservationType by remember { mutableStateOf("mangrove") }
    var communityGroup by remember { mutableStateOf("") }
    var species by remember { mutableStateOf("Rhizophora mucronata") }
    var communityShare by remember { mutableStateOf("6000") }
    var sites by remember { mutableStateOf("") }
    var plotCode by remember { mutableStateOf("") }
    var plotSite by remember { mutableStateOf("") }
    var area by remember { mutableStateOf("") }
    var latitude by remember { mutableStateOf("") }
    var longitude by remember { mutableStateOf("") }
    var methodologyId by remember { mutableStateOf(state.methodologies.firstOrNull()?.id ?: "") }

    LaunchedEffect(conservationType) {
        species = when (conservationType) {
            "seagrass" -> "Thalassia hemprichii"
            "reef" -> "Acropora spp."
            else -> "Rhizophora mucronata"
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Project name") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = communityGroup, onValueChange = { communityGroup = it }, label = { Text("Community group") }, modifier = Modifier.fillMaxWidth())
        var conservationExpanded by remember { mutableStateOf(false) }
        val conservationOptions = listOf("mangrove", "seagrass", "reef")
        ExposedDropdownMenuBox(
            expanded = conservationExpanded,
            onExpandedChange = { conservationExpanded = !conservationExpanded },
        ) {
            OutlinedTextField(
                value = conservationType,
                onValueChange = {},
                readOnly = true,
                label = { Text("Type of conservation") },
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = conservationExpanded) },
                modifier = Modifier.menuAnchor(MenuAnchorType.PrimaryNotEditable, true).fillMaxWidth(),
            )
            ExposedDropdownMenu(expanded = conservationExpanded, onDismissRequest = { conservationExpanded = false }) {
                conservationOptions.forEach { option ->
                    DropdownMenuItem(text = { Text(option) }, onClick = {
                        conservationType = option
                        conservationExpanded = false
                    })
                }
            }
        }
        val speciesOptions = when (conservationType) {
            "seagrass" -> listOf("Thalassia hemprichii", "Cymodocea rotundata", "Enhalus acoroides", "Halodule uninervis")
            "reef" -> listOf("Acropora spp.", "Porites spp.", "Pocillopora spp.", "Mixed hard coral")
            else -> listOf("Rhizophora mucronata", "Avicennia marina", "Sonneratia alba", "Bruguiera gymnorrhiza")
        }
        var speciesExpanded by remember { mutableStateOf(false) }
        ExposedDropdownMenuBox(
            expanded = speciesExpanded,
            onExpandedChange = { speciesExpanded = !speciesExpanded },
        ) {
            OutlinedTextField(
                value = species,
                onValueChange = {},
                readOnly = true,
                label = { Text("Species") },
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = speciesExpanded) },
                modifier = Modifier.menuAnchor(MenuAnchorType.PrimaryNotEditable, true).fillMaxWidth(),
            )
            ExposedDropdownMenu(expanded = speciesExpanded, onDismissRequest = { speciesExpanded = false }) {
                speciesOptions.forEach { option ->
                    DropdownMenuItem(text = { Text(option) }, onClick = {
                        species = option
                        speciesExpanded = false
                    })
                }
            }
        }
        OutlinedTextField(value = communityShare, onValueChange = { communityShare = it }, label = { Text("Community share (basis points)") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = sites, onValueChange = { sites = it }, label = { Text("Sites (comma separated)") }, placeholder = { Text("e.g. Mikoko Park, Mtwapa Creek") }, supportingText = { Text("Enter one or more site names separated by commas; the first plot will use the first site.") }, modifier = Modifier.fillMaxWidth())
        var methodologyExpanded by remember { mutableStateOf(false) }
        ExposedDropdownMenuBox(
            expanded = methodologyExpanded,
            onExpandedChange = { methodologyExpanded = !methodologyExpanded },
        ) {
            OutlinedTextField(
                value = state.methodologies.firstOrNull { it.id == methodologyId }?.name ?: methodologyId,
                onValueChange = {},
                readOnly = true,
                label = { Text("Methodology") },
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = methodologyExpanded) },
                modifier = Modifier.menuAnchor(MenuAnchorType.PrimaryNotEditable, true).fillMaxWidth(),
            )
            ExposedDropdownMenu(expanded = methodologyExpanded, onDismissRequest = { methodologyExpanded = false }) {
                state.methodologies.forEach { methodology ->
                    DropdownMenuItem(text = { Text("${methodology.name} ${methodology.version}") }, onClick = {
                        methodologyId = methodology.id
                        methodologyExpanded = false
                    })
                }
            }
        }
        OutlinedTextField(value = plotCode, onValueChange = { plotCode = it }, label = { Text("First plot code") }, placeholder = { Text("e.g. P1") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = plotSite, onValueChange = { plotSite = it }, label = { Text("First plot site name") }, placeholder = { Text("Defaults to first site above") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = area, onValueChange = { area = it }, label = { Text("Area (ha)") }, placeholder = { Text("e.g. 2.5") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = latitude, onValueChange = { latitude = it }, label = { Text("Latitude") }, placeholder = { Text("e.g. -4.0532") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = longitude, onValueChange = { longitude = it }, label = { Text("Longitude") }, placeholder = { Text("e.g. 39.6741") }, modifier = Modifier.fillMaxWidth())

        Button(
            onClick = {
                val siteList = sites.split(',').map { it.trim() }.filter { it.isNotBlank() }
                val firstSite = siteList.firstOrNull() ?: "Site 1"
                val plot = CreatePlotRequestDto(
                    siteName = plotSite.trim().ifBlank { firstSite },
                    code = plotCode.trim().ifBlank { "P1" },
                    latitude = latitude.toDoubleOrNull() ?: 0.0,
                    longitude = longitude.toDoubleOrNull() ?: 0.0,
                    areaHectares = area.trim().ifBlank { "1.0" },
                )
                viewModel.createProject(
                    name = name,
                    methodologyId = methodologyId,
                    conservationType = conservationType,
                    communityGroup = communityGroup,
                    species = species,
                    communityShareBasisPoints = communityShare.toIntOrNull() ?: 6000,
                    sites = if (siteList.isEmpty()) listOf("Site 1") else siteList,
                    plots = listOf(plot),
                )
            },
            enabled = !state.creating,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(if (state.creating) "Creating…" else "Create project")
        }
        if (state.createdProjectId != null) {
            TextButton(onClick = onDone) { Text("Done") }
        }
    }
}
