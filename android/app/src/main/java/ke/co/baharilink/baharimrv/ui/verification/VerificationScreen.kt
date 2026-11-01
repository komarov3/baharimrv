package ke.co.baharilink.baharimrv.ui.verification

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Description
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.DocumentDto
import ke.co.baharilink.baharimrv.data.remote.ProofDto
import ke.co.baharilink.baharimrv.data.remote.ProvenanceDto
import ke.co.baharilink.baharimrv.data.remote.ProvenanceEventDto
import ke.co.baharilink.baharimrv.data.remote.QueueItemDto
import ke.co.baharilink.baharimrv.data.remote.VerificationStageDto
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.ExpandableSection
import ke.co.baharilink.baharimrv.ui.components.LedgerCard
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.MicroLabel
import ke.co.baharilink.baharimrv.ui.components.StageNumberCircle
import ke.co.baharilink.baharimrv.ui.components.StatusChip
import ke.co.baharilink.baharimrv.ui.components.chainNetworkLabel
import ke.co.baharilink.baharimrv.ui.components.formatInstant
import ke.co.baharilink.baharimrv.ui.components.stateTone

@Composable
fun VerificationScreen(
    onBack: () -> Unit,
    viewModel: VerificationViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        item {
            ContextHeader(
                title = "Verification pipeline",
                subtitle = if (state.activityId.isBlank()) {
                    "Queue · activities awaiting an independent decision"
                } else {
                    "Activity ${state.activityId}"
                },
                leading = {
                    IconButton(onClick = if (state.activityId.isBlank()) onBack else viewModel::showQueue) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
                trailing = {
                    if (state.activityId.isNotBlank()) {
                        TextButton(onClick = viewModel::showQueue, modifier = Modifier.height(48.dp)) { Text("All requests") }
                    }
                },
            )
        }

        if (state.activityId.isBlank()) {
            item { ActivityLookup(state, viewModel) }
            when (val queue = state.queue) {
                LoadState.Loading -> item { LoadingState("Loading the verifier queue…") }
                is LoadState.Error -> item { ErrorState(queue.message, viewModel::loadQueue) }
                is LoadState.Success -> {
                    if (queue.data.isEmpty()) {
                        item { EmptyState("Nothing waiting", "The server returned no open verification requests.") }
                    } else {
                        item { MicroLabel("Open requests · ${queue.data.size}") }
                        items(queue.data, key = { it.id }) { item ->
                            QueueRow(item) { viewModel.openActivity(item.activityId) }
                        }
                    }
                }
                null -> item { LoadingState("Loading the verifier queue…") }
            }
        } else {
            state.message?.let { item { Notice(it) } }
            state.error?.let { item { Notice(it, isError = true) } }
            when (val provenance = state.provenance) {
                LoadState.Loading -> item { LoadingState("Loading provenance…") }
                is LoadState.Error -> item { ErrorState(provenance.message, viewModel::load) }
                is LoadState.Success -> item { ProvenanceContent(provenance.data, state, viewModel) }
                null -> item { EmptyState("No activity loaded", "Field, audit, and provenance events will appear here.") }
            }
        }
    }
}

@Composable
private fun ActivityLookup(state: VerificationUiState, viewModel: VerificationViewModel) {
    LedgerCard(contentPadding = 12.dp) {
        Text("Load an activity", style = MaterialTheme.typography.titleSmall)
        Text(
            "Opens the provenance the server publishes for one activity.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        OutlinedTextField(
            value = state.activityId,
            onValueChange = viewModel::updateActivityId,
            label = { Text("Activity ID") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Button(onClick = viewModel::load, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text("Load timeline") }
    }
}

@Composable
private fun QueueRow(item: QueueItemDto, onOpen: () -> Unit) {
    LedgerCard(modifier = Modifier.clickable(onClick = onOpen), contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(item.projectName, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(item.status, tone = stateTone(item.status))
        }
        Text(
            "${item.plotCode ?: "No plot code"} • ${item.recordsCount} records • ${item.evidenceCount} evidence",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            "Requested ${formatInstant(item.requestedAt)} • methodology ${item.methodologyVersion ?: "unspecified"}",
            style = MaterialTheme.typography.bodySmall,
        )
        if (item.unsignedRecords > 0) {
            Text(
                "${item.unsignedRecords} record(s) carry an unsigned identity attestation.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
    }
}

@Composable
private fun ProvenanceContent(
    provenance: ProvenanceDto,
    state: VerificationUiState,
    viewModel: VerificationViewModel,
) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(
                provenance.activity.projectName,
                style = MaterialTheme.typography.titleLarge,
                modifier = Modifier.weight(1f),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            StatusChip(provenance.activity.status, tone = stateTone(provenance.activity.status))
        }
        Text(
            "Completion claims and independent verification stay separate events. Nothing here is inferred from missing data.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        MicroLabel("Pipeline")
        if (provenance.stages.isEmpty()) {
            EmptyState("No stages returned", "The server published no pipeline for this activity.")
        } else {
            provenance.stages.forEachIndexed { index, stage ->
                StageStep(stage = stage, number = (index + 1).toString(), isLast = index == provenance.stages.lastIndex)
            }
        }

        MicroLabel("Documents")
        if (provenance.documents.isEmpty()) {
            Text(
                "No documents returned.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        } else {
            provenance.documents.forEach { document -> DocumentRow(document) }
        }

        ExpandableSection("Activity actions") { ActivityActions(state, viewModel) }
        ExpandableSection("Provenance timeline (${provenance.events.size})") {
            if (provenance.events.isEmpty()) {
                Text(
                    "No provenance events. No ledger confirmation is assumed until the server publishes one.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                provenance.events.forEach { event -> EventRow(event) }
            }
        }
        ExpandableSection("Chain-of-custody proof") { ProofPanel(state, viewModel) }
        if (state.requestId != null) {
            ExpandableSection("Verifier decision") { DecisionPanel(state, viewModel) }
        }
    }
}

/** Wireframe C3: numbered circles joined by a rail, stage detail beside each one. */
@Composable
private fun StageStep(stage: VerificationStageDto, number: String, isLast: Boolean) {
    val tone = stateTone(stage.status)
    Row(modifier = Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
        Column(
            modifier = Modifier.width(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            StageNumberCircle(number = number, isCompleted = tone == ChipTone.POSITIVE)
            if (!isLast) {
                Column(
                    modifier = Modifier
                        .width(2.dp)
                        .fillMaxHeight()
                        .background(MaterialTheme.colorScheme.outlineVariant),
                ) {}
            }
        }
        Column(
            modifier = Modifier
                .weight(1f)
                .padding(start = 12.dp, bottom = if (isLast) 0.dp else 14.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(stage.label, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                StatusChip(stage.status, tone = tone)
            }
            Text(
                listOfNotNull(stage.owner?.let { "Owner: $it" } ?: "Owner: unassigned", formatInstant(stage.timestamp))
                    .joinToString(" • "),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            stage.exception?.let {
                Text("Exception: $it", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            stage.nextAction?.let {
                Text("Next action: $it", style = MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Medium)
            }
            if (stage.evidenceReferences.isNotEmpty()) {
                ExpandableSection("Evidence (${stage.evidenceReferences.size})") {
                    stage.evidenceReferences.forEach { reference ->
                        Text(
                            reference,
                            style = MaterialTheme.typography.bodySmall,
                            fontFamily = FontFamily.Monospace,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DocumentRow(document: DocumentDto) {
    val tone = stateTone(document.status)
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Icon(
            Icons.Default.Description,
            contentDescription = null,
            modifier = Modifier.size(16.dp),
            tint = when (tone) {
                ChipTone.POSITIVE -> MaterialTheme.colorScheme.primary
                ChipTone.WARNING -> MaterialTheme.colorScheme.tertiary
                ChipTone.ERROR -> MaterialTheme.colorScheme.error
                ChipTone.NEUTRAL -> MaterialTheme.colorScheme.onSurfaceVariant
            },
        )
        Column(Modifier.weight(1f)) {
            Text(document.name, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val kind = document.kind?.let { "$it • " }.orEmpty()
            Text(
                "$kind version ${document.version ?: "unspecified"} • updated ${formatInstant(document.updatedAt)}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        StatusChip(document.status, tone = tone)
    }
}

@Composable
private fun ActivityActions(state: VerificationUiState, viewModel: VerificationViewModel) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        OutlinedTextField(
            value = state.disputeReason,
            onValueChange = { value -> viewModel.update { it.copy(disputeReason = value) } },
            label = { Text("What is contested?") },
            modifier = Modifier.fillMaxWidth(),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(
                onClick = viewModel::dispute,
                enabled = !state.working && state.disputeReason.isNotBlank(),
                modifier = Modifier.weight(1f).height(48.dp),
            ) { Text("Dispute") }
            OutlinedButton(
                onClick = viewModel::retryAnchor,
                enabled = !state.working,
                modifier = Modifier.weight(1f).height(48.dp),
            ) { Text("Retry anchor") }
        }
        Button(onClick = viewModel::markAchieved, enabled = !state.working, modifier = Modifier.fillMaxWidth().height(48.dp)) {
            Text("Mark activity achieved")
        }
        Text(
            "Marking achieved records the manager's claim only; it is verified independently.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun EventRow(event: ProvenanceEventDto) {
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(event.type.replace('_', ' '), style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(event.status, tone = stateTone(event.status))
        }
        Text(
            "Actor: ${event.actor} • ${formatInstant(event.timestamp)} • v${event.version}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        event.detail?.let { Text(it, style = MaterialTheme.typography.bodySmall) }
        Text(
            "Reference: ${event.chainReference ?: "not anchored"} on ${chainNetworkLabel(event.network)}",
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        event.commitment?.let {
            Text(
                "Commitment: $it",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        event.explorerUrl?.let {
            Text("Explorer: $it", style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun ProofPanel(state: VerificationUiState, viewModel: VerificationViewModel) {
    when (val proofs = state.proofs) {
        LoadState.Loading -> LoadingState("Loading proofs…")
        is LoadState.Error -> ErrorState(proofs.message, viewModel::load)
        is LoadState.Success -> Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            if (proofs.data.isEmpty()) {
                EmptyState("No proof issued yet", "A proof is built from server state once an achievement exists.")
            }
            proofs.data.forEach { proof -> ProofCard(proof, state.working, viewModel) }
            Button(
                onClick = viewModel::generateProof,
                enabled = !state.working && state.achievementId != null,
                modifier = Modifier.fillMaxWidth().height(48.dp),
            ) { Text(if (state.working) "Working…" else "Generate proof from current state") }
        }
        null -> Text(
            "Load the activity to see the proofs the server holds.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun ProofCard(proof: ProofDto, working: Boolean, viewModel: VerificationViewModel) {
    LedgerCard(contentPadding = 12.dp) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Proof v${proof.version}", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
            StatusChip(proof.status, tone = stateTone(proof.status))
        }
        Text(proof.claimSummary, style = MaterialTheme.typography.bodyMedium)
        Text(
            "${proof.projectName} • generated ${formatInstant(proof.generatedAt)} • type ${proof.proofType}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        proof.issuerDid?.let {
            Text(
                "Issuer: $it",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        proof.verifierAttestation?.let {
            Text("Verifier: ${it.organization} • ${it.decision} • ${formatInstant(it.timestamp)}", style = MaterialTheme.typography.bodySmall)
        }
        proof.chain?.let { chain ->
            Text(
                "Ledger: ${chainNetworkLabel(chain.network)} • ${chain.transactionReference ?: "no reference"}",
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            chain.explorerUrl?.let {
                Text("Explorer: $it", style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        if (proof.evidenceManifest.isNotEmpty()) {
            ExpandableSection("Evidence manifest (${proof.evidenceManifest.size})") {
                proof.evidenceManifest.forEach { entry ->
                    Text(
                        "${entry.name} • ${entry.access} • ${entry.sha256}",
                        style = MaterialTheme.typography.bodySmall,
                        fontFamily = FontFamily.Monospace,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
        if (proof.registryReferences.isNotEmpty()) {
            Text("Registry: ${proof.registryReferences.joinToString()}", style = MaterialTheme.typography.bodySmall)
        }
        proof.sandboxNotice?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.tertiary) }
        proof.supersededBy?.let { Text("Superseded by proof $it", style = MaterialTheme.typography.bodySmall) }
        if (proof.availableFormats.isNotEmpty()) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                proof.availableFormats.forEach { format ->
                    OutlinedButton(
                        onClick = { viewModel.downloadProof(proof, format) },
                        enabled = !working,
                        modifier = Modifier.height(48.dp),
                    ) { Text(format.uppercase()) }
                }
            }
        }
    }
}

@Composable
private fun DecisionPanel(state: VerificationUiState, viewModel: VerificationViewModel) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text(
            "Needs your password. The server records the decision and anchors it as its own event.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            DecisionChip("approve", "Approve", state.decision, viewModel)
            DecisionChip("request_corrections", "Corrections", state.decision, viewModel)
            DecisionChip("reject", "Reject", state.decision, viewModel)
        }
        OutlinedTextField(
            value = state.methodologyVersion,
            onValueChange = { value -> viewModel.update { it.copy(methodologyVersion = value) } },
            label = { Text("Methodology version applied") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = state.notes,
            onValueChange = { value -> viewModel.update { it.copy(notes = value) } },
            label = { Text("Written rationale") },
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
            onClick = viewModel::submitDecision,
            enabled = !state.working && state.decision.isNotBlank() && state.password.isNotBlank(),
            modifier = Modifier.fillMaxWidth().height(48.dp),
        ) { Text(if (state.working) "Recording…" else "Record decision") }
    }
}

@Composable
private fun DecisionChip(value: String, label: String, selected: String, viewModel: VerificationViewModel) {
    FilterChip(
        selected = selected == value,
        onClick = { viewModel.update { it.copy(decision = value) } },
        label = { Text(label) },
    )
}

@Composable
private fun Notice(text: String, isError: Boolean = false) {
    LedgerCard(contentPadding = 12.dp) {
        Text(
            text,
            style = MaterialTheme.typography.bodySmall,
            color = if (isError) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
        )
    }
}
