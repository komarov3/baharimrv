package ke.co.baharilink.baharimrv.ui.credits

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.ListingDto
import ke.co.baharilink.baharimrv.data.remote.PurchaseResultDto
import ke.co.baharilink.baharimrv.data.remote.RetirementResultDto
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
import ke.co.baharilink.baharimrv.ui.components.NavyLedgerCard
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.VerifiedBadge
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel
import ke.co.baharilink.baharimrv.ui.components.formatAmount
import ke.co.baharilink.baharimrv.ui.components.formatInstant
import ke.co.baharilink.baharimrv.ui.components.formatPrice
import ke.co.baharilink.baharimrv.ui.components.stateTone

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun CreditsScreen(viewModel: CreditsViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var showPurchaseSheet by remember { mutableStateOf(false) }
    var showRetirementSheet by remember { mutableStateOf(false) }

    LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item {
            ContextHeader(
                title = "Blue carbon market",
                subtitle = "Buyer view · listings returned by the registry server",
                trailing = {
                    IconButton(onClick = viewModel::load) {
                        Icon(Icons.Default.Refresh, contentDescription = "Reload marketplace listings")
                    }
                },
            )
        }

        state.error?.let { error ->
            item {
                LedgerCard(contentPadding = 12.dp) {
                    Text(error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
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

        state.purchase?.let { item { PurchaseReceipt(it) } }
        state.retirement?.let { item { RetirementCertificate(it) } }

        when (val listings = state.listings) {
            LoadState.Loading -> item { LoadingState("Loading marketplace…") }
            is LoadState.Error -> item { ErrorState(listings.message, viewModel::load) }
            is LoadState.Success -> {
                if (listings.data.isEmpty()) {
                    item { EmptyState("No batches listed", "The server returned no credits available for sale.") }
                } else {
                    item { MicroLabel("Available batches · ${listings.data.size}") }
                    items(listings.data, key = { it.id }) { listing ->
                        ListingCard(
                            listing = listing,
                            selected = state.selectedListing?.id == listing.id,
                            onSelect = {
                                viewModel.select(listing)
                                showPurchaseSheet = true
                            },
                        )
                    }
                }
            }
            null -> item { LoadingState("Loading marketplace…") }
        }

        item {
            OutlinedButton(
                onClick = { showRetirementSheet = true },
                modifier = Modifier.fillMaxWidth().height(48.dp),
            ) { Text("Retire credits from a batch") }
        }

        item {
            ExpandableSection("How an order is authorised") {
                Text(
                    "A purchase moves real money. This device sends the order with a single-use confirmation ticket minted from your password; the server decides whether it is accepted.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    "Prices, quantities and totals are shown exactly as the server returns them. Nothing is added up on this device.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }

    val listing = state.selectedListing
    if (showPurchaseSheet && listing != null) {
        ModalBottomSheet(onDismissRequest = { showPurchaseSheet = false }) {
            SheetColumn {
                ContextHeader(
                    title = "Buy credits",
                    subtitle = "${listing.projectName} · ${listing.vintage}",
                    trailing = { TextButton(onClick = { showPurchaseSheet = false }, modifier = Modifier.height(48.dp)) { Text("Close") } },
                )
                LedgerCard(contentPadding = 12.dp) {
                    KeyValue("Price / tonne", formatPrice(listing.pricePerTonne))
                    KeyValue("Available", "${listing.availableTonnes} t")
                    KeyValue("Product", listing.productType)
                    KeyValue("Registry", "${listing.registry.name} · ${listing.registry.status}")
                    KeyValue("Batch", listing.batchId, monospace = true)
                    StatusChip(chainNetworkLabel(listing.mode), tone = stateTone(listing.mode))
                }
                OutlinedTextField(
                    value = state.quantityTonnes,
                    onValueChange = { value -> viewModel.update { it.copy(quantityTonnes = value) } },
                    label = { Text("Tonnes") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = state.password,
                    onValueChange = { value -> viewModel.update { it.copy(password = value) } },
                    label = { Text("Password") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
                Button(
                    onClick = {
                        viewModel.purchase()
                        showPurchaseSheet = false
                    },
                    enabled = !state.working && state.quantityTonnes.isNotBlank() && state.password.isNotBlank(),
                    modifier = Modifier.fillMaxWidth().height(48.dp),
                ) { Text(if (state.working) "Sending order…" else "Place order") }
                Text(
                    "The order needs a single-use confirmation ticket minted from your password. The server, not this device, decides if it is accepted.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }

    if (showRetirementSheet) {
        ModalBottomSheet(onDismissRequest = { showRetirementSheet = false }) {
            SheetColumn {
                ContextHeader(
                    title = "Retire credits",
                    subtitle = "Recorded against the batch the server holds",
                    trailing = { TextButton(onClick = { showRetirementSheet = false }, modifier = Modifier.height(48.dp)) { Text("Close") } },
                )
                OutlinedTextField(
                    value = state.retirementBatchId,
                    onValueChange = { value -> viewModel.update { it.copy(retirementBatchId = value) } },
                    label = { Text("Credit batch ID") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = state.retirementQuantity,
                    onValueChange = { value -> viewModel.update { it.copy(retirementQuantity = value) } },
                    label = { Text("Tonnes") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = state.retirementReason,
                    onValueChange = { value -> viewModel.update { it.copy(retirementReason = value) } },
                    label = { Text("Reason for retirement") },
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = state.retirementPassword,
                    onValueChange = { value -> viewModel.update { it.copy(retirementPassword = value) } },
                    label = { Text("Password") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
                Button(
                    onClick = {
                        viewModel.retire()
                        showRetirementSheet = false
                    },
                    enabled = !state.working && state.retirementBatchId.isNotBlank() && state.retirementQuantity.isNotBlank() &&
                        state.retirementReason.isNotBlank() && state.retirementPassword.isNotBlank(),
                    modifier = Modifier.fillMaxWidth().height(48.dp),
                ) { Text(if (state.working) "Sending…" else "Retire") }
            }
        }
    }
}

@Composable
private fun SheetColumn(content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .imePadding()
            .padding(horizontal = 16.dp)
            .padding(bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
        content = content,
    )
}

@Composable
private fun NavyKeyValue(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top,
    ) {
        Text(label, style = MaterialTheme.typography.bodySmall, color = NavyMuted, modifier = Modifier.weight(1f))
        Spacer(Modifier.size(8.dp))
        Text(
            value,
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
            textAlign = TextAlign.End,
            modifier = Modifier.weight(1f),
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ListingCard(listing: ListingDto, selected: Boolean, onSelect: () -> Unit) {
    LedgerCard(
        modifier = Modifier.clickable(onClick = onSelect),
        contentPadding = 12.dp,
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(
                listing.projectName,
                style = MaterialTheme.typography.titleSmall,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val registryVerified = listing.registry.status.uppercase().contains("VERIFIED") ||
                listing.registry.status.uppercase().contains("ACTIVE")
            if (registryVerified) VerifiedBadge()
            StatusChip(listing.registry.name, tone = ChipTone.NEUTRAL)
        }
        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            StatusChip("${listing.availableTonnes} t available", tone = ChipTone.NEUTRAL)
            StatusChip(
                text = listing.productType,
                tone = if (listing.productType.uppercase().contains("PRE")) ChipTone.WARNING else ChipTone.POSITIVE,
            )
            StatusChip(listing.registry.status, tone = stateTone(listing.registry.status))
        }
        if (listing.coBenefits.isNotEmpty()) {
            Text(
                "Co-benefits: ${listing.coBenefits.joinToString(" · ")}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Bottom,
        ) {
            Column {
                Text(
                    "price / tCO₂e",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    formatPrice(listing.pricePerTonne) + " 📦",
                    style = MaterialTheme.typography.displayLarge,
                    color = Color(0xFF079B76),
                    fontWeight = androidx.compose.ui.text.font.FontWeight.Bold,
                )
            }
            Button(onClick = onSelect, modifier = Modifier.height(48.dp)) { Text("Buy credits") }
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(
                "${listing.vintage} · batch ${listing.batchId}",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            StatusChip(chainNetworkLabel(listing.mode), tone = stateTone(listing.mode))
        }
        if (selected) Text("Selected listing", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.primary)
    }
}

@Composable
private fun PurchaseReceipt(result: PurchaseResultDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Order response", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(result.status, tone = stateTone(result.status))
        }
        KeyValue("Integration", chainNetworkLabel(result.mode))
        KeyValue("Quantity", "${result.quantityTonnes} t")
        KeyValue("Total", formatAmount(result.total))
        KeyValue("Batch", result.batchId, monospace = true)
        result.custodyReference?.let { KeyValue("Custody reference", it, monospace = true) }
        result.settlementReference?.let { KeyValue("Settlement reference", it, monospace = true) }
        result.detail?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
    }
}

@Composable
private fun RetirementCertificate(result: RetirementResultDto) {
    NavyLedgerCard {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Retirement certificate", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(result.status, tone = stateTone(result.status))
        }
        Text(
            "${result.quantityTonnes} tCO₂e retired",
            style = MaterialTheme.typography.headlineSmall,
            color = NavyEmerald,
        )
        NavyKeyValue("Batch", result.batchId)
        result.registrySerialReference?.let { NavyKeyValue("Registry serial", it) }
        result.registryCertificateReference?.let { NavyKeyValue("Registry certificate", it) }
        result.chainReference?.let { NavyKeyValue("Chain reference", it) }
        result.chainNetwork?.let { NavyKeyValue("Ledger", chainNetworkLabel(it)) }
        result.retiredAt?.let { NavyKeyValue("Retired", formatInstant(it)) }
        NavyKeyValue("Integration", chainNetworkLabel(result.mode))
        result.explorerUrl?.let {
            Text(
                "Explorer: $it",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
        result.detail?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
        Text(
            "Revenue split (contract-enforced)",
            style = MaterialTheme.typography.labelSmall,
            color = NavyMuted,
        )
        Text(
            "community 60 · project 25 · platform 15 🔒",
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
        )
    }
}

private val NavyEmerald = Color(0xFF71DBB9)
private val NavyMuted = Color(0xFFB4C9DA)
