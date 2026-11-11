package ke.co.baharilink.baharimrv.ui.planting

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.background
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
import androidx.compose.material.icons.filled.Event
import androidx.compose.material.icons.filled.Science
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
import androidx.compose.ui.graphics.Color
import androidx.compose.foundation.shape.RoundedCornerShape
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
                    items(items, key = { it.id }) { activity ->
                        BahariCard(
                            highlighted = activity.status.equals("open", ignoreCase = true),
                            onClick = { onActivityClick(activity.id) },
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.Top,
                                horizontalArrangement = Arrangement.spacedBy(12.dp),
                            ) {
                                // Leading status-aware icon
                                Box(
                                    modifier = Modifier
                                        .size(44.dp)
                                        .background(
                                            color = when {
                                                activity.status.equals("complete", ignoreCase = true) -> Color(0xFF059669).copy(alpha = 0.12f)
                                                activity.status.equals("open", ignoreCase = true) -> Color(0xFFD97706).copy(alpha = 0.12f)
                                                else -> MaterialTheme.colorScheme.surfaceVariant
                                            },
                                            shape = RoundedCornerShape(12.dp),
                                        ),
                                    contentAlignment = Alignment.Center,
                                ) {
                                    Icon(
                                        Icons.Default.Forest,
                                        contentDescription = null,
                                        tint = when {
                                            activity.status.equals("complete", ignoreCase = true) -> Color(0xFF059669)
                                            activity.status.equals("open", ignoreCase = true) -> Color(0xFFD97706)
                                            else -> MaterialTheme.colorScheme.onSurfaceVariant
                                        },
                                        modifier = Modifier.size(24.dp),
                                    )
                                }

                                Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Text(
                                            text = if (activity.plotId.isNotBlank()) activity.plotId else "Activity ${activity.id.take(8)}",
                                            style = MaterialTheme.typography.titleMedium,
                                            fontWeight = FontWeight.SemiBold,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.weight(1f),
                                        )
                                        StatusChip(
                                            text = activity.status.replaceFirstChar { it.uppercase() },
                                            tone = when {
                                                activity.status.equals("complete", ignoreCase = true) -> ChipTone.POSITIVE
                                                activity.status.equals("open", ignoreCase = true) -> ChipTone.WARNING
                                                activity.status.equals("due", ignoreCase = true) -> ChipTone.ERROR
                                                else -> ChipTone.NEUTRAL
                                            },
                                        )
                                    }

                                    Text(
                                        text = activity.activityType.replaceFirstChar { it.uppercase() },
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurface,
                                    )

                                    val period = when {
                                        !activity.periodStart.isNullOrBlank() && !activity.periodEnd.isNullOrBlank() ->
                                            "${activity.periodStart} → ${activity.periodEnd}"
                                        !activity.periodStart.isNullOrBlank() -> activity.periodStart!!
                                        !activity.periodEnd.isNullOrBlank() -> activity.periodEnd!!
                                        else -> null
                                    }
                                    period?.let {
                                        Row(
                                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                        ) {
                                            Icon(
                                                Icons.Default.Event,
                                                contentDescription = null,
                                                modifier = Modifier.size(14.dp),
                                                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                            Text(
                                                text = it,
                                                style = MaterialTheme.typography.bodySmall,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                        }
                                    }

                                    activity.methodologyVersion?.takeIf { it.isNotBlank() }?.let { mv ->
                                        Row(
                                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                        ) {
                                            Icon(
                                                Icons.Default.Science,
                                                contentDescription = null,
                                                modifier = Modifier.size(14.dp),
                                                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                            Text(
                                                text = "Methodology $mv",
                                                style = MaterialTheme.typography.bodySmall,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                        }
                                    }

                                    Text(
                                        text = "ID ${activity.id}",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
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
