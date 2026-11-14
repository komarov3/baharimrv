package ke.co.baharilink.baharimrv.ui.satellite

import kotlin.math.PI
import kotlin.math.atan
import kotlin.math.exp
import kotlin.math.pow
import ke.co.baharilink.baharimrv.data.remote.SatelliteOverlayDto
import okhttp3.OkHttpClient
import okhttp3.Request
import retrofit2.HttpException

/**
 * Fetches NDVI tile bytes for one (projectId, sceneId) pair through the app's
 * authenticated OkHttp client. The renderer (osmdroid) calls [fetchTile] on its
 * own tile-loader thread, so every operation here is blocking and thread safe;
 * OkHttp's `execute()` is the synchronous entry point this surface needs.
 *
 * When the server moves past the scene this provider was built for, every tile
 * request answers 410 and the provider asks the caller to refetch the overlay
 * and rebuild with the new scene. A tile that 404s, 410s, falls outside the
 * project bbox/zoom band, or fails on the network returns null so the basemap
 * shows through rather than retrying in a loop on a broken link.
 */
class NdviTileProvider(
    private val projectId: String,
    private val source: SatelliteOverlayDto,
    val baseUrl: String,
    private val tilesClient: OkHttpClient,
    private val onSceneSuperseded: () -> Unit,
) {

    private val zoomRange = source.zoomRange
    private val bbox = source.bbox
    private val sceneId = source.sceneId.orEmpty()
    private val tileBaseUrl = source.tileBaseUrl?.trimEnd('/')

    // A refetch has already been requested once for this provider instance.
    // Guarding against a flood of 410s firing the callback on every tile in
    // the viewport.
    @Volatile
    private var supersedeSignalled: Boolean = false

    /** PNG bytes for the slippy tile, or null when there is no NDVI to draw. */
    fun fetchTile(x: Int, y: Int, zoom: Int): ByteArray? {
        if (tileBaseUrl.isNullOrBlank() || sceneId.isBlank()) return null
        val range = zoomRange
        if (range != null && (zoom < range.min || zoom > range.max)) return null
        if (zoom < MIN_ZOOM || zoom > MAX_ZOOM) return null
        val box = bbox
        if (box != null && !tileIntersects(x, y, zoom, box.minLat, box.minLon, box.maxLat, box.maxLon)) {
            return null
        }
        val url = tileUrl(baseUrl, tileBaseUrl, zoom, x, y, sceneId) ?: return null
        val request = Request.Builder().url(url).build()
        return try {
            tilesClient.newCall(request).execute().use { response ->
                when (response.code) {
                    200 -> response.body?.bytes()
                    410 -> {
                        signalSuperseded()
                        null
                    }
                    else -> null
                }
            }
        } catch (_: Exception) {
            null
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
            // The callback runs on a tile-loader thread; never let its failure
            // propagate into the renderer.
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

        /**
         * The overlay returns a relative tile path, so it has to be joined to the API
         * origin before OkHttp sees it. A relative URL is rejected outright, which
         * would silently drop every tile and leave the basemap bare.
         */
        fun tileUrl(baseUrl: String, tileBaseUrl: String?, zoom: Int, x: Int, y: Int, sceneId: String): String? {
            val path = tileBaseUrl?.trimEnd('/')?.takeIf { it.isNotBlank() } ?: return null
            if (sceneId.isBlank()) return null
            val origin = if (path.startsWith("http://") || path.startsWith("https://")) path else baseUrl.trimEnd('/') + path
            return "$origin/$zoom/$x/$y?layer=ndvi&scene=${urlEncode(sceneId)}"
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
