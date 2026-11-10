package ke.co.baharilink.baharimrv.ui.satellite

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
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
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.EmptyState
import ke.co.baharilink.baharimrv.ui.components.ErrorState
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.LoadingState

@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun SatelliteMonitoringScreen(
    viewModel: SatelliteMonitoringViewModel = hiltViewModel(),
    onBack: () -> Unit,
    projectId: String,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

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
        Column(
            modifier = Modifier.fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            when (state.latestPass) {
                is LoadState.Loading -> {
                    LoadingState("Loading satellite data…")
                }
                is LoadState.Error -> {
                    ErrorState(message = state.latestPass.message, onRetry = { viewModel.retry() })
                }
                is LoadState.Success -> {
                    BahariCard(modifier = Modifier.padding(16.dp)) {
                        Column(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Icon(
                                imageVector = Icons.Default.Satellite,
                                contentDescription = "Satellite",
                                tint = MaterialTheme.colorScheme.primary,
                            )
                            Text(
                                text = "Satellite monitoring",
                                style = MaterialTheme.typography.titleMedium,
                                modifier = Modifier.padding(top = 8.dp),
                            )
                            Text(
                                text = "Placeholder — satellite imagery integration coming soon.",
                                style = MaterialTheme.typography.bodyMedium.copy(
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                ),
                                modifier = Modifier.padding(top = 4.dp),
                            )
                            if (state.latestPass.value != null) {
                                Text(
                                    text = "Latest: ${state.latestPass.value?.sceneId ?: state.latestPass.value?.id}",
                                    style = MaterialTheme.typography.bodySmall.copy(
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    ),
                                    modifier = Modifier.padding(top = 4.dp),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
