package ke.co.baharilink.baharimrv.ui.planting

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.remote.ActivityDetailDto
import ke.co.baharilink.baharimrv.data.repository.ProjectRepository
import ke.co.baharilink.baharimrv.ui.components.LoadState
import ke.co.baharilink.baharimrv.ui.components.readableMessage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class PlantingDetailUiState(
    // null = not yet requested; Loading = in flight; Success/Error = done
    val detail: LoadState<ActivityDetailDto>? = null,
    // decoded evidence photo; null until the detail resolves with at least one
    // evidence object, then Loading while the bytes stream and decode.
    val image: LoadState<ImageBitmap>? = null,
)

@HiltViewModel
class PlantingDetailViewModel @Inject constructor(
    private val repository: ProjectRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlantingDetailUiState())
    val state = _state.asStateFlow()

    fun load(activityId: String) {
        _state.update { it.copy(detail = LoadState.Loading) }
        viewModelScope.launch {
            try {
                val detail = repository.activityDetail(activityId)
                _state.update { it.copy(detail = LoadState.Success(detail)) }
            } catch (e: Exception) {
                _state.update { it.copy(detail = LoadState.Error(e.readableMessage())) }
            }
        }
    }

    /**
     * Streams one evidence object's bytes over the authenticated Retrofit client and
     * decodes them into an ImageBitmap for the thumbnail and the full-screen overlay.
     * Decoding runs on Dispatchers.IO and is downsampled so a large field photo never
     * decodes every pixel and risks an out-of-memory crash on a phone.
     */
    fun loadImage(evidenceId: String) {
        _state.update { it.copy(image = LoadState.Loading) }
        viewModelScope.launch {
            try {
                val bitmap = withContext(Dispatchers.IO) {
                    val body = repository.evidenceMedia(evidenceId)
                    val bytes = body.use { it.bytes() }
                    decodeSampled(bytes)
                }
                if (bitmap == null) {
                    _state.update { it.copy(image = LoadState.Error("Could not decode the evidence image.")) }
                } else {
                    _state.update { it.copy(image = LoadState.Success(bitmap.asImageBitmap())) }
                }
            } catch (e: Exception) {
                _state.update { it.copy(image = LoadState.Error(e.readableMessage())) }
            }
        }
    }

    private fun decodeSampled(bytes: ByteArray): Bitmap? {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
        val options = BitmapFactory.Options().apply { inSampleSize = sampleSize(bounds.outWidth, bounds.outHeight, 2048) }
        return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
    }

    private fun sampleSize(width: Int, height: Int, target: Int): Int {
        if (width <= 0 || height <= 0) return 1
        var sample = 1
        while ((width / sample) > target || (height / sample) > target) sample *= 2
        return sample
    }
}
