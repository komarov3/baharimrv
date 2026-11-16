package ke.co.baharilink.baharimrv.ui.planting

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ActivityNdviDto
import ke.co.baharilink.baharimrv.data.remote.ActivityNdviReadingDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.BahariCard
import ke.co.baharilink.baharimrv.ui.components.KeyValue
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.Locale

private val pendingNdviStates = setOf("waiting", "queued", "running")

data class ActivityNdviUiState(
    val data: ActivityNdviDto? = null,
    val loading: Boolean = false,
    val recapturing: Boolean = false,
    val error: String? = null,
    val revision: Int = 0,
)

@HiltViewModel
class ActivityNdviViewModel @Inject constructor(private val repository: ProjectRepository) : ViewModel() {
    private val _state = MutableStateFlow(ActivityNdviUiState())
    val state = _state.asStateFlow()

    suspend fun observe(activityId: String) {
        try {
            do {
                _state.update { it.copy(loading = true, error = null) }
                val data = repository.activityNdvi(activityId)
                _state.update { it.copy(data = data, loading = false) }
                if (data.status !in pendingNdviStates) return
                delay(15_000)
            } while (true)
        } catch (e: CancellationException) { throw e
        } catch (e: Exception) {
            _state.update { it.copy(loading = false, error = e.readableMessage()) }
        } finally { _state.update { it.copy(loading = false) } }
    }

    fun checkStatus() { _state.update { it.copy(revision = it.revision + 1) } }

    fun recapture(activityId: String) {
        if (_state.value.recapturing || _state.value.data?.status in pendingNdviStates) return
        _state.update { it.copy(recapturing = true, error = null) }
        viewModelScope.launch {
            try {
                val data = repository.recaptureActivityNdvi(activityId)
                _state.update { it.copy(data = data, recapturing = false, revision = it.revision + 1) }
            } catch (e: CancellationException) { throw e
            } catch (e: Exception) {
                _state.update { it.copy(recapturing = false, error = e.readableMessage()) }
            }
        }
    }
}

@Composable
fun ActivityNdviSection(activityId: String?, canRecapture: Boolean = false) {
    BahariCard {
        Text("Planting NDVI", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        Text(
            "Vegetation signal, not a seedling survival count or carbon verification. Clouds, shadows and tides can limit optical NDVI.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (activityId == null) {
            Text("Captured automatically after saving, using the plot boundary or its buffered location.")
            Text("Search window: 15 days before and after the reporting start date, or the creation date if no start is supplied.", style = MaterialTheme.typography.bodySmall)
        } else {
            val viewModel: ActivityNdviViewModel = hiltViewModel(key = "ndvi-$activityId")
            val state by viewModel.state.collectAsStateWithLifecycle()
            val lifecycle = LocalLifecycleOwner.current.lifecycle
            LaunchedEffect(activityId, lifecycle, state.revision) {
                lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) { viewModel.observe(activityId) }
            }
            val data = state.data
            if (state.loading && data == null) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            data?.let {
                Text(ndviStatusText(it.status), style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.SemiBold)
                if (it.status in pendingNdviStates) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                if (it.status == "waiting") Text("Waiting for this project's imagery slot. The shared limit is six refreshes per hour.", style = MaterialTheme.typography.bodySmall)
                if (it.windowStart != null && it.windowEnd != null) KeyValue("Search window", "${it.windowStart.take(10)} to ${it.windowEnd.take(10)}")
                if (it.status == "no_observation") Text("No usable observation was returned. Imagery may be cloud-limited or unavailable; this is not an NDVI value of zero.", style = MaterialTheme.typography.bodySmall)
                it.error?.takeIf(String::isNotBlank)?.let { error -> Text(error, color = MaterialTheme.colorScheme.error) }
                it.observation?.let { reading -> NdviReading("Planting capture", reading) }
                it.baseline?.takeIf { baseline -> baseline.id != it.observation?.id }?.let { baseline -> NdviReading("Reference baseline", baseline) }
                it.latestPass?.let { pass ->
                    NdviReading("Latest comparable pass", pass)
                    KeyValue("NDVI change", it.changeAbs?.let { value -> String.format(Locale.US, "%+.4f", value) } ?: "Not available")
                    KeyValue("Relative change", it.changePct?.let { value -> String.format(Locale.US, "%+.2f%%", value) } ?: "Not available")
                }
                if (it.latestPass == null && it.observation != null) Text("No later comparable pass yet.", style = MaterialTheme.typography.bodySmall)
                Text("Reports update from subsequent project imagery refreshes. Comparisons require the same plot geometry and compatible processing.", style = MaterialTheme.typography.bodySmall)
            }
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = viewModel::checkStatus, enabled = !state.loading && !state.recapturing) { Text("Check status") }
                if (canRecapture) {
                    TextButton(
                        onClick = { viewModel.recapture(activityId) },
                        enabled = !state.loading && !state.recapturing && data?.status !in pendingNdviStates,
                    ) { Text(if (state.recapturing) "Requesting…" else "Recapture baseline") }
                }
            }
        }
    }
}

private fun ndviStatusText(status: String): String = when (status) {
    "waiting" -> "Waiting for imagery slot"
    "queued" -> "Capture queued"
    "running" -> "Capturing baseline"
    "complete" -> "Capture complete"
    "partial" -> "Partial coverage"
    "cloudy" -> "Cloud-limited observation"
    "synthetic" -> "Sandbox output, not satellite imagery"
    "no_observation" -> "No usable observation"
    "failed" -> "Capture failed"
    "not_requested" -> "No baseline requested"
    else -> status
}

@Composable
private fun NdviReading(title: String, reading: ActivityNdviReadingDto) {
    HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))
    Text(title, style = MaterialTheme.typography.titleSmall)
    Text(
        if (reading.adapterMode == "sandbox") "SANDBOX · Synthetic, not live imagery" else "${reading.adapterMode} · ${reading.quality}",
        fontWeight = FontWeight.SemiBold,
        color = MaterialTheme.colorScheme.onSurface,
    )
    KeyValue("Mean NDVI", reading.meanNdvi?.let { String.format(Locale.US, "%.4f", it) } ?: "Not measured")
    KeyValue("Scene date", reading.acquiredAt?.take(10) ?: "No acquisition")
    KeyValue("Cloud cover", reading.cloudPct?.let { String.format(Locale.US, "%.1f%%", it) } ?: "Not measured")
    KeyValue("Usable coverage", reading.coveragePct?.let { String.format(Locale.US, "%.1f%%", it) } ?: "Not measured")
    if (reading.quality == "partial") Text("Limited usable pixels. Do not treat this as a full-coverage baseline.", style = MaterialTheme.typography.bodySmall)
    var expanded by remember(reading.id) { mutableStateOf(false) }
    TextButton(onClick = { expanded = !expanded }) { Text(if (expanded) "Hide provenance" else "Show provenance") }
    if (expanded) {
        reading.sceneId?.let { KeyValue("Scene", it) }
        KeyValue("Provider", reading.provider)
        KeyValue("Processing", reading.processingVersion)
        KeyValue("Result SHA-256", reading.resultHash, monospace = true)
        reading.sourceActivityId?.let { KeyValue("Source activity", it, monospace = true) }
    }
}
