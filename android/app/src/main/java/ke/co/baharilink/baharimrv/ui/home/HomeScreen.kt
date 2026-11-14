package ke.co.baharilink.baharimrv.ui.home

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Circle
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.AlertDto
import ke.co.baharilink.baharimrv.data.remote.DashboardDto
import ke.co.baharilink.baharimrv.data.remote.MetricDto
import ke.co.baharilink.baharimrv.data.remote.NdviChangeDto
import ke.co.baharilink.baharimrv.data.remote.NdviDto
import ke.co.baharilink.baharimrv.data.remote.PlotDto
import ke.co.baharilink.baharimrv.data.remote.ProjectRowDto
import ke.co.baharilink.baharimrv.data.remote.WalletDto
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.ExpandableSection
import ke.co.baharilink.baharimrv.ui.components.KeyValue
import ke.co.baharilink.baharimrv.ui.components.LedgerCard
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.SectionHeader
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.WalletRevenueStream
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel
import ke.co.baharilink.baharimrv.ui.components.dashedBorder
import ke.co.baharilink.baharimrv.ui.components.formatAmount
import ke.co.baharilink.baharimrv.ui.components.formatInstant
import ke.co.baharilink.baharimrv.ui.components.stateTone
import ke.co.baharilink.baharimrv.ui.satellite.PlotMarker
import ke.co.baharilink.baharimrv.ui.satellite.ndviPlotMarker
import org.osmdroid.tileprovider.tilesource.TileSourceFactory
import org.osmdroid.util.BoundingBox
import org.osmdroid.util.GeoPoint
import org.osmdroid.views.MapView

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    onFieldVisit: () -> Unit,
    onVerification: () -> Unit,
    onProjects: () -> Unit,
    onPlanting: (String) -> Unit,
    onSatellite: (String) -> Unit,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val dashboardState = state.dashboard
    var showAlerts by remember { mutableStateOf(false) }

    LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (state.choosing || dashboardState !is LoadState.Success) {
            item { ProjectPicker(state = state, viewModel = viewModel, onCreateProject = onProjects) }
        } else {
            val dashboard = dashboardState.data
            item {
                ContextHeader(
                    title = dashboard.project.name,
                    subtitle = "Project dashboard · ${dashboard.project.siteCount} sites",
                    trailing = {
                        TextButton(onClick = viewModel::chooseProject, modifier = Modifier.height(48.dp)) { Text("Change") }
                        IconButton(onClick = { showAlerts = true }) {
                            Icon(Icons.Default.Notifications, contentDescription = "Open project alerts")
                        }
                    },
                )
            }
            item {
                DashboardContent(
                    dashboard = dashboard,
                    satelliteChange = state.satelliteChange,
                    wallet = state.wallet,
                    onRetryWallet = viewModel::loadWallet,
                    onFieldVisit = onFieldVisit,
                    onVerification = onVerification,
                    onProjects = onProjects,
                    onPlanting = onPlanting,
                    onSatellite = onSatellite,
                )
            }
        }
    }

    if (state.choosing && dashboardState is LoadState.Success) {
        ModalBottomSheet(onDismissRequest = viewModel::cancelChoosing) {
            LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                item { ProjectPicker(state = state, viewModel = viewModel, onCreateProject = onProjects) }
            }
        }
    }

    if (showAlerts && dashboardState is LoadState.Success) {
        ModalBottomSheet(onDismissRequest = { showAlerts = false }) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                ContextHeader("Project alerts", "Server-returned issues and next actions")
                AlertList(dashboardState.data.alerts)
            }
        }
    }
}

@Composable
private fun ProjectPicker(state: HomeUiState, viewModel: HomeViewModel, onCreateProject: () -> Unit) {
    LedgerCard(contentPadding = 12.dp) {
        Text("Project dashboard", style = MaterialTheme.typography.titleLarge)
        Text(
            "Your authorised projects load from the server. No sample impact or financial data is shown.",
            style = MaterialTheme.typography.bodyMedium,
        )
        OutlinedTextField(
            value = state.projectId,
            onValueChange = viewModel::updateProjectId,
            label = { Text("Project ID") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = viewModel::load, modifier = Modifier.weight(1f).height(48.dp)) { Text("Load by ID") }
            OutlinedButton(onClick = viewModel::loadProjects, modifier = Modifier.height(48.dp)) { Text("Refresh list") }
        }
    }
    when (val projects = state.projects) {
        LoadState.Loading -> LoadingState("Loading your projects…")
        is LoadState.Error -> ErrorState(projects.message, viewModel::loadProjects)
        is LoadState.Success -> {
            if (projects.data.rows.isEmpty()) {
                EmptyState("No projects returned", "This account is not attached to a conservation project yet.")
                Button(onClick = onCreateProject, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text("Create your first project") }
            } else {
                SectionHeader("Authorized projects")
                projects.data.summary?.let { summary ->
                    LedgerCard(contentPadding = 12.dp) {
                        KeyValue("Projects", summary.total.toString())
                        KeyValue("Active", summary.active.toString())
                        KeyValue("Awaiting verification", summary.pendingVerification.toString())
                        KeyValue("Achieved, unverified", summary.achievedUnverified.toString())
                    }
                }
                projects.data.rows.forEach { row -> ProjectRow(row) { viewModel.selectProject(row.id) } }
            }
        }
        null -> Unit
    }
    if (state.dashboard is LoadState.Loading) LoadingState("Loading project dashboard…")
    if (state.dashboard is LoadState.Error) ErrorState(state.dashboard.message, viewModel::load)
}

@Composable
private fun ProjectRow(row: ProjectRowDto, onSelect: () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onSelect),
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        border = CardDefaults.outlinedCardBorder(),
    ) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(7.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(row.name, style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                StatusChip(row.status, tone = stateTone(row.status))
            }
            Text(
                listOfNotNull(row.communityGroup, row.site, row.species, row.activityType).joinToString(" · ")
                    .ifBlank { "No site detail returned" },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                row.verificationState?.let { StatusChip(it, tone = stateTone(it)) }
                if (row.proofAvailable) StatusChip("Proof available", tone = ChipTone.POSITIVE)
                if (row.sandboxDemo) StatusChip("Sandbox", tone = ChipTone.WARNING)
            }
            row.integrationNote?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
        }
    }
}

@Composable
private fun DashboardContent(
    dashboard: DashboardDto,
    satelliteChange: LoadState<NdviChangeDto?>,
    wallet: LoadState<WalletDto>?,
    onRetryWallet: () -> Unit,
    onFieldVisit: () -> Unit,
    onVerification: () -> Unit,
    onProjects: () -> Unit,
    onPlanting: (String) -> Unit,
    onSatellite: (String) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        SatelliteHeadline(
            change = (satelliteChange as? LoadState.Success)?.data,
            legacyNdvi = dashboard.ndvi,
            onOpenSatellite = { onSatellite(dashboard.project.id) },
        )
        ProjectMap(dashboard.plots)

        if (dashboard.metrics.isEmpty()) {
            EmptyState("No metrics returned", "This dashboard response carries no measured project metrics yet.")
        } else {
            dashboard.metrics.take(4).chunked(2).forEach { pair ->
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    pair.forEach { metric -> CompactMetricCard(metric, Modifier.weight(1f)) }
                    if (pair.size == 1) Box(Modifier.weight(1f))
                }
            }
        }

        WalletSummary(wallet, onRetryWallet)

        Text("ALERTS & NEXT ACTIONS", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        AlertList(dashboard.alerts)

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = onFieldVisit, modifier = Modifier.weight(1f).height(48.dp)) { Text("+  Field visit") }
            OutlinedButton(onClick = onVerification, modifier = Modifier.weight(1f).height(48.dp)) { Text("Reports") }
            OutlinedButton(onClick = { onPlanting(dashboard.project.id) }, modifier = Modifier.weight(1f).height(48.dp)) { Text("Planting") }
            OutlinedButton(onClick = { onSatellite(dashboard.project.id) }, modifier = Modifier.weight(1f).height(48.dp)) { Text("Satellite") }
        }

        ExpandableSection("More metrics, methodology & project tools") {
            dashboard.metrics.drop(4).forEach { metric -> MetricCard(metric, Modifier.fillMaxWidth()) }
            Text("Integration: ${chainNetworkLabel(dashboard.chainMode)}", style = MaterialTheme.typography.bodySmall)
            dashboard.project.methodology?.let {
                Text("Methodology: $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            dashboard.project.methodologyPeriod?.let {
                Text("Methodology period: $it", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            OutlinedButton(onClick = onProjects, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text("Manage projects") }
        }
    }
}

@Composable
private fun WalletSummary(wallet: LoadState<WalletDto>?, onRetry: () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer),
        border = CardDefaults.outlinedCardBorder(),
    ) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(7.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("PROJECT WALLET", style = MaterialTheme.typography.labelSmall, modifier = Modifier.weight(1f))
                if (wallet is LoadState.Success) StatusChip(chainNetworkLabel(wallet.data.mode), tone = stateTone(wallet.data.mode))
            }
            when (wallet) {
                LoadState.Loading -> {
                    LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                    Text("Loading server balances", style = MaterialTheme.typography.bodySmall)
                }
                is LoadState.Error -> {
                    Text("Wallet balances unavailable", style = MaterialTheme.typography.titleSmall)
                    Text(wallet.message, style = MaterialTheme.typography.bodySmall)
                    TextButton(onClick = onRetry, modifier = Modifier.height(48.dp)) { Text("Retry wallet") }
                }
                is LoadState.Success -> {
                    val data = wallet.data
                    val emeraldGreen = Color(0xFF079B76)
                    WalletRevenueStream("From tourism contributions", formatAmount(data.balances.tourismContributions), amountColor = emeraldGreen)
                    WalletRevenueStream("From carbon credit sales", formatAmount(data.balances.carbonCreditSales), amountColor = emeraldGreen)
                    val communityLabel = data.revenueRule?.communityPercent?.let { "Community share ($it%)" } ?: "Community share"
                    WalletRevenueStream(communityLabel, formatAmount(data.balances.communityShare), amountColor = emeraldGreen, amountWeight = FontWeight.Bold)
                    Text("Server-reported balances · no local calculation", style = MaterialTheme.typography.bodySmall)
                }
                null -> Text("Wallet balances unavailable from this dashboard.", style = MaterialTheme.typography.bodySmall)
            }
        }
    }
}

@Composable
private fun AlertList(alerts: List<AlertDto>) {
    if (alerts.isEmpty()) {
        Text("No open alerts. The server returned no current project alerts.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    } else {
        alerts.forEach { alert -> AlertCard(alert) }
    }
}

@Composable
private fun AlertCard(alert: AlertDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(
                imageVector = Icons.Default.Circle,
                contentDescription = null,
                modifier = Modifier.size(10.dp),
                tint = when (alert.severity.uppercase()) {
                    "ERROR", "CRITICAL" -> MaterialTheme.colorScheme.error
                    "WARNING" -> MaterialTheme.colorScheme.tertiary
                    else -> MaterialTheme.colorScheme.primary
                },
            )
            Column(Modifier.weight(1f)) {
                Text(alert.title, style = MaterialTheme.typography.titleSmall)
                alert.detail?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            }
            StatusChip(alert.severity, tone = stateTone(alert.severity))
        }
    }
}

@Composable
private fun CompactMetricCard(metric: MetricDto, modifier: Modifier = Modifier) {
    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        border = CardDefaults.outlinedCardBorder(),
    ) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                metric.label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                listOfNotNull(metric.value, metric.unit).joinToString(" "),
                style = MaterialTheme.typography.displayLarge,
                fontWeight = FontWeight.Bold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            metric.status?.let { StatusChip(it, tone = stateTone(it)) }
        }
    }
}

@Composable
private fun MetricCard(metric: MetricDto, modifier: Modifier = Modifier) {
    LedgerCard(modifier = modifier, contentPadding = 12.dp) {
        Text(metric.label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(listOfNotNull(metric.value, metric.unit).joinToString(" "), style = MaterialTheme.typography.displayMedium)
        metric.status?.let { StatusChip(it, tone = stateTone(it)) }
        metric.methodologyPeriod?.let { Text("Period: $it", style = MaterialTheme.typography.bodySmall) }
    }
}

/**
 * The headline chip never calls a project "sandbox" just because no project
 * summary exists yet. Sandbox is claimed only when the server reports a sandbox
 * adapter mode; an uncomputed project reads "no imagery". Kept pure so the
 * labelling rule can be unit tested without a Compose runtime.
 */
internal data class SatelliteHeadlineState(
    val chipText: String,
    val isSandbox: Boolean,
    val liveChange: NdviChangeDto?,
)

internal fun satelliteHeadlineState(change: NdviChangeDto?, legacyNdvi: NdviDto?): SatelliteHeadlineState {
    val liveChange = change?.takeIf { it.quality != "unavailable" }
    val knownMode = change?.adapterMode ?: legacyNdvi?.adapterMode
    val isSandbox = knownMode.equals("sandbox", ignoreCase = true)
    val chipText = when {
        liveChange != null -> liveChange.adapterMode ?: "live"
        isSandbox -> "sandbox"
        else -> "no imagery"
    }
    return SatelliteHeadlineState(chipText, isSandbox, liveChange)
}

/**
 * Replaces the placeholder NDVI line the dashboard used to carry. A real
 * change figure from `/satellite/change` drives the headline; the legacy
 * [NdviDto] on the dashboard is the fall-back for deployments that have
 * not run a refresh yet. The card never presents a sandbox number as
 * live — adapter mode is always visible and a partial coverage figure is
 * labelled partial.
 */
@Composable
private fun SatelliteHeadline(
    change: NdviChangeDto?,
    legacyNdvi: NdviDto?,
    onOpenSatellite: () -> Unit,
) {
    val headline = satelliteHeadlineState(change, legacyNdvi)
    val isSandbox = headline.isSandbox
    val liveChange = headline.liveChange
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Satellite NDVI", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
            StatusChip(
                text = headline.chipText,
                tone = when {
                    liveChange != null -> stateTone(liveChange.adapterMode ?: "live")
                    isSandbox -> ChipTone.WARNING
                    else -> ChipTone.NEUTRAL
                },
            )
        }
        when {
            liveChange != null -> {
                Spacer(Modifier.height(8.dp))
                liveChange.meanNdvi?.let {
                    KeyValue("Mean NDVI", "%.3f".format(it))
                }
                liveChange.baselineMeanNdvi?.let {
                    KeyValue("Baseline mean", "%.3f".format(it))
                }
                liveChange.changeAbs?.let { delta ->
                    val sign = if (delta >= 0) "+" else ""
                    val tone = when {
                        delta > 0.02 -> Color(0xFF059669)
                        delta < -0.02 -> MaterialTheme.colorScheme.error
                        else -> MaterialTheme.colorScheme.onSurface
                    }
                    KeyValue("Change", "$sign${"%.3f".format(delta)}", valueColor = tone)
                }
                KeyValue(
                    "Coverage",
                    "${liveChange.coveragePct.toInt()}% over ${liveChange.plotCount} plots",
                )
                if (liveChange.coveragePct < 60.0 && liveChange.plotCount > 0) {
                    Spacer(Modifier.height(6.dp))
                    Text(
                        "Partial coverage. Treat the headline figure as indicative until a clearer pass sharpens it.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.tertiary,
                    )
                }
                liveChange.currentAcquiredAt?.let { acquired ->
                    Text(
                        "Scene acquired ${formatInstant(acquired)}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                liveChange.resultHash?.let {
                    Text(
                        "Result hash: ${it.take(12)}…",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            legacyNdvi != null -> {
                Spacer(Modifier.height(6.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    StatusChip("NDVI: ${legacyNdvi.status}", tone = stateTone(legacyNdvi.status))
                    listOfNotNull(legacyNdvi.source, legacyNdvi.acquiredAt?.let(::formatInstant))
                        .joinToString(" · ")
                        .takeIf(String::isNotBlank)
                        ?.let { detail ->
                            Text(detail, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                }
                Text(
                    "Tap Satellite below to compute the project-level change figure.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            else -> {
                Spacer(Modifier.height(6.dp))
                Text(
                    "No imagery computed yet. Tap Satellite below to start a refresh.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
        Spacer(Modifier.height(8.dp))
        OutlinedButton(onClick = onOpenSatellite, modifier = Modifier.fillMaxWidth().height(44.dp)) {
            Text("Open satellite screen")
        }
    }
}

@Composable
private fun ProjectMap(plots: List<PlotDto>) {
    Column(
        modifier = Modifier.fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .dashedBorder(MaterialTheme.colorScheme.outline, 12.dp)
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Project plots", style = MaterialTheme.typography.labelLarge, modifier = Modifier.weight(1f))
            Text("${plots.size} plots", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        if (plots.isEmpty()) {
            Text("No GPS plots returned for this dashboard.", style = MaterialTheme.typography.bodySmall)
            return@Column
        }
        var selected by remember { mutableStateOf<PlotDto?>(null) }
        val context = LocalContext.current
        key(plots) {
            val mapView = remember {
                MapView(context).apply {
                    setTileSource(TileSourceFactory.MAPNIK)
                    setMultiTouchControls(true)
                    setBuiltInZoomControls(false)
                    controller.setZoom(if (plots.size == 1) 15.0 else 13.0)
                    controller.setCenter(GeoPoint(plots.first().latitude, plots.first().longitude))
                    plots.forEach { plot ->
                        val marker = ndviPlotMarker(
                            this,
                            PlotMarker(plot.id, plot.label, plot.latitude, plot.longitude, plot.ndvi),
                            plot.ndvi?.let { "NDVI %.3f".format(it) } ?: "NDVI unavailable",
                        )
                        marker.setOnMarkerClickListener { _, _ ->
                            selected = plot
                            true
                        }
                        overlays.add(marker)
                    }
                    if (plots.size > 1) {
                        val box = BoundingBox(
                            plots.maxOf { it.latitude }, plots.maxOf { it.longitude },
                            plots.minOf { it.latitude }, plots.minOf { it.longitude },
                        )
                        addOnFirstLayoutListener { _, _, _, _, _ -> zoomToBoundingBox(box, false, 48) }
                    }
                }
            }
            DisposableEffect(mapView) {
                mapView.onResume()
                onDispose {
                    mapView.onPause()
                    mapView.onDetach()
                }
            }
            AndroidView(factory = { mapView }, modifier = Modifier.fillMaxWidth().height(160.dp).clipToBounds())
        }
        Text("Tap a plot for returned NDVI, GPS, area and status.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        selected?.let { plot ->
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(plot.label, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                StatusChip(plot.status, tone = stateTone(plot.status))
            }
            Text("NDVI: ${plot.ndvi?.toString() ?: "Unavailable"} · GPS: ${plot.latitude}, ${plot.longitude}", style = MaterialTheme.typography.bodySmall)
            plot.areaHectares?.let { Text("Area: $it ha", style = MaterialTheme.typography.bodySmall) }
            Text("Plot ID: ${plot.id}", style = MaterialTheme.typography.bodySmall)
        }
    }
}
