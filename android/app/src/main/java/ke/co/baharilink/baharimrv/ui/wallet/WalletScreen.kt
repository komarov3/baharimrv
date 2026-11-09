package ke.co.baharilink.baharimrv.ui.wallet

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBalanceWallet
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.AllocationDto
import ke.co.baharilink.baharimrv.data.remote.DocumentDto
import ke.co.baharilink.baharimrv.data.remote.ReconciliationDto
import ke.co.baharilink.baharimrv.data.remote.WalletDto
import ke.co.baharilink.baharimrv.data.remote.WalletEntryDto
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.ExpandableSection
import ke.co.baharilink.baharimrv.ui.components.KeyValue
import ke.co.baharilink.baharimrv.ui.components.LedgerCard
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.MicroLabel
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.WalletRevenueStream
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel
import ke.co.baharilink.baharimrv.ui.components.formatAmount
import ke.co.baharilink.baharimrv.ui.components.formatInstant
import ke.co.baharilink.baharimrv.ui.components.stateTone

private val GreenPrimary = Color(0xFF059669)
private val GreenContainer = Color(0xFFE7F8F1)

@Composable
fun WalletScreen(viewModel: WalletViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {

        // ── header ────────────────────────────────────────────────────────
        item {
            ContextHeader(
                title = "Project wallet",
                subtitle = if (state.projectId.isBlank()) "Enter a project ID below"
                           else "${state.projectId} · read-only imports",
                trailing = {
                    IconButton(onClick = viewModel::load, enabled = !state.working) {
                        Icon(Icons.Default.Refresh, contentDescription = "Reload wallet")
                    }
                },
            )
        }

        // ── project-ID input ──────────────────────────────────────────────
        item {
            LedgerCard(contentPadding = 12.dp) {
                Text(
                    "Figures are imported from the BahariLink server. This screen never derives a balance locally.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedTextField(
                    value = state.projectId,
                    onValueChange = viewModel::updateProjectId,
                    label = { Text("Project ID") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Button(
                    onClick = viewModel::load,
                    enabled = !state.working,
                    modifier = Modifier.fillMaxWidth().height(48.dp),
                ) {
                    Text("Load wallet")
                }
            }
        }

        // ── inline messages ───────────────────────────────────────────────
        state.message?.let { msg ->
            item {
                LedgerCard(contentPadding = 12.dp) {
                    Text(msg, style = MaterialTheme.typography.bodySmall, color = GreenPrimary)
                }
            }
        }
        state.error?.let { err ->
            item {
                LedgerCard(contentPadding = 12.dp) {
                    Text(err, style = MaterialTheme.typography.bodySmall,
                         color = MaterialTheme.colorScheme.error)
                }
            }
        }

        // ── wallet content ────────────────────────────────────────────────
        when (val wallet = state.wallet) {
            LoadState.Loading -> item { LoadingState("Loading wallet…") }
            is LoadState.Error -> item { ErrorState(wallet.message, viewModel::load) }
            is LoadState.Success -> item { WalletContent(wallet.data, state, viewModel) }
            null -> Unit
        }

        // ── reconciliation ────────────────────────────────────────────────
        when (val reconciliation = state.reconciliation) {
            is LoadState.Success -> item { ReconciliationCard(reconciliation.data) }
            else -> Unit
        }

        // ── documents ─────────────────────────────────────────────────────
        when (val documents = state.documents) {
            LoadState.Loading -> item { LoadingState("Loading documents…") }
            is LoadState.Error -> item { ErrorState(documents.message, viewModel::load) }
            is LoadState.Success -> {
                if (documents.data.isNotEmpty()) {
                    item {
                        ExpandableSection("Financial evidence (${documents.data.size})") {
                            documents.data.forEach { doc ->
                                DocumentRow(doc, enabled = !state.working) { viewModel.download(doc) }
                            }
                        }
                    }
                }
            }
            null -> Unit
        }

        // ── footer note ───────────────────────────────────────────────────
        item {
            Text(
                "Financial evidence exports via proof workflows · no disbursements initiated from this app",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Wallet content — mirrors W1 layout: balance hero → streams → rule → ledger
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun WalletContent(wallet: WalletDto, state: WalletUiState, viewModel: WalletViewModel) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {

        // ── available-balance hero ────────────────────────────────────────
        Card(
            modifier = Modifier.fillMaxWidth(),
            colors = CardDefaults.cardColors(containerColor = GreenContainer),
        ) {
            Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Icon(
                            Icons.Default.AccountBalanceWallet,
                            contentDescription = null,
                            tint = GreenPrimary,
                            modifier = Modifier.size(20.dp),
                        )
                        Text(
                            "Available balance",
                            style = MaterialTheme.typography.labelMedium,
                            color = GreenPrimary,
                            fontWeight = FontWeight.SemiBold,
                        )
                    }
                    StatusChip(
                        text = chainNetworkLabel(wallet.mode),
                        tone = stateTone(wallet.mode),
                    )
                }

                // Community share is the headline figure per W1
                val community = wallet.balances.communityShare
                Text(
                    formatAmount(community),
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.Bold,
                    color = GreenPrimary,
                )

                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    // Cash-out is backend-controlled; button shown disabled per annotation 4
                    Button(
                        onClick = {},
                        enabled = false,
                        colors = ButtonDefaults.buttonColors(
                            disabledContainerColor = GreenPrimary.copy(alpha = 0.15f),
                            disabledContentColor = GreenPrimary,
                        ),
                        modifier = Modifier.weight(1f).height(40.dp),
                    ) {
                        Text("Cash out", style = MaterialTheme.typography.labelMedium)
                    }
                    OutlinedButton(
                        onClick = viewModel::load,
                        modifier = Modifier.weight(1f).height(40.dp),
                    ) {
                        Icon(Icons.Default.History, contentDescription = null, modifier = Modifier.size(16.dp))
                        Spacer(modifier = Modifier.width(4.dp))
                        Text("History", style = MaterialTheme.typography.labelMedium)
                    }
                }

                Text(
                    "Cash out is backend-controlled · BahariMRV displays imports only",
                    style = MaterialTheme.typography.bodySmall,
                    color = GreenPrimary.copy(alpha = 0.7f),
                )
            }
        }

        // ── revenue streams ───────────────────────────────────────────────
        LedgerCard(contentPadding = 12.dp) {
            Text(
                "Revenue streams",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold,
            )
            Spacer(Modifier.height(4.dp))

            RevenueRow(
                label = "Tourism",
                amount = formatAmount(wallet.balances.tourismContributions),
            )
            RevenueRow(
                label = "Carbon sales",
                amount = formatAmount(wallet.balances.carbonCreditSales),
            )
            HorizontalDivider(modifier = Modifier.padding(vertical = 4.dp))

            val communityLabel = wallet.revenueRule?.communityPercent
                ?.let { "Community share (${it}%)" } ?: "Community share"
            RevenueRow(
                label = communityLabel,
                amount = formatAmount(wallet.balances.communityShare),
                highlight = true,
            )

            wallet.lastImportedAt?.let { ts ->
                Text(
                    "Last import · ${formatInstant(ts)} · read-only",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }

        // ── revenue rule ──────────────────────────────────────────────────
        wallet.revenueRule?.let { rule ->
            LedgerCard(contentPadding = 12.dp) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Column {
                        Text(
                            "Revenue rule",
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            rule.label,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    // e.g. "60/25/15"
                    val split = rule.allocations.takeIf { it.isNotEmpty() }
                        ?.joinToString("/") { it.percent }
                        ?: "${rule.communityPercent}%"
                    Surface(
                        color = GreenContainer,
                        shape = MaterialTheme.shapes.small,
                    ) {
                        Text(
                            split,
                            style = MaterialTheme.typography.labelMedium,
                            fontWeight = FontWeight.Bold,
                            color = GreenPrimary,
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                        )
                    }
                }

                if (rule.allocations.isNotEmpty()) {
                    Spacer(Modifier.height(4.dp))
                    rule.allocations.forEach { a -> AllocationRow(a) }
                    Text(
                        "Community · project · platform",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }

                KeyValue("Rule ID", rule.id, monospace = true)
                rule.effectiveFrom?.let {
                    Text(
                        "Effective ${formatInstant(it)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        // ── audit ledger ──────────────────────────────────────────────────
        if (wallet.entries.isNotEmpty()) {
            MicroLabel("Audit ledger · ${wallet.entries.size} entries")
            wallet.entries.forEach { entry -> EntryCard(entry) }
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Revenue row — matches W1 two-column layout
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun RevenueRow(label: String, amount: String, highlight: Boolean = false) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.bodyMedium,
            color = if (highlight) GreenPrimary else MaterialTheme.colorScheme.onSurface,
            fontWeight = if (highlight) FontWeight.SemiBold else FontWeight.Normal,
        )
        Text(
            amount,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = if (highlight) FontWeight.Bold else FontWeight.Medium,
            color = if (highlight) GreenPrimary else MaterialTheme.colorScheme.onSurface,
        )
    }
}

@Composable
private fun AllocationRow(allocation: AllocationDto) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(
            "${allocation.label} (${allocation.percent}%)",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            formatAmount(allocation.amount),
            style = MaterialTheme.typography.bodySmall,
            fontWeight = FontWeight.Medium,
        )
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Ledger entry card — matches W1 audit ledger rows
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun EntryCard(entry: WalletEntryDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            // Colour-coded dot per entry kind
            val dotColor = when {
                entry.kind.contains("tourism", ignoreCase = true) -> Color(0xFF3B82F6)
                entry.kind.contains("carbon", ignoreCase = true) -> GreenPrimary
                entry.kind.contains("community", ignoreCase = true) -> Color(0xFF8B5CF6)
                else -> MaterialTheme.colorScheme.onSurfaceVariant
            }
            Surface(
                modifier = Modifier.size(8.dp),
                shape = androidx.compose.foundation.shape.CircleShape,
                color = dotColor,
            ) {}

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    entry.description,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    entry.kind.replace('_', ' '),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            // Amount — always positive in wallet view
            Text(
                "+${formatAmount(entry.amount)}",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold,
                color = GreenPrimary,
            )
        }

        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            StatusChip(entry.status, tone = stateTone(entry.status))
            Text(formatInstant(entry.occurredAt), style = MaterialTheme.typography.bodySmall)
        }

        entry.reference?.let {
            Text(
                it,
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Reconciliation card — W1 annotation 3: mismatches stay visible
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun ReconciliationCard(reconciliation: ReconciliationDto) {
    val isOk = reconciliation.status.equals("reconciled", ignoreCase = true) ||
               reconciliation.status.equals("ok", ignoreCase = true)
    LedgerCard(contentPadding = 12.dp) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    imageVector = if (isOk) Icons.Default.CheckCircle else Icons.Default.Warning,
                    contentDescription = null,
                    tint = if (isOk) GreenPrimary else MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(18.dp),
                )
                Text(
                    "Reconciliation",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            StatusChip(
                text = if (isOk) "✓ reconciled" else reconciliation.status,
                tone = if (isOk) ChipTone.POSITIVE else ChipTone.ERROR,
            )
        }

        reconciliation.checkedAt?.let { ts ->
            Text(
                "Checked ${formatInstant(ts)}${if (isOk) " · no mismatches" else ""}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        if (reconciliation.issues.isNotEmpty()) {
            Spacer(Modifier.height(4.dp))
            reconciliation.issues.forEach { issue ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text(
                        issue.detail,
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.weight(1f),
                    )
                    StatusChip(issue.severity, tone = stateTone(issue.severity))
                }
            }
        }
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Document row
// ─────────────────────────────────────────────────────────────────────────────

@Composable
private fun DocumentRow(document: DocumentDto, enabled: Boolean, onDownload: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                document.name,
                style = MaterialTheme.typography.bodyMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val kind = document.kind?.let { "$it · " }.orEmpty()
            Text(
                "${kind}v${document.version ?: "—"} · updated ${formatInstant(document.updatedAt)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        StatusChip(document.status, tone = stateTone(document.status))
        OutlinedButton(
            onClick = onDownload,
            enabled = enabled,
            modifier = Modifier.height(44.dp),
        ) { Text("Save") }
    }
}
