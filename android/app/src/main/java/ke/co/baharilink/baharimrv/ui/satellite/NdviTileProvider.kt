package ke.co.baharilink.baharimrv.ui.satellite

import com.google.android.gms.maps.model.Tile
import com.google.android.gms.maps.model.TileProvider
import kotlin.math.PI
import kotlin.math.atan
import kotlin.math.exp
import kotlin.math.pow
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import okhttp3.OkHttpClient
import okhttp3.Request
import retrofit2.HttpException

/**
 * Renders NDVI tiles over the map. `TileProvider.getTile` is invoked on a
 * binder thread inside the Maps SDK, so every operation here must be thread
 * safe and blocking. OkHttp's `execute()` is the synchronous entry point,
 * which is exactly what this surface needs.
 *
 * The provider is built for one (projectId, sceneId) pair. When the
 * server moves past that scene, every tile request answers 410 and the
 * provider asks the caller to refetch the overlay and rebuild with the new
 * scene. A tile that 404s or 410s returns [TileProvider.NO_TILE] so the basemap
 * shows through; a network failure returns [TileProvider.NO_TILE] too, because
 * [TileProvider.UNKNOWN_TILE] would cause the SDK to retry in a loop and
 * burn battery on a broken link.
 */
class NdviTileProvider(
    private val projectId: String,
    private val source: SatelliteOverlayDto,
    private val baseUrl: String,
    private val tilesClient: OkHttpClient,
    private val onSceneSuperseded: () -> Unit,
) : TileProvider {

    private val zoomRange = source.zoomRange
    private val bbox = source.bbox
    private val sceneId = source.sceneId.orEmpty()
    private val tileBaseUrl = source.tileBaseUrl?.trimEnd('/')

    // A refetch has already been requested once for this provider instance.
    // Guarding against a flood of 410s from the SDK firing the callback on
    // every tile in the viewport.
    @Volatile
    private var supersedeSignalled: Boolean = false

    override fun getTile(x: Int, y: Int, zoom: Int): Tile {
        if (tileBaseUrl.isNullOrBlank() || sceneId.isBlank()) return TileProvider.NO_TILE
        val range = zoomRange
        if (range != null && (zoom < range.min || zoom > range.max)) return TileProvider.NO_TILE
        if (zoom < MIN_ZOOM || zoom > MAX_ZOOM) return TileProvider.NO_TILE
        val box = bbox
        if (box != null && !tileIntersects(x, y, zoom, box.minLat, box.minLon, box.maxLat, box.maxLon)) {
            return TileProvider.NO_TILE
        }
        val url = "$tileBaseUrl/$zoom/$x/$y?layer=ndvi&scene=${urlEncode(sceneId)}"
        val request = Request.Builder().url(url).build()
        return try {
            tilesClient.newCall(request).execute().use { response ->
                when (response.code) {
                    200 -> {
                        val body = response.body?.bytes() ?: return TileProvider.NO_TILE
                        Tile(TILE_SIZE, TILE_SIZE, body)
                    }
                    410 -> {
                        signalSuperseded()
                        TileProvider.NO_TILE
                    }
                    404 -> TileProvider.NO_TILE
                    else -> TileProvider.NO_TILE
                }
            }
        } catch (_: Exception) {
            TileProvider.NO_TILE
        }
    }

    /** Whether this instance was constructed for the given scene. */
    fun isForScene(scene: String?): Boolean = scene != null && scene == sceneId

    private fun signalSuperseded() {
        if (supersedeSignalled) return
        supersedeSignalled = true
        try {
            onSceneSuperseded()
        } catch (_: Exception) {
            // The callback runs on a binder thread; never let its failure
            // propagate into the Maps SDK.
        }
    }

    companion object {
        const val TILE_SIZE = 256
        const val MIN_ZOOM = 8
        const val MAX_ZOOM = 16

        /**
         * True when the slippy tile (x, y, z) overlaps the given bbox. Tiles
         * are axis aligned, so a quick min/max lat/lon comparison against the
         * tile's four corners is sufficient.
         */
        fun tileIntersects(
            x: Int,
            y: Int,
            zoom: Int,
            minLat: Double,
            minLon: Double,
            maxLat: Double,
            maxLon: Double,
        ): Boolean {
            val n = 2.0.pow(zoom.toDouble())
            if (n <= 0) return false
            val tileMinLon = x / n * 360.0 - 180.0
            val tileMaxLon = (x + 1) / n * 360.0 - 180.0
            val tileMaxLat = tileLat(y, n)
            val tileMinLat = tileLat(y + 1, n)
            return !(tileMaxLon < minLon || tileMinLon > maxLon ||
                tileMaxLat < minLat || tileMinLat > maxLat)
        }

        private fun tileLat(y: Int, n: Double): Double {
            val rad = PI * (1.0 - 2.0 * y / n)
            return Math.toDegrees(2.0 * atan(exp(rad)) - PI / 2.0)
        }

        private fun urlEncode(value: String): String =
            java.net.URLEncoder.encode(value, Charsets.UTF_8.name())
    }
}

/**
 * Translates a satellite HTTP error into a typed shape the ViewModel can
 * branch on. Only 410 is special: it means the scene the provider was built
 * for has been superseded and the client must refetch the overlay.
 */
class SceneSupersededException : RuntimeException("satellite scene superseded")

internal fun HttpException.isSceneSuperseded(): Boolean =
    code() == 410
