package ke.co.baharilink.baharimrv.ui.satellite

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ke.co.baharilink.baharimrv.data.remote.SatellitePassDto
import ke.co.baharilink.baharimrv.ui.components.BahariButton
import ke.co.baharilink.baharimrv.ui.components.BahariButtonVariant
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.BahariStatusPill
import ke.co.baharilink.baharimrv.ui.components.ChipTone
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.StatusChip

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun SatelliteMonitoringScreen(
    viewModel: SatelliteMonitoringViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val markers = viewModel.plotMarkers()

    LaunchedEffect(projectId) {
        viewModel.setProjectId(projectId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(text = "Satellite monitoring") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // NDVI Plot Markers summary
            if (markers.isNotEmpty()) {
                BahariCard(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                ) {
                    Column(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally,
                    ) {
                        Icon(
                            imageVector = Icons.Default.Satellite,
                            contentDescription = "Satellite",
                            tint = MaterialTheme.colorScheme.primary,
                        )
                        Text(
                            text = "Plot NDVI Overview",
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(top = 8.dp),
                        )
                        LazyColumn(
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            items(viewModel.plotMarkers()) { marker ->
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                                ) {
                                    Text(
                                        text = marker.label,
                                        style = MaterialTheme.typography.bodyMedium,
                                        modifier = Modifier.weight(1f),
                                    )
                                    Text(
                                        text = marker.ndvi?.let { "%.2f".format(it) } ?: "N/A",
                                        style = MaterialTheme.typography.bodyMedium.copy(
                                            color = MaterialTheme.colorScheme.primary,
                                            fontWeight = FontWeight.Bold,
                                        ),
                                    )
                                }
                            }
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Latest satellite pass
            when (val latestPass = state.latestPass) {
                is LoadState.Loading -> {
                    LoadingState("Loading satellite data…")
                }
                is LoadState.Error -> {
                    ErrorState(message = latestPass.message, onRetry = { viewModel.retry() })
                }
                is LoadState.Success -> {
                    if (latestPass.value != null) {
                        BahariCard(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(16.dp),
                        ) {
                            Column(
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text(
                                    text = "Latest Pass",
                                    style = MaterialTheme.typography.titleMedium,
                                    modifier = Modifier.padding(bottom = 8.dp),
                                )
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                                ) {
                                    Text(
                                        text = latestPass.value?.sceneId ?: latestPass.value?.id ?: "—",
                                        style = MaterialTheme.typography.titleSmall,
                                        modifier = Modifier.weight(1f),
                                    )
                                    Text(
                                        text = latestPass.value?.cloudCoverPct
                                            ?.let { "Cloud ${it.toInt()}%" }
                                            ?: "—",
                                        style = MaterialTheme.typography.bodySmall.copy(
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        ),
                                    )
                                    StatusChip(
                                        text = latestPass.value?.adapterMode ?: "sandbox",
                                        tone = ChipTone.NEUTRAL,
                                    )
                                }
                            }
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Passes list
            Text(
                text = "Satellite Passes",
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(horizontal = 16.dp, top = 8.dp),
            )
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = 16.dp),
            ) {
                items(state.passes.valueOrNull() ?: emptyList()) { pass ->
                    BahariCard(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(bottom = 8.dp),
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                        ) {
                            Text(
                                text = pass.sceneId ?: pass.id,
                                style = MaterialTheme.typography.titleSmall,
                                modifier = Modifier.weight(1f),
                            )
                            Text(
                                text = pass.cloudCoverPct
                                    ?.let { "Cloud ${it.toInt()}%" }
                                    ?: "—",
                                style = MaterialTheme.typography.bodySmall.copy(
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                ),
                            )
                            StatusChip(
                                text = pass.adapterMode ?: "sandbox",
                                tone = ChipTone.NEUTRAL,
                            )
                        }
                    }
                }
                if ((state.passes.valueOrNull() ?: emptyList()).isEmpty()) {
                    item {
                        EmptyState(
                            title = "No satellite passes yet",
                            detail = "NDVI corroboration appears after a provider-neutral tile match returns.",
                        )
                    }
                }
            }

            BahariButton(
                onClick = viewModel::retry,
                variant = BahariButtonVariant.OUTLINED,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, bottom = 16.dp),
            ) {
                Text(text = "Refresh satellite data")
            }
        }
    }
}

private enum class ChipTone {
    POSITIVE, WARNING, INFO, NEUTRAL
}
