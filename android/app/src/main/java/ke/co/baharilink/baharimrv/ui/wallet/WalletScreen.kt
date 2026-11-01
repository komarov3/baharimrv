package ke.co.baharilink.baharimrv.ui.wallet

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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

@Composable
fun WalletScreen(viewModel: WalletViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item {
            ContextHeader(
                title = "Project wallet",
                subtitle = if (state.projectId.isBlank()) "Load a project to see server balances" else state.projectId,
                trailing = {
                    IconButton(onClick = viewModel::load, enabled = !state.working) {
                        Icon(Icons.Default.Refresh, contentDescription = "Reload wallet")
                    }
                },
            )
        }
        item {
            LedgerCard(contentPadding = 12.dp) {
                Text(
                    "Money arrives from tourism contributions and credit sales. Every figure below is the server's, never a total worked out on this device.",
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
                Button(onClick = viewModel::load, enabled = !state.working, modifier = Modifier.fillMaxWidth().height(48.dp)) {
                    Text("Load wallet")
                }
            }
        }
        state.message?.let { message ->
            item {
                LedgerCard(contentPadding = 12.dp) {
                    Text(message, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.primary)
                }
            }
        }
        state.error?.let { error ->
            item {
                LedgerCard(contentPadding = 12.dp) {
                    Text(error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                }
            }
        }

        when (val wallet = state.wallet) {
            LoadState.Loading -> item { LoadingState("Loading wallet…") }
            is LoadState.Error -> item { ErrorState(wallet.message, viewModel::load) }
            is LoadState.Success -> item { WalletContent(wallet.data, state, viewModel) }
            null -> Unit
        }

        when (val reconciliation = state.reconciliation) {
            is LoadState.Success -> item { ReconciliationCard(reconciliation.data) }
            else -> Unit
        }

        when (val documents = state.documents) {
            LoadState.Loading -> item { LoadingState("Loading documents…") }
            is LoadState.Error -> item { ErrorState(documents.message, viewModel::load) }
            is LoadState.Success -> {
                if (documents.data.isEmpty()) {
                    item {
                        Text(
                            "The server holds no documents for this project.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                } else {
                    item {
                        ExpandableSection("Project documents (${documents.data.size})") {
                            documents.data.forEach { document ->
                                DocumentRow(document, enabled = !state.working) { viewModel.download(document) }
                            }
                        }
                    }
                }
            }
            null -> Unit
        }
    }
}

@Composable
private fun WalletContent(wallet: WalletDto, state: WalletUiState, viewModel: WalletViewModel) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        LedgerCard(contentPadding = 12.dp) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Balances", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                StatusChip(chainNetworkLabel(wallet.mode), tone = stateTone(wallet.mode))
            }
            WalletRevenueStream("From tourism contributions", formatAmount(wallet.balances.tourismContributions))
            WalletRevenueStream("From carbon credit sales", formatAmount(wallet.balances.carbonCreditSales))
            val communityLabel = wallet.revenueRule?.communityPercent?.let { "Community share ($it%)" } ?: "Community share"
            WalletRevenueStream(
                communityLabel,
                formatAmount(wallet.balances.communityShare),
                amountColor = MaterialTheme.colorScheme.primary,
            )
            Text(
                "Last imported ${formatInstant(wallet.lastImportedAt)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        wallet.revenueRule?.let { rule ->
            LedgerCard(contentPadding = 12.dp) {
                Text("Revenue rule from the server", style = MaterialTheme.typography.titleSmall)
                Text(rule.label, style = MaterialTheme.typography.bodyMedium)
                KeyValue("Community share", "${rule.communityPercent}%")
                KeyValue("Rule ID", rule.id, monospace = true)
                Text(
                    "Effective ${formatInstant(rule.effectiveFrom)}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (rule.allocations.isNotEmpty()) {
                    MicroLabel("Allocations", modifier = Modifier.padding(top = 4.dp))
                    rule.allocations.forEach { allocation -> AllocationRow(allocation) }
                } else {
                    Text(
                        "No allocation breakdown returned for this rule.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        MicroLabel("Ledger entries · ${wallet.entries.size}")
        if (wallet.entries.isEmpty()) {
            Text(
                "The server returned no entries for this project.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        wallet.entries.forEach { entry -> EntryCard(entry) }
    }
}

@Composable
private fun AllocationRow(allocation: AllocationDto) {
    KeyValue("${allocation.label} (${allocation.percent}%)", formatAmount(allocation.amount))
}

@Composable
private fun EntryCard(entry: WalletEntryDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Column(Modifier.weight(1f)) {
                Text(entry.description, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(
                    entry.kind.replace('_', ' '),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Text(formatAmount(entry.amount), style = MaterialTheme.typography.titleSmall)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            StatusChip(entry.status, tone = stateTone(entry.status))
            Text(formatInstant(entry.occurredAt), style = MaterialTheme.typography.bodySmall)
        }
        entry.source?.let { Text("Source: $it", style = MaterialTheme.typography.bodySmall) }
        entry.reference?.let {
            Text(
                "Reference: $it",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun ReconciliationCard(reconciliation: ReconciliationDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Reconciliation", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(reconciliation.status, tone = stateTone(reconciliation.status))
        }
        Text(
            "Checked ${formatInstant(reconciliation.checkedAt)}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (reconciliation.issues.isEmpty()) {
            Text("No open issues.", style = MaterialTheme.typography.bodySmall)
        } else {
            reconciliation.issues.forEach { issue ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(issue.detail, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
                    StatusChip(issue.severity, tone = stateTone(issue.severity))
                }
            }
        }
    }
}

@Composable
private fun DocumentRow(document: DocumentDto, enabled: Boolean, onDownload: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Column(Modifier.weight(1f)) {
            Text(document.name, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val kind = document.kind?.let { "$it • " }.orEmpty()
            Text(
                "$kind version ${document.version ?: "unspecified"} • updated ${formatInstant(document.updatedAt)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        StatusChip(document.status, tone = stateTone(document.status))
        OutlinedButton(onClick = onDownload, enabled = enabled, modifier = Modifier.height(44.dp)) { Text("Save") }
    }
}
