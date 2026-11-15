package ke.co.baharilink.baharimrv.ui.planting

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.ProjectActivityDto
import ke.co.baharilink.baharimrv.ui.components.*
import kotlin.math.ceil

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlantingListScreen(
    viewModel: PlantingListViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
    onActivityClick: (String) -> Unit,
    onCreate: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    LaunchedEffect(projectId) { viewModel.load(projectId) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Planting activities") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { padding ->
        when (val activities = state.activities) {
            is LoadState.Loading -> Column(
                modifier = Modifier.fillMaxSize().padding(padding).padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                repeat(3) { SkeletonCard() }
            }
            is LoadState.Error -> Box(modifier = Modifier.fillMaxSize().padding(padding)) {
                ErrorState(message = activities.message, onRetry = { viewModel.load(projectId) })
            }
            is LoadState.Success -> {
                val items = activities.data
                val pageSize = PlantingListViewModel.PAGE_SIZE
                val filtered = remember(items, state.query) { items.filter { matchesQuery(it, state.query) } }
                val pageCount = maxOf(1, ceil(filtered.size / pageSize.toDouble()).toInt())
                val page = state.page.coerceIn(0, pageCount - 1)
                val pageItems = remember(filtered, page) { filtered.drop(page * pageSize).take(pageSize) }

                Column(modifier = Modifier.fillMaxSize().padding(padding)) {
                    // Search + export controls stay fixed above the scrolling list.
                    Column(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        OutlinedTextField(
                            value = state.query,
                            onValueChange = viewModel::setQuery,
                            modifier = Modifier.fillMaxWidth(),
                            placeholder = { Text("Search plot, type, status…") },
                            leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                            trailingIcon = {
                                if (state.query.isNotEmpty()) {
                                    IconButton(onClick = { viewModel.setQuery("") }) {
                                        Icon(Icons.Default.Close, contentDescription = "Clear search")
                                    }
                                }
                            },
                            singleLine = true,
                            shape = MaterialTheme.shapes.small,
                        )
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Text(
                                text = if (state.query.isBlank()) "${items.size} activities"
                                else "${filtered.size} of ${items.size} match",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.weight(1f),
                            )
                            BahariButton(
                                onClick = { shareActivitiesCsv(context, filtered) },
                                variant = BahariButtonVariant.OUTLINED,
                                small = true,
                                enabled = filtered.isNotEmpty(),
                            ) {
                                Icon(Icons.Default.FileDownload, contentDescription = null, modifier = Modifier.size(18.dp))
                                Text("Export CSV")
                            }
                        }
                    }

                    LazyColumn(
                        modifier = Modifier.fillMaxWidth().weight(1f),
                        contentPadding = PaddingValues(16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        items(pageItems, key = { it.id }) { activity ->
                            ActivityCard(activity = activity, onClick = { onActivityClick(activity.id) })
                        }

                        if (filtered.isEmpty()) {
                            item {
                                if (items.isEmpty()) {
                                    EmptyState(
                                        title = "No planting activities yet",
                                        detail = "Create one to begin tracking survival.",
                                        actionLabel = "Create planting activity",
                                        onAction = onCreate,
                                    )
                                } else {
                                    EmptyState(
                                        title = "No matches",
                                        detail = "No activities match “${state.query}”. Try a different search.",
                                        actionLabel = "Clear search",
                                        onAction = { viewModel.setQuery("") },
                                    )
                                }
                            }
                        }

                        if (pageCount > 1) {
                            item {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.Center,
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    IconButton(onClick = { viewModel.setPage(page - 1) }, enabled = page > 0) {
                                        Icon(Icons.Default.ChevronLeft, contentDescription = "Previous page")
                                    }
                                    Text(
                                        text = "Page ${page + 1} of $pageCount",
                                        style = MaterialTheme.typography.bodyMedium,
                                        fontWeight = FontWeight.Medium,
                                        modifier = Modifier.padding(horizontal = 12.dp),
                                    )
                                    IconButton(onClick = { viewModel.setPage(page + 1) }, enabled = page < pageCount - 1) {
                                        Icon(Icons.Default.ChevronRight, contentDescription = "Next page")
                                    }
                                }
                            }
                        }

                        item {
                            BahariButton(
                                onClick = onCreate,
                                variant = BahariButtonVariant.GREEN,
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Icon(Icons.Default.Add, contentDescription = null, modifier = Modifier.size(20.dp))
                                Text("Create planting activity")
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ActivityCard(activity: ProjectActivityDto, onClick: () -> Unit) {
    BahariCard(
        highlighted = activity.status.equals("open", ignoreCase = true),
        onClick = onClick,
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            BahariIconBackground(
                color = when {
                    activity.status.equals("complete", ignoreCase = true) -> IconBackgroundColor.GREEN
                    activity.status.equals("open", ignoreCase = true) -> IconBackgroundColor.AMBER
                    else -> IconBackgroundColor.DEFAULT
                },
            ) {
                Icon(Icons.Default.Forest, contentDescription = null, modifier = Modifier.size(20.dp))
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
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Event, contentDescription = null, modifier = Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(text = it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }

                activity.methodologyVersion?.takeIf { it.isNotBlank() }?.let { mv ->
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Science, contentDescription = null, modifier = Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(text = "Methodology $mv", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }

                Text(
                    text = "ID ${activity.id}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            Icon(
                Icons.Default.ChevronRight,
                contentDescription = null,
                modifier = Modifier.size(20.dp).align(Alignment.CenterVertically),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}