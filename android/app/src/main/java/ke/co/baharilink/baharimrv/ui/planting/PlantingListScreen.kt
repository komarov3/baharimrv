package ke.co.baharilink.baharimrv.ui.planting

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Forest
import androidx.compose.material.icons.filled.Nature
import androidx.compose.material.icons.filled.Satellite
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.layout.size
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.ContextHeader
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.StatusChip

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun PlantingListScreen(
    viewModel: PlantingListViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
    onActivityClick: (String) -> Unit,
    onCreate: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(projectId) {
        viewModel.load(projectId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(text = "Planting activities") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) {
        when (val activities = state.activities) {
            is LoadState.Loading -> {
                androidx.compose.foundation.layout.Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = androidx.compose.ui.Alignment.Center,
                ) {
                    LoadingState("Loading planting activities…")
                }
            }
            is LoadState.Error -> {
                ErrorState(message = activities.message, onRetry = { viewModel.load(projectId) })
            }
            is LoadState.Success -> {
                val items = activities.data
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    item {
                        ContextHeader(
                            title = "Planting activities",
                            subtitle = "Open · due · completed",
                        )
                    }
                    items(items) { activity ->
                        BahariCard(
                            highlighted = activity.status.equals("open", ignoreCase = true),
                            onClick = { onActivityClick(activity.id) },
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Icon(Icons.Default.Nature, contentDescription = null, modifier = Modifier.size(18.dp))
                                Column(modifier = Modifier.weight(1f)) {
                                    // "Gazi Bay · P-14" style header from PL2
                                    val projectPart = if (activity.plotId.isNotBlank())
                                        activity.plotId else activity.id.take(8)
                                    Text(
                                        text = projectPart,
                                        style = MaterialTheme.typography.titleSmall,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    // "Planting · 280 seedlings · R. mucronata" row
                                    Text(
                                        text = buildString {
                                            append(activity.activityType.replaceFirstChar { it.uppercase() })
                                            if (activity.periodStart != null && activity.periodEnd != null) {
                                                append(" · ${activity.periodStart} – ${activity.periodEnd}")
                                            }
                                        },
                                        style = MaterialTheme.typography.bodySmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                                StatusChip(
                                    text = activity.status.capitalize(),
                                    tone = when {
                                        activity.status.equals("complete", ignoreCase = true) -> ChipTone.POSITIVE
                                        activity.status.equals("open", ignoreCase = true) -> ChipTone.WARNING
                                        activity.status.equals("due", ignoreCase = true) -> ChipTone.ERROR
                                        else -> ChipTone.NEUTRAL
                                    },
                                )
                            }
                            // Methodology version if available
                            activity.methodologyVersion?.let { mv ->
                                Text(
                                    "Methodology: $mv",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                    }

                    // ── "Next step" callout (PL2 wireframe) ──────────────
                    item {
                        if (items.isNotEmpty()) {
                            androidx.compose.material3.Card(
                                modifier = Modifier.fillMaxWidth(),
                                colors = androidx.compose.material3.CardDefaults.cardColors(
                                    containerColor = MaterialTheme.colorScheme.surfaceVariant,
                                ),
                            ) {
                                Column(
                                    modifier = Modifier.padding(16.dp),
                                    verticalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    Text(
                                        "Next step: field visit",
                                        style = MaterialTheme.typography.titleSmall,
                                        fontWeight = FontWeight.SemiBold,
                                    )
                                    Row(
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Icon(
                                            Icons.Default.Forest,
                                            contentDescription = null,
                                            modifier = Modifier.size(16.dp),
                                            tint = MaterialTheme.colorScheme.primary,
                                        )
                                        Text(
                                            "Record survival · alive / planted · photos · voice",
                                            style = MaterialTheme.typography.bodySmall,
                                        )
                                    }
                                    Row(
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Icon(
                                            Icons.Default.Satellite,
                                            contentDescription = null,
                                            modifier = Modifier.size(16.dp),
                                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                        )
                                        Text(
                                            "Satellite NDVI check · pending tile match",
                                            style = MaterialTheme.typography.bodySmall,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        )
                                    }
                                    Text(
                                        "Each activity shows its planted baseline — the denominator for survival rate.",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                            }
                        }
                    }
                    item {
                        if (items.isEmpty()) {
                            EmptyState(
                                title = "No planting activities yet",
                                detail = "Create one to begin tracking survival.",
                                actionLabel = "Create planting activity",
                                onAction = onCreate,
                            )
                        }
                    }
                    item {
                        BahariButton(
                            onClick = onCreate,
                            variant = BahariButtonVariant.GREEN,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Text(text = "Create planting activity")
                        }
                    }
                }
            }
        }
    }
}

private fun String?.capitalize(): String = this?.ifEmpty { "" }?.replaceFirstChar { it.uppercase() } ?: ""
